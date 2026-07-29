package commands

import "github.com/moby/moby/client"

const volumeDirName string = "volumes"
const metadataFileName string = "metadata.json"

const exportVersion string = "0.5"

// NOTE: The simple JSON File (deprecated in 61d84f2b9064d6bab3e5ef315bf359690a6f4243) is not considered a propper export format and therefore not given its own version.
// Versions:
//    0.5: tar archive, compressed with gz, metadata.json in root, volumecontents are stored in a tarball in the volumes directory

type dockerState struct {
	Version    string                          `json:"version"`
	Images     []client.ImageInspectResult     `json:"images"`
	Volumes    []client.VolumeInspectResult    `json:"volumes"`
	Networks   []client.NetworkInspectResult   `json:"networks"`
	Containers []client.ContainerInspectResult `json:"containers"`
}
