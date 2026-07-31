package docker

import (
	"bytes"
	"fmt"
	"io"
	"os"

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

func VolumeCreate(opts client.VolumeCreateOptions) error {
	_, err := dockerClient.VolumeCreate(ctx, opts)

	if err != nil {
		return err
	}

	return nil
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

	out, err := os.Create(fmt.Sprintf("%s/%s.tar", outDir, saveName))

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

	buffer, err := os.ReadFile(fmt.Sprintf("%s/%s.tar", inDir, saveName))
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
