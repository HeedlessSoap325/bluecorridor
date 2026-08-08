package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"maps"
	"os"
	"strings"

	"github.com/heedlesssoap325/bluecorridor/internal/compression"
	"github.com/heedlesssoap325/bluecorridor/internal/console"
	"github.com/heedlesssoap325/bluecorridor/internal/docker"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client"
)

func handleImport(args []string) error {
	/// HANDLE FLAGS
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	file := fs.String("file", "docker-export.tar.gz", "The file from which to import docker")
	help := fs.Bool("help", false, "Print this message")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s import [options]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Options:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("error occured while parsing flags: %s", err)
	}

	if *help {
		fs.Usage()
		return nil
	}

	/// CREATE TEMPORARY DIRECTORY
	tmpDir, metadataFile, volumeDir, err := getTempPaths()
	if err != nil {
		return err
	}

	defer os.RemoveAll(tmpDir) // CLEANUP

	/// EXTRACT TAR ARCHIVE INTO TEMPORARY DIRECTORY
	if err := compression.Untar(*file, tmpDir); err != nil {
		return fmt.Errorf("Error occured while untaring file: %s", err)
	}

	/// RESTORE DOCKER STATE FROM METADATA FILE
	raw, err := os.ReadFile(metadataFile)
	if err != nil {
		return fmt.Errorf("Error occured while reading metadata file %s: %s", metadataFile, err)
	}

	var state dockerState
	if err := json.Unmarshal(raw, &state); err != nil {
		return fmt.Errorf("Error occured while parsing JSON: %s", err)
	}

	if state.Version != exportVersion {
		return fmt.Errorf("Incompatible metadata versions. Export: %s, this programm: %s\nThe file provided is either to new or to old for this programm", state.Version, exportVersion)
	}

	if err := importDockerState(&state); err != nil {
		return err
	}

	/// RESTORE VOLUME CONTENTS
	if err:= restoreVolumes(state.Volumes, volumeDir); err != nil {
		return err
	}

	return nil
}

func importDockerState(state *dockerState) error {
	for _, inspect := range state.Images {
		if len(inspect.RepoTags) <= 0 {
			fmt.Fprintln(os.Stderr, "UNIMPLEMENTED: Image had no RepoTags")
			continue
		}

		fmt.Fprintf(os.Stdout, "Pulling Image '%s'\n", inspect.RepoTags[0])

		// TODO: The code assumes the images are pullable!
		// In the future, the code should check Image availability and otherwise fall back on the image save in the export
		err := docker.ImagePull(inspect.RepoTags[0], true)
		if err != nil {
			return err
		}

		console.MoveCursorUpNLines(1)
		console.ClearCurrentLine() // Clear the "Pulling Image ..." line
		console.PrintWithColoredForeground(os.Stdout, console.SUCCESS, "Successfully pulled image '%s'", inspect.RepoTags[0])
	}

	volumeMap := make(map[string]string)
	for _, inspect := range state.Volumes {
		originalName := inspect.Volume.Name
		if isReference, reference := docker.VolumeReference(inspect.Volume.Labels); isReference {
			originalName = reference
		}

		if docker.VolumeAnonymous(inspect.Volume.Labels) {
			volumeMap[originalName] = "" // This will cause docker to create a new truely anonymous volume when the container gets created
			continue                     // Don't do anything else here
		}

		fmt.Fprintf(os.Stdout, "Re-Creating volume '%s'\n", inspect.Volume.Name)

		var clusterVolumeSpec *volume.ClusterVolumeSpec
		if inspect.Volume.ClusterVolume != nil {
			clusterVolumeSpec = &inspect.Volume.ClusterVolume.Spec
		}

		volumeLabels := make(map[string]string, len(inspect.Volume.Labels))
		maps.Copy(volumeLabels, inspect.Volume.Labels)

		// Delete labels assigned by bluecorridor for metadata file only, so they don't get re-created
		delete(volumeLabels, "dev.heedlesssoap.bluecorridor.volume.dataless")
		delete(volumeLabels, "dev.heedlesssoap.bluecorridor.volume.reference")

		volume, err := docker.VolumeCreate(client.VolumeCreateOptions{
			Name:              inspect.Volume.Name,
			Driver:            inspect.Volume.Driver,
			DriverOpts:        inspect.Volume.Options,
			Labels:            volumeLabels,
			ClusterVolumeSpec: clusterVolumeSpec,
		})

		if err != nil {
			return err
		}

		volumeMap[originalName] = volume.Name // Map the original name present in the exports to the new name present on the host

		console.MoveCursorUpNLines(1)
		console.ClearCurrentLine() // Clear "Creating volume ..." line
		console.PrintWithColoredForeground(os.Stdout, console.SUCCESS, "Successfully created volume '%s'", volume.Name)
	}

	networkNameToID := make(map[string]string)
	for _, inspect := range state.Networks {
		if docker.NetworkNameReserved(inspect.Network.Name) {
			console.PrintWithColoredForeground(os.Stdout, console.WARNING, "Network '%s' is a built-in network. Can't use that name. Skiping", inspect.Network.Name)
			continue
		}

		fmt.Fprintf(os.Stdout, "Creating network '%s'\n", inspect.Network.Name)

		id, err := docker.NetworkCreate(inspect.Network.Name, client.NetworkCreateOptions{
			Driver:     inspect.Network.Driver,
			Scope:      inspect.Network.Scope,
			EnableIPv4: &inspect.Network.EnableIPv4,
			EnableIPv6: &inspect.Network.EnableIPv6,
			IPAM:       &inspect.Network.IPAM,
			Internal:   inspect.Network.Internal,
			Attachable: inspect.Network.Attachable,
			Ingress:    inspect.Network.Ingress,
			ConfigOnly: inspect.Network.ConfigOnly,
			ConfigFrom: inspect.Network.ConfigFrom.Network,
			Options:    inspect.Network.Options,
			Labels:     inspect.Network.Labels,
		})

		if err != nil {
			return err
		}

		networkNameToID[inspect.Network.Name] = id

		console.MoveCursorUpNLines(1)
		console.ClearCurrentLine() // Clear "Creating network ..." line
		console.PrintWithColoredForeground(os.Stdout, console.SUCCESS, "Successfully created network '%s': %s", inspect.Network.Name, id)
	}

	for _, inspect := range state.Containers {
		containerName, _ := strings.CutPrefix(inspect.Container.Name, "/")

		anonymousMounts := make(map[string]string)
		newMounts := make([]mount.Mount, 0, len(inspect.Container.Mounts))

		// Override the containers old mounts to consider the potential changes with anonymous volumes
		for _, mountPoint := range inspect.Container.Mounts {
			switch mountPoint.Type {
			case mount.TypeVolume:
				newName, ok := volumeMap[mountPoint.Name] // old name (or old anonymous ID) -> new volume name (or empty for anonymous volumes)
				if !ok {
					return fmt.Errorf("no mapping found for volume %q (dest %s)", mountPoint.Name, mountPoint.Destination)
				}
				if newName == "" {
					anonymousMounts[mountPoint.Name] = mountPoint.Destination
				}

				newMounts = append(newMounts, mount.Mount{
					Type:   mount.TypeVolume,
					Source: newName, // new volume name (docker will create a anonymous volume when this is empty)
					Target: mountPoint.Destination,
				})
			case mount.TypeBind:
				// keep as-is
				newMounts = append(newMounts, mount.Mount{
					Type:     mount.TypeBind,
					Source:   mountPoint.Source,
					Target:   mountPoint.Destination,
					ReadOnly: !mountPoint.RW,
				})
			default:
				return fmt.Errorf("UNSUPPORTED: volume of type %s can't be re-created (yet)", mountPoint.Type)
			}
		}

		inspect.Container.HostConfig.Mounts = newMounts
		inspect.Container.HostConfig.Binds = nil // clear legacy Binds so it doesn't fight with Mounts

		id, err := docker.ContainerCreate(client.ContainerCreateOptions{
			Config:           inspect.Container.Config,
			HostConfig:       inspect.Container.HostConfig,
			NetworkingConfig: nil, // Connect to networks later
			Platform:         nil,
			Name:             containerName,
		})

		if err != nil {
			return err
		}

		console.PrintWithColoredForeground(os.Stdout, console.SUCCESS, "Successfully created container '%s': %s", containerName, id)

		inspect, err := docker.ContainerInspect(id)
		if err != nil {
			return err
		}

		// This searches for all newly created anonymous volumes (identified by their destination)
		// and then replaces the old name from the export with the name of the newly created truely anonymous volume
		for originalName, dest := range anonymousMounts {
			for _, mount := range inspect.Container.Mounts {
				if mount.Destination == dest {
					for idx := range state.Volumes {
						if state.Volumes[idx].Volume.Name == originalName {
							state.Volumes[idx].Volume.Name = mount.Name
						}
					}
				}
			}
		}

		// If the client or daemon version were below 1.44, passing multiple networks for container creation would result in a error or wrong configuration of the container
		// This approach of itterating all networks the container was connected to and re-connecting them works with all versions, and is herefor more compatible, even tough prbably never actually necessary
		for networkName, endpointSettings := range inspect.Container.NetworkSettings.Networks {
			newNetworkID, ok := networkNameToID[networkName]
			if !ok {
				// e.g. it was a reserved/skipped network like "bridge"/"host"/"none"
				newNetworkID = networkName // fall back to connecting by name
			}

			// Strip stale identifiers from the old host before reusing the struct
			// these will be filled in by the docker deamon once the container starts up
			endpointSettings.NetworkID = ""
			endpointSettings.EndpointID = ""

			err := docker.NetworkConnect(newNetworkID, client.NetworkConnectOptions{
				Container:      id,
				EndpointConfig: endpointSettings,
			})

			if err != nil {
				return err
			}
		}
	}

	return nil
}

func restoreVolumes(volumeInspects []client.VolumeInspectResult, inDir string) error {
	for _, volume := range volumeInspects {
		if docker.VolumeDataless(volume.Volume.Labels) {
			continue // No data to import so just skip it
		}

		volumeName := volume.Volume.Name
		saveName := volume.Volume.Name

		if isReference, reference := docker.VolumeReference(volume.Volume.Labels); isReference {
			saveName = reference
		}

		fmt.Fprintf(os.Stdout, "Restoring contents of volume '%s'\n", saveName)

		err := docker.VolumeRestore(volumeName, saveName, inDir)
		if err != nil {
			return err
		}

		console.MoveCursorUpNLines(1)
		console.ClearCurrentLine() // Clear "Restoring contents of volume ..." line
		console.PrintWithColoredForeground(os.Stdout, console.SUCCESS, "Successfully restored contents of volume '%s'", saveName)
	}

	return nil
}
