package commands

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/heedlesssoap325/bluecorridor/internal/docker"
	"github.com/heedlesssoap325/bluecorridor/internal/printing"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

func handleImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	file := fs.String("file", "docker-export.json", "The file from which to import docker")
	help := fs.Bool("help", false, "Print this message")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s import [options]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Options:")
		fs.PrintDefaults()
	}

	fs.Parse(args)

	if *help {
		fs.Usage()
	}

	raw, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error occured while reading File %s: %s\n", *file, err)
		os.Exit(1)
	}

	var state dockerState

	if json.Unmarshal(raw, &state) != nil {
		fmt.Fprintf(os.Stderr, "Error occured while parsing JSON: %s\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	apiClient, err := client.New(client.FromEnv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error occured while creating docker API client: %s\n", err)
		os.Exit(1)
	}
	defer apiClient.Close()

	for _, inspect := range state.Images {
		if len(inspect.RepoTags) <= 0 {
			fmt.Fprintln(os.Stderr, "UNIMPLEMENTED: Image had no RepoTags")
			continue
		}

		fmt.Fprintf(os.Stdout, "Pulling Image '%s'\n", inspect.RepoTags[0])

		// TODO: The code assumes the images are pullable!
		// In the future, the code should check Image availability and otherwise fall back on the image save in the export
		err := docker.ImagePull(inspect.RepoTags[0], true)
		if err != nil {
			return err
		}

		printing.ClearCurrentLine() // Clear the "Pulling Image ..." line
		printing.PrintWithColoredForeground(os.Stdout, printing.SUCCESS, "Successfully pulled image '%s'", inspect.RepoTags[0])
	}

	for _, inspect := range state.Containers {
		_, err := apiClient.ContainerCreate(ctx, client.ContainerCreateOptions{
			Config:     inspect.Container.Config,
			HostConfig: inspect.Container.HostConfig,
			NetworkingConfig: &network.NetworkingConfig{
				EndpointsConfig: inspect.Container.NetworkSettings.Networks,
			},
			Platform: nil,
			Name:     inspect.Container.Name,
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error occured while creating docker container: %s\n", err)
			os.Exit(1)
		}
	}

	return nil
}
