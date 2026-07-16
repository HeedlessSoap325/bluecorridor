package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/moby/moby/client"
)

func listCMD(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	quiet := fs.Bool("quiet", false, "Print a quiet output, with the resources seperated by an empty line")
	help := fs.Bool("help", false, "Print this message")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s list [options]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Options:")
		fs.PrintDefaults()
	}

	fs.Parse(args)

	if *help {
		fs.Usage()
	}

	ctx := context.Background()
	apiClient, err := client.New(client.FromEnv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error occured while creating docker API client: %s\n", err)
		os.Exit(1)
	}
	defer apiClient.Close()

	containers, err := apiClient.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error occured while listing docker containers: %s\n", err)
		os.Exit(1)
	}

	if !*quiet {
		fmt.Println("Containers:")
	}

	if len(containers.Items) <= 0 {
		fmt.Println("    No Containers found")
	} else {
		for _, container := range containers.Items {
			fmt.Fprintf(os.Stdout, "    %s\n", container.Names[0])
		}
	}

	volumes, err := apiClient.VolumeList(ctx, client.VolumeListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error occured while listing docker volumes: %s\n", err)
		os.Exit(1)
	}

	fmt.Println()
	if !*quiet {
		fmt.Println("Volumes:")
	}

	if len(volumes.Items) <= 0 {
		fmt.Println("    No volumes found")
	} else {
		for _, volume := range volumes.Items {
			fmt.Fprintf(os.Stdout, "    %s\n", volume.Name)
		}
	}

	images, err := apiClient.ImageList(ctx, client.ImageListOptions{All: true})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error occured while listing docker images: %s\n", err)
		os.Exit(1)
	}

	fmt.Println()
	if !*quiet {
		fmt.Println("Images:")
	}

	if len(images.Items) <= 0 {
		fmt.Println("    No images found")
	} else {
		for _, image := range images.Items {
			fmt.Fprintf(os.Stdout, "    %s\n", image.RepoTags[0])
		}
	}

	networks, err := apiClient.NetworkList(ctx, client.NetworkListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error occured while listing docker networks: %s\n", err)
		os.Exit(1)
	}

	fmt.Println()
	if !*quiet {
		fmt.Println("Networks:")
	}

	if len(networks.Items) <= 0 {
		fmt.Println("    No networks found")
	} else {
		for _, network := range networks.Items {
			fmt.Fprintf(os.Stdout, "    %s\n", network.Name)
		}
	}
}
