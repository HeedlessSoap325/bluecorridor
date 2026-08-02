package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
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
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	file := fs.String("file", "docker-export.tar.gz", "The file from which to import docker")
	help := fs.Bool("help", false, "Print this message")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s import [options]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Options:")
		fs.PrintDefaults()
	}

	fs.Parse(args)

	if *help {
		fs.Usage()
	}

	/// CREATE TEMPORARY DIRECTORY
	inputDir := filepath.Base(*file) // TODO: Maybe use os.TempDir() here, instead of hoping that the folder is empty

	err := os.MkdirAll(inputDir, 0755)
	if err != nil {
		return fmt.Errorf("Could not create input directory %s: %s", inputDir, err)
	}

	defer os.RemoveAll(inputDir) // CLEANUP

	/// EXTRACT TAR ARCHIVE INTO TEMPORARY DIRECTORY
	compression.Untar(*file, inputDir)

	/// RESTORE DOCKER STATE FROM METADATA FILE
	metadataFile := fmt.Sprintf("%s/%s", inputDir, metadataFileName)
	raw, err := os.ReadFile(metadataFile)
	if err != nil {
		return fmt.Errorf("Error occured while reading metadata file %s: %s", metadataFile, err)
	}

	volumeDir := fmt.Sprintf("%s/%s", inputDir, volumeDirName)

	var state dockerState
	if json.Unmarshal(raw, &state) != nil {
		return fmt.Errorf("Error occured while parsing JSON: %s", err)
	}

	if state.Version != exportVersion {
		return fmt.Errorf("Incompatible metadata versions. Export: %s, this programm: %s\nThe file provided is either to new or to old for this programm", state.Version, exportVersion)
	}

	if err := importDockerState(&state); err != nil {
		return err
	}

	/// RESTORE VOLUME CONTENTS
	restoreVolumes(state.Volumes, volumeDir)

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
	for idx, inspect := range state.Volumes {
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

		volumeName := inspect.Volume.Name
		if docker.VolumeAnonymous(inspect.Volume.Labels) {
			volumeName = "" // This creates a anonymous volume
		}

		volume, err := docker.VolumeCreate(client.VolumeCreateOptions{
			Name:              volumeName,
			Driver:            inspect.Volume.Driver,
			DriverOpts:        inspect.Volume.Options,
			Labels:            volumeLabels,
			ClusterVolumeSpec: clusterVolumeSpec,
		})

		if err != nil {
			return err
		}

		origName := inspect.Volume.Name
		if isReference, reference := docker.VolumeReference(inspect.Volume.Labels); isReference {
			origName = reference
		}

		volumeMap[origName] = volume.Name

		if docker.VolumeAnonymous(inspect.Volume.Labels) {
			state.Volumes[idx].Volume.Name = volume.Name // Set name to the newly created anonymous volume
		}

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

		// Override the containers old mounts to consider the potential changes with anonymous volumes
		newMounts := make([]mount.Mount, 0, len(inspect.Container.Mounts))
		for _, mountPoint := range inspect.Container.Mounts {
			switch mountPoint.Type {
			case mount.TypeVolume:
				newName, ok := volumeMap[mountPoint.Name] // old name (or old anonymous ID) -> new volume name
				if !ok {
					return fmt.Errorf("no mapping found for volume %q (dest %s)", mountPoint.Name, mountPoint.Destination)
				}

				newMounts = append(newMounts, mount.Mount{
					Type:   mount.TypeVolume,
					Source: newName, // new volume name
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
	// TODO: handle anonymous and dataless volumes
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
