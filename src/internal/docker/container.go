package docker

import (
	"fmt"

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
