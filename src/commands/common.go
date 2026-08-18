package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/heedlesssoap325/bluecorridor/internal/docker"
	"github.com/moby/moby/client"
)

const volumeDirName string = "volumes"
const imageDirName string = "images"
const metadataFileName string = "metadata.json"

const exportVersion string = "1.0"

// NOTE: The simple JSON File (deprecated in 61d84f2b9064d6bab3e5ef315bf359690a6f4243) is not considered a propper export format and therefore not given its own version.
// Versions:
//
//	0.5: tar archive, compressed with gz, metadata.json in root, volumecontents are stored in a tarball in the volumes directory

type imageMetadata struct {
	Name   string                `json:"name"`
	Method docker.TransferMethod `json:"method"`

	// Only available if Method is equal to MethodSaveLoad
	//
	// This will represent the name of the image save file
	ID string `json:"id,omitempty"`

	// Only available if Method is equal to MethodSaveLoad
	//
	// This contains the original repoTags of the image
	RepoTags []string `json:"repotags,omitempty"`

	// Only available if Method is equal to MethodPull
	RepoTag string `json:"repotag,omitempty"`
}

type dockerState struct {
	Version    string                          `json:"version"`
	Images     []imageMetadata                 `json:"images"`
	Volumes    []client.VolumeInspectResult    `json:"volumes"`
	Networks   []client.NetworkInspectResult   `json:"networks"`
	Containers []client.ContainerInspectResult `json:"containers"`
}

// Creates a Temporary directory and returns the directory, the volumes directory, as well as the metadata File
//
// It is the callers responsibility to cleanup the tmpDir afterwards
func getTempPaths() (tmpDir string, metadataFile string, volumeDir string, imageDir string, err error) {
	tmpDir, err = os.MkdirTemp("", "bluecorridor-*")
	if err != nil {
		return "", "", "", "", fmt.Errorf("Error occured while creating temp directory: %s", err)
	}

	volumeDir = filepath.Join(tmpDir, volumeDirName)

	if err := os.MkdirAll(volumeDir, 0755); err != nil {
		return "", "", "", "", fmt.Errorf("Could not create temp volumes directory: %s", err)
	}

	imageDir = filepath.Join(tmpDir, imageDirName)

	if err := os.MkdirAll(imageDir, 0755); err != nil {
		return "", "", "", "", fmt.Errorf("Could not create temp images directory: %s", err)
	}

	metadataFile = filepath.Join(tmpDir, metadataFileName)

	return
}
