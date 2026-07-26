package commands

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/heedlesssoap325/bluecorridor/internal/docker"
	"github.com/moby/moby/client"
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

	ctx := context.Background()
	apiClient, err := client.New(client.FromEnv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error occured while creating docker API client: %s\n", err)
		os.Exit(1)
	}
	defer apiClient.Close()

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
		inspect, err := docker.VolumeInspect(volume.Name)
		if err != nil {
			return err
		}

		state.Volumes = append(state.Volumes, inspect)
	}

	networks, err := docker.NetworkList(nil)
	if err != nil {
		return err
	}

	for _, network := range networks {
		inspect, err := docker.NetworkInspect(network.ID)
		if err != nil {
			return err
		}

		state.Networks = append(state.Networks, inspect)
	}

	containers, err := apiClient.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error occured while listing docker containers: %s\n", err)
		os.Exit(1)
	}

	for _, container := range containers.Items {
		inspect, err := apiClient.ContainerInspect(ctx, container.ID, client.ContainerInspectOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error occured while inspecting container: %s\n", err)
			os.Exit(1)
		}
		state.Containers = append(state.Containers, inspect)
	}

	data, err := json.MarshalIndent(state, "", "    ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error occured while creating JSON: %s\n", err)
		os.Exit(1)
	}

	os.WriteFile(*output, data, 0644)

	return nil
}
