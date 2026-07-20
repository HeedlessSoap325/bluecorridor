package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

func importCMD(args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	file := flag.String("file", "docker-export.json", "The file from which to import docker")
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

	var state DockerState

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
		res, err := apiClient.ImagePull(ctx, inspect.RepoTags[0], client.ImagePullOptions{})

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error occured while pulling docker image: %s\n", err)
			os.Exit(1)
		}

		msgs := res.JSONMessages(ctx)
		windowHeight := 5
		window := make([]string, 0, windowHeight)
		printed := 0
		previousMsg := ""

		for m := range msgs {
			if m.Progress != nil {
				var s string
				if m.Progress.HideCounts {
					s = fmt.Sprintf("%s %d%s", m.Status, m.Progress.Current, m.Progress.Units)
				} else {
					var unit string
					if unit = "B"; m.Progress.Units != "" {
						unit = m.Progress.Units
					}
					s = fmt.Sprintf("%s %d%s/%d%s", m.Status, m.Progress.Current, unit, m.Progress.Total, unit)
				}

				if m.Status == previousMsg {
					window[printed-1] = s // This is okay, because prviosMsg will be empty, unless printed is >= 1
				} else {
					window = append(window, s)
				}
			} else if m.Status != previousMsg {
				window = append(window, m.Status)
			}
			previousMsg = m.Status

			if len(window) > windowHeight {
				window = window[1:]
			}

			// Move cursor back to the start of the previous window
			if printed > 0 {
				fmt.Fprintf(os.Stdout, "\033[%dA", printed)
			}

			for _, line := range window {
				// Print line, but clear the line beforehand
				fmt.Fprintf(os.Stdout, "\033[2K\033[38;5;245m    %s\033[0m\n", line)
			}
			printed = len(window)
		}

		printed += 1 // Also clear initial "Pulling Image ..." text
		if printed > 0 {
			// Move cursor back to the start of the previous window
			fmt.Fprintf(os.Stdout, "\033[%dA", printed)

			// Clear the lines of the previous window
			for i := 0; i < printed; i++ {
				fmt.Fprintf(os.Stdout, "\033[2K\n")
			}

			// Move cursor back to the start of the previous window
			fmt.Fprintf(os.Stdout, "\033[%dA", printed)
		}

		fmt.Fprintf(os.Stdout, "Successfully pulled image '%s'\n", inspect.RepoTags[0])
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
}
