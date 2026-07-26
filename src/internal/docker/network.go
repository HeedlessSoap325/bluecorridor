package docker

import (
	"fmt"

	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

// List all Volumes
func NetworkList(filters client.Filters) ([]network.Summary, error) {
	networks, err := dockerClient.NetworkList(ctx, client.NetworkListOptions{
		Filters: filters,
	})

	if err != nil {
		return nil, fmt.Errorf("Error occured while listing docker networks: %s", err)
	}

	return networks.Items, nil
}

// Inspect an Network
//
// The output is not verbose
func NetworkInspect(networkID string) (client.NetworkInspectResult, error) {
	inspect, err := dockerClient.NetworkInspect(ctx, networkID, client.NetworkInspectOptions{})

	if err != nil {
		return client.NetworkInspectResult{}, fmt.Errorf("Error occured while inspecting docker network: %s", err)
	}

	return inspect, nil
}
