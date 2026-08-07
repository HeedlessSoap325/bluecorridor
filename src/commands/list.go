package commands

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/heedlesssoap325/bluecorridor/internal/console"
	"github.com/heedlesssoap325/bluecorridor/internal/docker"
)

func handleList(args []string) error {
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

	var totalExportSize int64 = 0

	containers, err := docker.ContainerList(nil)
	if err != nil {
		return err
	}

	if !*quiet {
		fmt.Println("Containers:")
	}

	if len(containers) <= 0 {
		fmt.Println("    No Containers found")
	} else {
		for _, container := range containers {
			containerName, _ := strings.CutPrefix(container.Names[0], "/")
			fmt.Fprintf(os.Stdout, "    %s\n", containerName)
		}
	}

	volumes, err := docker.VolumeList(nil)
	if err != nil {
		return err
	}

	fmt.Println()
	if !*quiet {
		fmt.Println("Volumes:")
	}

	if len(volumes) <= 0 {
		fmt.Println("    No volumes found")
	} else {
		for _, volume := range volumes {
			volumeSize, err := docker.VolumeSize(volume.Name)
			if err != nil {
				return err
			}

			fmt.Fprintf(os.Stdout, "    %s (%s)\n", volume.Name, console.FormatBytes(volumeSize))
			totalExportSize += volumeSize
		}
	}

	images, err := docker.ImageList(nil)
	if err != nil {
		return err
	}

	fmt.Println()
	if !*quiet {
		fmt.Println("Images:")
	}

	if len(images) <= 0 {
		fmt.Println("    No images found")
	} else {
		for _, image := range images {
			imageName := "<none>:<none>"
			if len(image.RepoTags) > 0 {
				imageName = image.RepoTags[0]
			}

			method, _ := docker.DetermineTransferMethod(image.RepoTags, image.RepoDigests)
			switch method {
			case docker.MethodPull:
				fmt.Fprintf(os.Stdout, "    %s (pullable)\n", image.RepoTags[0])

			case docker.MethodSaveLoad:
				inspect, err := docker.ImageInspect(image.ID)
				if err != nil {
					return err
				}

				fmt.Fprintf(os.Stdout, "    %s (>=%s)\n", image.RepoTags[0], console.FormatBytes(inspect.Size))
				totalExportSize += inspect.Size
			}
		}
	}

	networks, err := docker.NetworkList(nil)
	if err != nil {
		return err
	}

	fmt.Println()
	if !*quiet {
		fmt.Println("Networks:")
	}

	if len(networks)-3 <= 0 { // Don't show Built-In networks, and there are three of them
		fmt.Println("    No custom networks found")
	} else {
		for _, network := range networks {
			if !docker.NetworkNameReserved(network.Name) {
				fmt.Fprintf(os.Stdout, "    %s\n", network.Name)
			}
		}
	}

	if !*quiet {
		fmt.Println()
		fmt.Fprintf(os.Stdout, "Estimated size of uncompressed export file: %s\n", console.FormatBytes(totalExportSize))
		fmt.Println("The above estimate may be noticeably below or above the actual size due to, among other factors, compression.")
	}

	return nil
}
