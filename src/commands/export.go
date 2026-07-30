package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/heedlesssoap325/bluecorridor/internal/compression"
	"github.com/heedlesssoap325/bluecorridor/internal/console"
	"github.com/heedlesssoap325/bluecorridor/internal/docker"
)

func handleExport(args []string) error {
	/// HANDLE FLAGS
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	output := fs.String("output", "docker-export", "The path in which to place the export file (noe extension required)")
	help := fs.Bool("help", false, "Print this message")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s export [options]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Options:")
		fs.PrintDefaults()
	}

	fs.Parse(args)

	if *help {
		fs.Usage()
	}

	/// CREATE TEMPORARY DIRECTORY
	err := os.MkdirAll(*output, 0755) // TODO: Maybe use os.TempDir() here instead of just ssuming that the output folder is empty
	if err != nil {
		return fmt.Errorf("Could not create output directory %s: %s", *output, err)
	}

	defer os.RemoveAll(*output) // CLEANUP

	/// SAVE VOLUMES
	volumeDir := fmt.Sprintf("%s/%s", *output, volumeDirName)
	err = os.MkdirAll(volumeDir, 0755)
	if err != nil {
		return fmt.Errorf("Could not create volumes directory in output directory %s: %s", *output, err)
	}

	saveVolumes(volumeDir)

	/// CREATE METADATA FILE
	var state dockerState
	if err := extractDockerState(&state); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "    ")
	if err != nil {
		return fmt.Errorf("Error occured while creating JSON: %s", err)
	}

	metadataFile := fmt.Sprintf("%s/%s", *output, metadataFileName)
	err = os.WriteFile(metadataFile, data, 0644)
	if err != nil {
		return fmt.Errorf("Error occured while creating file %s: %s", metadataFile, err)
	}

	/// ASSEMBLE FINAL OUTPUT FILE
	outputFile := fmt.Sprintf("%s.tar.gz", *output)
	err = compression.Tar(*output, outputFile)
	if err != nil {
		return fmt.Errorf("Error occured while creating final tar archive %s: %s", outputFile, err)
	}

	return nil
}

func extractDockerState(state *dockerState) error {
	state.Version = exportVersion

	images, err := docker.ImageList(nil)
	if err != nil {
		return err
	}

	for _, image := range images {
		inspect, err := docker.ImageInspect(image.ID)
		if err != nil {
			return err
		}

		state.Images = append(state.Images, inspect)
	}

	volumes, err := docker.VolumeList(nil)
	if err != nil {
		return err
	}

	for _, volume := range volumes {
		if docker.VolumeAnonymous(volume.Labels) {
			console.PrintWithColoredForeground(os.Stdout, console.WARNING, "Volume '%s' is anonymous and won't be exported", volume.Name)
			continue // Don't export anonymous volumes
		}

		inspect, err := docker.VolumeInspect(volume.Name)
		if err != nil {
			return err
		}

		state.Volumes = append(state.Volumes, inspect)
	}

	networks, err := docker.NetworkList(nil)
	if err != nil {
		return err
	}

	for _, network := range networks {
		if docker.NetworkNameReserved(network.Name) {
			// Don't print a warning here because these networks will always be present, just dont add them to the export
			continue // Don't export networks "bridge", "host", and "none" as the will always exist on the other device, because they are built-in
		}

		inspect, err := docker.NetworkInspect(network.ID)
		if err != nil {
			return err
		}

		state.Networks = append(state.Networks, inspect)
	}

	containers, err := docker.ContainerList(nil)
	if err != nil {
		return err
	}

	for _, container := range containers {
		inspect, err := docker.ContainerInspect(container.ID)
		if err != nil {
			return err
		}

		state.Containers = append(state.Containers, inspect)
	}

	return nil
}

func saveVolumes(outputDir string) error {
	volumes, err := docker.VolumeList(nil)
	if err != nil {
		return err
	}

	for _, volume := range volumes {
		if docker.VolumeAnonymous(volume.Labels) {
			// TODO: give user options:
			//     A) Keep volume anonymous and keeep data
			//     B) Keep volume anonymous and drop data
			//     C) Convert to named volume and keep data
			//     D) Convert to named volume and drop data
			//     E) Abort
			continue // Don't export anonymous volumes
		}

		err = docker.VolumeSave(volume.Name, volume.Name, outputDir)
		if err != nil {
			return err
		}
	}

	return nil
}
