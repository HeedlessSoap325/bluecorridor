package docker

import (
	"context"
	"fmt"
	"os"

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

	if value, _ := os.LookupEnv("DOCKER_HOST"); value == "" {
		fmt.Print("\033[38;5;214m")
		fmt.Println("[WARNING] Your environment does not appear to provide the DOCKER_HOST environment variable")
		fmt.Println("[WARNING] The DOCKER_HOST is used to connect to the right docker socket, without it you may see different resources than you expect or none at all")
		fmt.Printf("[WARNING] The default DOCKER_HOST value that will be used is '%s'\n", client.DefaultDockerHost)
		fmt.Println("[WARNING] To avoid this warning, please export DOCKER_HOST in your environment")
		fmt.Println("[WARNING] If you are unsure on which docker host is the right one, you can run 'docker context ls' to see your current and all other available hosts")
		fmt.Print("\033[0m")
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
