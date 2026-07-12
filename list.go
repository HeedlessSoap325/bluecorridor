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
	verbose := fs.Bool("verbose", false, "Print a verbose output")
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

	fmt.Printf("verbose: %t\n\n\n", *verbose)

	ctx := context.Background()
	apiClient, err := client.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error occured while creating docker API client: %s\n", err)
	}
	defer apiClient.Close()

	containers, err := apiClient.ContainerList(ctx, client.ContainerListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error occured while listing docker containers: %s\n", err)
	}

	if len(containers.Items) <= 0 {
		fmt.Println("No Containers found")
	} else {
		for _, container := range containers.Items {
			fmt.Println(container.ID)
		}
	}
}
