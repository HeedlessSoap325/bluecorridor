package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/heedlesssoap325/bluecorridor/internal/compression"
	"github.com/heedlesssoap325/bluecorridor/internal/docker"
	"github.com/heedlesssoap325/bluecorridor/internal/printing"
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

	/// EXTRACT TAR ARCHIVE INTO TEMPORARY DIRECTORY
	compression.Untar(*file, inputDir)

	/// RESTORE DOCKER STATE FROM METADATA FILE
	metadataFile := fmt.Sprintf("%s/metadata.json", inputDir)
	raw, err := os.ReadFile(metadataFile)
	if err != nil {
		return fmt.Errorf("Error occured while reading metadata file %s: %s", metadataFile, err)
	}

	volumeDir := fmt.Sprintf("%s/volumes", inputDir)

	var state dockerState
	if json.Unmarshal(raw, &state) != nil {
		return fmt.Errorf("Error occured while parsing JSON: %s", err)
	}

	if err := importDockerState(state); err != nil {
		return err
	}

	/// RESTORE VOLUME CONTENTS
	restoreVolumes(state.Volumes, volumeDir)

	/// CLEANUP
	err = os.RemoveAll(inputDir)
	if err != nil {
		return fmt.Errorf("Error occured while cleaning up temporary workinDir %s: %s", inputDir, err)
	}

	return nil
}

func importDockerState(state dockerState) error {
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

		printing.MoveCursorUpNLines(1)
		printing.ClearCurrentLine() // Clear the "Pulling Image ..." line
		printing.PrintWithColoredForeground(os.Stdout, printing.SUCCESS, "Successfully pulled image '%s'", inspect.RepoTags[0])
	}

	for _, inspect := range state.Volumes {
		if docker.VolumeAnonymous(inspect.Volume.Labels) {
			printing.PrintWithColoredForeground(os.Stdout, printing.WARNING, "Volume '%s' is anonymous, can't recreate", inspect.Volume.Name)
			continue
		}

		fmt.Fprintf(os.Stdout, "Creating volume '%s'\n", inspect.Volume.Name)

		var clusterVolumeSpec *volume.ClusterVolumeSpec
		if inspect.Volume.ClusterVolume != nil {
			clusterVolumeSpec = &inspect.Volume.ClusterVolume.Spec
		}

		err := docker.VolumeCreate(client.VolumeCreateOptions{
			Name:              inspect.Volume.Name,
			Driver:            inspect.Volume.Driver,
			DriverOpts:        inspect.Volume.Options,
			Labels:            inspect.Volume.Labels,
			ClusterVolumeSpec: clusterVolumeSpec,
		})

		if err != nil {
			return err
		}

		printing.MoveCursorUpNLines(1)
		printing.ClearCurrentLine() // Clear "Creating volume ..." line
		printing.PrintWithColoredForeground(os.Stdout, printing.SUCCESS, "Successfully created volume '%s'", inspect.Volume.Name)
	}

	for _, inspect := range state.Networks {
		if docker.NetworkNameReserved(inspect.Network.Name) {
			printing.PrintWithColoredForeground(os.Stdout, printing.WARNING, "Network '%s' is a built-in network. Can't use that name. Skiping", inspect.Network.Name)
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

		printing.MoveCursorUpNLines(1)
		printing.ClearCurrentLine() // Clear "Creating network ..." line
		printing.PrintWithColoredForeground(os.Stdout, printing.SUCCESS, "Successfully created network '%s': %s", inspect.Network.Name, id)
	}

	for _, inspect := range state.Containers {
		id, err := docker.ContainerCreate(client.ContainerCreateOptions{
			Config:           inspect.Container.Config,
			HostConfig:       inspect.Container.HostConfig,
			NetworkingConfig: nil, // Connect to networks later
			Platform:         nil,
			Name:             inspect.Container.Name,
		})

		if err != nil {
			return err
		}

		printing.PrintWithColoredForeground(os.Stdout, printing.SUCCESS, "Successfully created container '%s': %s", inspect.Container.Name, id)

		// If the client or daemon version were below 1.44, passing multiple networks for container creation would result in a error or wrong configuration of the container
		// This approach of itterating all networks the container was connected to and re-connecting them works with all versions, and is herefor more compatible, even tough prbably never actually necessary
		for network, endpointSettings := range inspect.Container.NetworkSettings.Networks {
			err := docker.NetworkConnect(network, client.NetworkConnectOptions{
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
		fmt.Fprintf(os.Stdout, "Restoring contents of volume '%s'\n", volume.Volume.Name)
		err := docker.VolumeRestore(volume.Volume.Name, volume.Volume.Name, inDir)

		if err != nil {
			return err
		}

		printing.MoveCursorUpNLines(1)
		printing.ClearCurrentLine() // Clear "Restoring contents of volume ..." line
		printing.PrintWithColoredForeground(os.Stdout, printing.SUCCESS, "Successfully restored contents of volume '%s'", volume.Volume.Name)
	}

	return nil
}
