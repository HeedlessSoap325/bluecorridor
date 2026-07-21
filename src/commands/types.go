package commands

import "github.com/moby/moby/client"

type dockerState struct {
	Images     []client.ImageInspectResult     `json:"images"`
	Volumes    []client.VolumeInspectResult    `json:"volumes"`
	Networks   []client.NetworkInspectResult   `json:"networks"`
	Containers []client.ContainerInspectResult `json:"containers"`
}
