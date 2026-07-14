package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/moby/moby/client"
)

type DockerState struct {
	Containers []client.ContainerInspectResult `json:"containers"`
}

func exportCMD(args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
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

	containers, err := apiClient.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error occured while listing docker containers: %s\n", err)
		os.Exit(1)
	}

	var state DockerState
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

	os.WriteFile("docker-export.json", data, 0644)
}