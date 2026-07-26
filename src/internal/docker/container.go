package docker

import (
	"fmt"
	"os"

	"github.com/heedlesssoap325/bluecorridor/internal/printing"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

// List all Containers
func ContainerList(filters client.Filters) ([]container.Summary, error) {
	containers, err := dockerClient.ContainerList(ctx, client.ContainerListOptions{
		Size:    false,
		All:     true,
		Filters: filters,
	})

	if err != nil {
		return nil, fmt.Errorf("Error occured while listing docker containers: %s", err)
	}

	return containers.Items, nil
}

// Inspect a Container
//
// The output is not verbose
func ContainerInspect(containerID string) (client.ContainerInspectResult, error) {
	inspect, err := dockerClient.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{
		Size: false,
	})

	if err != nil {
		return client.ContainerInspectResult{}, fmt.Errorf("Error occured while inspecting docker container: %s", err)
	}

	return inspect, nil
}

func ContainerCreate(opts client.ContainerCreateOptions) (string, error) {
	res, err := dockerClient.ContainerCreate(ctx, opts)

	if err != nil {
		return "", fmt.Errorf("Error occured while creating docker container: %s\n", err)
	}

	if len(res.Warnings) > 0 {
		printContainerCreationWarnings(opts.Name, res.Warnings)
	}

	return res.ID, nil
}

func printContainerCreationWarnings(name string, warnings []string) {
	printing.PrintWithColoredForeground(os.Stdout, printing.WARNING, "[WARNING] Warnings occured while creating container '%s'", name)
	for _, warning := range warnings {
		printing.PrintWithColoredForeground(os.Stdout, printing.WARNING, "    %s", warning)
	}
}
