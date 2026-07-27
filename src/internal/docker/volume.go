package docker

import (
	"fmt"

	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client"
)

// List all Volumes
func VolumeList(filters client.Filters) ([]volume.Volume, error) {
	volumes, err := dockerClient.VolumeList(ctx, client.VolumeListOptions{
		Filters: filters,
	})

	if err != nil {
		return nil, fmt.Errorf("Error occured while listing docker volumes: %s", err)
	}

	return volumes.Items, nil
}

// Inspect an Volume
func VolumeInspect(volumeID string) (client.VolumeInspectResult, error) {
	inspect, err := dockerClient.VolumeInspect(ctx, volumeID, client.VolumeInspectOptions{})

	if err != nil {
		return client.VolumeInspectResult{}, fmt.Errorf("Error occured while inspecting docker volume: %s", err)
	}

	return inspect, nil
}

func VolumeCreate(opts client.VolumeCreateOptions) error {
	_, err := dockerClient.VolumeCreate(ctx, opts)

	if err != nil {
		return err
	}

	return nil
}

func VolumeAnonymous(VolumeLabels map[string]string) bool {
	_, isAnonymous := VolumeLabels["com.docker.volume.anonymous"]
	return isAnonymous
}
