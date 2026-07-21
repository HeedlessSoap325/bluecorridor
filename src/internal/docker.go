package internal

import (
	"context"
	"fmt"

	"github.com/moby/moby/client"
)

var ctx context.Context
var dockerClient *client.Client

// Initialize the internal dockerClient
//
// Must be called before any other docker operations can be executed
func InitializeDockerClient() error {
	// Deinitialize previous docker API client if it exists
	if dockerClient != nil {
		DeinitializeDockerClient()
	}

	var err error

	ctx = context.Background()
	dockerClient, err = client.New(client.FromEnv)
	if err != nil {
		return fmt.Errorf("Error occured while creating docker API client: %s\n", err)
	}

	return nil
}

// Deinitialize the internal dockerClient
//
// Must be called before exiting the programm, prefferably in a defer statement right after InitializeDockerClient()
func DeinitializeDockerClient() error {
	return dockerClient.Close()
}
