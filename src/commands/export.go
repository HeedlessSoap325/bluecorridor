package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/heedlesssoap325/bluecorridor/internal/docker"
	"github.com/heedlesssoap325/bluecorridor/internal/printing"
)

func handleExport(args []string) error {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	output := fs.String("output", "docker-export.json", "The path in which to place the export file")
	help := fs.Bool("help", false, "Print this message")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s export [options]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Options:")
		fs.PrintDefaults()
	}

	fs.Parse(args)

	if *help {
		fs.Usage()
	}

	var state dockerState

	images, err := docker.ImageList(nil)
	if err != nil {
		return err
	}

	for _, image := range images {
		inspect, err := docker.ImageInspect(image.ID)
		if err != nil {
			return err
		}

		state.Images = append(state.Images, inspect)
	}

	// TODO: Create dummy container to dump the volume content
	volumes, err := docker.VolumeList(nil)
	if err != nil {
		return err
	}

	for _, volume := range volumes {
		if docker.VolumeAnonymous(volume.Labels) {
			printing.PrintWithColoredForeground(os.Stdout, printing.WARNING, "Volume '%s' is anonymous and won't be exported", volume.Name)
			continue // Don't export anonymous volumes
		}

		inspect, err := docker.VolumeInspect(volume.Name)
		if err != nil {
			return err
		}

		state.Volumes = append(state.Volumes, inspect)

		err = docker.VolumeSave(volume.Name, volume.Name, ".")
		if err != nil {
			return err
		}
	}

	networks, err := docker.NetworkList(nil)
	if err != nil {
		return err
	}

	for _, network := range networks {
		if docker.NetworkNameReserved(network.Name) {
			// Don't print a warning here because these networks will always be present, just dont add them to the export
			continue // Don't export networks "bridge", "host", and "none" as the will always exist on the other device, because they are built-in
		}

		inspect, err := docker.NetworkInspect(network.ID)
		if err != nil {
			return err
		}

		state.Networks = append(state.Networks, inspect)
	}

	containers, err := docker.ContainerList(nil)
	if err != nil {
		return err
	}

	for _, container := range containers {
		inspect, err := docker.ContainerInspect(container.ID)
		if err != nil {
			return err
		}

		state.Containers = append(state.Containers, inspect)
	}

	data, err := json.MarshalIndent(state, "", "    ")
	if err != nil {
		return fmt.Errorf("Error occured while creating JSON: %s", err)
	}

	os.WriteFile(*output, data, 0644)

	return nil
}
