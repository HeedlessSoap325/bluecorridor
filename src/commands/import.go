package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/heedlesssoap325/bluecorridor/internal/docker"
	"github.com/heedlesssoap325/bluecorridor/internal/printing"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client"
)

func handleImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	file := fs.String("file", "docker-export.json", "The file from which to import docker")
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

	raw, err := os.ReadFile(*file)
	if err != nil {
		return fmt.Errorf("Error occured while reading File %s: %s", *file, err)
	}

	var state dockerState

	if json.Unmarshal(raw, &state) != nil {
		return fmt.Errorf("Error occured while parsing JSON: %s", err)
	}

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

	// TODO: Use data from export to restore the volumes data
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
			Config:     inspect.Container.Config,
			HostConfig: inspect.Container.HostConfig,
			NetworkingConfig: &network.NetworkingConfig{
				EndpointsConfig: inspect.Container.NetworkSettings.Networks,
			},
			Platform: nil,
			Name:     inspect.Container.Name,
		})

		if err != nil {
			return err
		}

		printing.PrintWithColoredForeground(os.Stdout, printing.SUCCESS, "Successfully created container '%s': %s", inspect.Container.Name, id)
	}

	return nil
}
