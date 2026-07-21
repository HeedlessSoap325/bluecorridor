package commands

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/moby/moby/client"
)

func exportCMD(args []string) error {
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

	images, err := apiClient.ImageList(ctx, client.ImageListOptions{All: true})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error occured while listing docker images: %s\n", err)
		os.Exit(1)
	}

	for _, image := range images.Items {
		inspect, err := apiClient.ImageInspect(ctx, image.ID, client.ImageInspectWithManifests(true))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error occured while inspecting volume: %s\n", err)
			os.Exit(1)
		}
		state.Images = append(state.Images, inspect)
	}

	// TODO: Create dummy container to dump the volume content
	volumes, err := apiClient.VolumeList(ctx, client.VolumeListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error occured while listing docker volumes: %s\n", err)
		os.Exit(1)
	}

	for _, volume := range volumes.Items {
		inspect, err := apiClient.VolumeInspect(ctx, volume.Name, client.VolumeInspectOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error occured while inspecting volume: %s\n", err)
			os.Exit(1)
		}
		state.Volumes = append(state.Volumes, inspect)
	}

	networks, err := apiClient.NetworkList(ctx, client.NetworkListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error occured while listing docker networks: %s\n", err)
		os.Exit(1)
	}

	for _, network := range networks.Items {
		inspect, err := apiClient.NetworkInspect(ctx, network.ID, client.NetworkInspectOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error occured while inspecting network: %s\n", err)
			os.Exit(1)
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
