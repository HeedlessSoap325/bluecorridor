package docker

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client"
)

const VolumeSaveAndRestoreMountPath = "/mount"

// List all Volumes
func VolumeList(filters client.Filters) ([]volume.Volume, error) {
	volumes, err := dockerClient.VolumeList(ctx, client.VolumeListOptions{
		Filters: filters,
	})

	if err != nil {
		return nil, fmt.Errorf("Error occured while listing docker volumes: %s", err)
	}

	return volumes.Items, nil
}

// Inspect an Volume
func VolumeInspect(volumeID string) (client.VolumeInspectResult, error) {
	inspect, err := dockerClient.VolumeInspect(ctx, volumeID, client.VolumeInspectOptions{})

	if err != nil {
		return client.VolumeInspectResult{}, fmt.Errorf("Error occured while inspecting docker volume: %s", err)
	}

	return inspect, nil
}

func VolumeCreate(opts client.VolumeCreateOptions) (volume.Volume, error) {
	res, err := dockerClient.VolumeCreate(ctx, opts)

	if err != nil {
		return volume.Volume{}, err
	}

	return res.Volume, nil
}

// Save the contents of a volume into a tar archive
//
// This function will create a file called {outDir}/{saveName}.tar and store the contents of the {volumeName} volume in it
//
// The root of the archive is {VolumeSaveAndRestoreMountPath}
func VolumeSave(volumeName string, saveName string, outDir string) error {
	err := ImagePull("alpine", false)
	if err != nil {
		return err
	}

	id, err := ContainerCreate(client.ContainerCreateOptions{
		Config: &container.Config{
			Image: "alpine",
		},
		HostConfig: &container.HostConfig{
			Mounts: []mount.Mount{
				{
					Type:     mount.TypeVolume,
					Source:   volumeName,
					Target:   VolumeSaveAndRestoreMountPath,
					ReadOnly: true,
				},
			},
		},
		NetworkingConfig: nil,
		Platform:         nil,
		Name:             "",
	})

	if err != nil {
		return err
	}
	defer ContainerRemove(id)

	reader, err := dockerClient.CopyFromContainer(ctx, id, client.CopyFromContainerOptions{
		SourcePath: VolumeSaveAndRestoreMountPath,
	})

	if err != nil {
		return fmt.Errorf("Error occured while copying files from container '%s' to host: %s", id, err)
	}
	defer reader.Content.Close()

	fileName := fmt.Sprintf("%s.tar", saveName)
	out, err := os.Create(path.Join(outDir, fileName))

	if err != nil {
		return fmt.Errorf("Error occured while creating File '%s.tar' to %s: %s", saveName, outDir, err)
	}
	defer out.Close()

	_, err = io.Copy(out, reader.Content)
	if err != nil {
		return fmt.Errorf("Error occured while copying tar archive: %s", err)
	}

	return nil
}

func VolumeRestore(volumeName string, saveName string, inDir string) error {
	err := ImagePull("alpine", false)
	if err != nil {
		return err
	}

	id, err := ContainerCreate(client.ContainerCreateOptions{
		Config: &container.Config{
			Image: "alpine",
		},
		HostConfig: &container.HostConfig{
			Mounts: []mount.Mount{
				{
					Type:     mount.TypeVolume,
					Source:   volumeName,
					Target:   VolumeSaveAndRestoreMountPath,
					ReadOnly: false,
				},
			},
		},
		NetworkingConfig: nil,
		Platform:         nil,
		Name:             "",
	})

	if err != nil {
		return err
	}
	defer ContainerRemove(id)

	fileName := fmt.Sprintf("%s.tar", saveName)
	buffer, err := os.ReadFile(path.Join(inDir, fileName))
	if err != nil {
		return fmt.Errorf("Error occured while reading file %s/%s.tar: %s", inDir, saveName, err)
	}

	content := bytes.NewReader(buffer)

	_, err = dockerClient.CopyToContainer(ctx, id, client.CopyToContainerOptions{
		DestinationPath:           "/", // The TAR archive is expected to contain a root filder named {VolumeSaveAndRestoreMountPath}, therefore, the extraction must take place in root
		Content:                   content,
		AllowOverwriteDirWithFile: false,
		CopyUIDGID:                true,
	})

	if err != nil {
		return fmt.Errorf("Error occured while copying tar archive to container '%s': %s", id, err)
	}
	return nil
}

func VolumeSize(volumeName string) (int64, error) {
	// Pull alpine:latest if not already present
	err := ImagePull("alpine", false)
	if err != nil {
		return 0, err
	}

	// Create a dummy container which mounts the volume and executes du
	id, err := ContainerCreate(client.ContainerCreateOptions{
		Config: &container.Config{
			Image: "alpine",
			Cmd:   []string{"du", "-sb", "/data"},
		},
		HostConfig: &container.HostConfig{
			Mounts: []mount.Mount{
				{
					Type:     mount.TypeVolume,
					Source:   volumeName,
					Target:   "/data",
					ReadOnly: true,
				},
			},
		},
		NetworkingConfig: nil,
		Platform:         nil,
		Name:             "",
	})
	if err != nil {
		return 0, err
	}

	defer ContainerRemove(id)

	// Start the dummy container so the command runs
	if err := ContainerStart(id); err != nil {
		return 0, fmt.Errorf("container start failed: %w", err)
	}

	// Wait for the container to finish execution
	res := dockerClient.ContainerWait(ctx, id, client.ContainerWaitOptions{})
	select {
	case err := <-res.Error:
		if err != nil {
			return 0, fmt.Errorf("container wait failed: %w", err)
		}
	case <-res.Result:
	}

	// Grab the Stdout of the container
	out, err := dockerClient.ContainerLogs(ctx, id, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return 0, fmt.Errorf("container logs failed: %w", err)
	}

	defer out.Close()

	// Docker multiplexes stdout/stderr into a stream with 8-byte headers stdcopy strips those headers properly
	var buf strings.Builder
	if _, err := stdcopy.StdCopy(&buf, io.Discard, out); err != nil {
		return 0, fmt.Errorf("stdcopy failed: %w", err)
	}

	// Output is "<size in bytes>\t/data\n"
	fields := strings.Fields(buf.String())
	if len(fields) == 0 {
		return 0, nil
	}

	bytes, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse du output %q: %w", fields[0], err)
	}
	return bytes, nil
}

func VolumeAnonymous(VolumeLabels map[string]string) bool {
	_, isAnonymous := VolumeLabels["com.docker.volume.anonymous"]
	return isAnonymous
}

func VolumeDataless(VolumeLabels map[string]string) bool {
	_, isdataless := VolumeLabels["dev.heedlesssoap.bluecorridor.volume.dataless"]
	return isdataless
}

func VolumeReference(VolumeLabels map[string]string) (bool, string) {
	ref, isReference := VolumeLabels["dev.heedlesssoap.bluecorridor.volume.reference"]
	return isReference, ref
}
