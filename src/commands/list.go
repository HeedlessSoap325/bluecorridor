package commands

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/heedlesssoap325/bluecorridor/internal/console"
	"github.com/heedlesssoap325/bluecorridor/internal/docker"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/volume"
)

func handleList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	estimate_size := fs.Bool("estimate_size", false, "Estimate the size of the final export\nTHIS IS SLOW AND POTENTIALLY INACCURATE")
	help := fs.Bool("help", false, "Print this message")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s list [options]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Options:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("error occured while parsing flags: %s", err)
	}

	if *help {
		fs.Usage()
	}

	if err := listContainers(); err != nil {
		return err
	}

	if err := listVolumes(false); err != nil { // TODO: give the user the option to display the size as well
		return err
	}

	if err := listImages(false); err != nil { // TODO: give the user the option to display the pullability as well
		return err
	}

	if err := listNetworks(); err != nil {
		return err
	}

	if *estimate_size {
		fmt.Println()
		exportSize, err := estimateExportSize()
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stdout, "Estimated size of uncompressed export file: %s\n", exportSize)
		fmt.Println("The above estimate may be noticeably below or above the actual size due to, among other factors, compression.")
	}

	return nil
}

func listContainers() error {
	containers, err := docker.ContainerList(nil)
	if err != nil {
		return err
	}

	titles := []string{"NAMES", "ID", "STATE", "STATUS"}

	rows := make([][]string, 0, len(containers))
	for _, container := range containers {
		rows = append(rows, []string{
			containerNames(container),
			container.ID[:12],
			string(container.State),
			container.Status,
		})
	}

	fmt.Printf("\n### CONTAINERS ###\n\n")
	console.PrintTable(titles, rows, 3)

	return nil
}

func containerNames(c container.Summary) string {
	names := make([]string, len(c.Names))

	for i, name := range c.Names {
		names[i] = strings.TrimPrefix(name, "/")
	}

	return strings.Join(names, ", ")
}

func listVolumes(includeSize bool) error {
	volumes, err := docker.VolumeList(nil)
	if err != nil {
		return err
	}

	titles := []string{"NAME", "SIZE", "SCOPE"}

	rows := make([][]string, 0, len(volumes))
	for _, volume := range volumes {

		rows = append(rows, []string{
			volume.Name,
			volumeSize(volume, includeSize),
			volume.Scope,
		})
	}

	fmt.Printf("\n### VOLUMES ###\n\n")
	console.PrintTable(titles, rows, 3)

	return nil

}

func volumeSize(v volume.Volume, includeSize bool) string {
	if !includeSize {
		return "unknown"
	}

	volumeSize, err := docker.VolumeSize(v.Name)
	if err != nil {
		return "unknown"
	}
	return console.FormatBytes(volumeSize)
}

func listImages(includePullable bool) error {
	images, err := docker.ImageList(nil)
	if err != nil {
		return err
	}

	titles := []string{"IMAGE", "ID", "DISK USAGE", "PULLABLE", "IN USE"}

	rows := make([][]string, 0, len(images))
	for _, image := range images {
		rows = append(rows, []string{
			imageName(image),
			image.ID,
			console.FormatBytes(image.Size),
			imagePullable(image, includePullable),
			imageInUse(image),
		})
	}

	fmt.Printf("\n### IMAGES ###\n\n")
	console.PrintTable(titles, rows, 3)

	return nil
}

func imageName(i image.Summary) string {
	if len(i.RepoTags) == 0 {
		return "<none>:<none>"
	}

	return i.RepoTags[0]
}

func imagePullable(i image.Summary, includePullable bool) string {
	if !includePullable {
		return "unknown"
	}

	switch method, _ := docker.DetermineTransferMethod(i.RepoTags, i.RepoDigests); method {
	case docker.MethodPull:
		return "Yes"
	case docker.MethodSaveLoad:
		return "No"
	default:
		return "unknown"
	}
}

func imageInUse(i image.Summary) string {
	switch {
	case i.Containers == 0:
		return "No"
	case i.Containers > 0:
		return "Yes"
	default:
		return "unknown" // if image.Containers is -1, the field hasn't been set / calculated yet
	}
}

func listNetworks() error {
	networks, err := docker.NetworkList(nil)
	if err != nil {
		return err
	}

	titles := []string{"NAME", "ID", "DRIVER", "SCOPE"}

	rows := make([][]string, 0, len(networks))
	for _, network := range networks {
		if docker.NetworkNameReserved(network.Name) {
			continue
		}

		rows = append(rows, []string{
			network.Name,
			network.ID,
			network.Driver,
			network.Scope,
		})
	}

	fmt.Printf("\n### NETWORKS ###\n\n")
	console.PrintTable(titles, rows, 3)

	return nil
}

func estimateExportSize() (string, error) {
	console.PrintWithColoredForeground(os.Stdout, console.INFO, "Estimating export size...")
	var totalExportSize int64 = 0

	volumes, err := docker.VolumeList(nil)
	if err != nil {
		return "", err
	}

	for _, volume := range volumes {
		volumeSize, err := docker.VolumeSize(volume.Name)
		if err != nil {
			return "", err
		}

		totalExportSize += volumeSize
	}

	images, err := docker.ImageList(nil)
	if err != nil {
		return "", err
	}

	for _, image := range images {
		switch method, _ := docker.DetermineTransferMethod(image.RepoTags, image.RepoDigests); method {
		case docker.MethodPull:

		case docker.MethodSaveLoad:
			totalExportSize += image.Size
		default:
			return "", fmt.Errorf("Pullability of image was unknown")
		}
	}

	console.ClearNLinesAndPositionCursorAtStart(1)

	return console.FormatBytes(totalExportSize), nil
}
