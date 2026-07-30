package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/heedlesssoap325/bluecorridor/internal/compression"
	"github.com/heedlesssoap325/bluecorridor/internal/console"
	"github.com/heedlesssoap325/bluecorridor/internal/docker"
	"github.com/moby/moby/client"
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

	/// SAVE VOLUMES
	volumeDir := fmt.Sprintf("%s/%s", *output, volumeDirName)
	err = os.MkdirAll(volumeDir, 0755)
	if err != nil {
		return fmt.Errorf("Could not create volumes directory in output directory %s: %s", *output, err)
	}

	saveVolumes(state.Volumes, volumeDir)

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
		console.PrintWithColoredForeground(os.Stdout, console.SUCCESS, "Successfully saved metadata of image '%s'", image.RepoTags[0])
	}

	volumes, err := docker.VolumeList(nil)
	if err != nil {
		return err
	}

	for _, volume := range volumes {
		inspect, err := docker.VolumeInspect(volume.Name)
		if err != nil {
			return err
		}

		if docker.VolumeAnonymous(volume.Labels) {
			// TODO: give user options:
			//     A) Keep volume anonymous and keeep data
			//     B) Keep volume anonymous and drop data
			//     C) Convert to named volume and keep data
			//     D) Convert to named volume and drop data
			//     E) Abort

			console.PrintWithColoredForeground(os.Stdout, console.INFO, "Volume '%s' is anonymous, please choose how to handle this:", volume.Name)
			console.PrintWithColoredForeground(os.Stdout, console.INFO, "    A) Keep volume anonymous and drop data")
			console.PrintWithColoredForeground(os.Stdout, console.INFO, "    B) Keep volume anonymous and keep data")
			console.PrintWithColoredForeground(os.Stdout, console.INFO, "    C) Convert to named volume and drop data")
			console.PrintWithColoredForeground(os.Stdout, console.INFO, "    D) Convert to named volume and keep data")
			console.PrintWithColoredForeground(os.Stdout, console.INFO, "    E) Abort")

			input := console.Prompt("Choose an option (A/B/C/D/E): ", []string{"A", "B", "C", "D", "E"})

			console.ClearNLinesAndPositionCursorAtStart(7)

			switch input {
			case "A":
				inspect.Volume.Labels["dev.heedlesssoap.bluecorridor.volume.dataless"] = ""
				inspect.Volume.Labels["com.docker.volume.anonymous"] = ""
			case "B":
				inspect.Volume.Labels["dev.heedlesssoap.bluecorridor.volume.reference"] = inspect.Volume.Name
				inspect.Volume.Labels["com.docker.volume.anonymous"] = ""
			case "C":
				inspect.Volume.Labels["dev.heedlesssoap.bluecorridor.volume.dataless"] = ""
				delete(inspect.Volume.Labels, "com.docker.volume.anonymous")

				name := console.Prompt("New volume name: ", []string{})
				console.ClearNLinesAndPositionCursorAtStart(1)
				inspect.Volume.Name = name
			case "D":
				inspect.Volume.Labels["dev.heedlesssoap.bluecorridor.volume.reference"] = inspect.Volume.Name
				delete(inspect.Volume.Labels, "com.docker.volume.anonymous")

				name := console.Prompt("New volume name: ", []string{})
				console.ClearNLinesAndPositionCursorAtStart(1)
				inspect.Volume.Name = name
			case "E":
				return fmt.Errorf("User abort")
			}
		}

		state.Volumes = append(state.Volumes, inspect)
		console.PrintWithColoredForeground(os.Stdout, console.SUCCESS, "Successfully saved metadata of volume '%s'", inspect.Volume.Name)
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
		console.PrintWithColoredForeground(os.Stdout, console.SUCCESS, "Successfully saved metadata of network '%s'", network.Name)
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
		console.PrintWithColoredForeground(os.Stdout, console.SUCCESS, "Successfully saved metadata of container '%s'", inspect.Container.Name)
	}

	return nil
}

func saveVolumes(volumes []client.VolumeInspectResult, outputDir string) error {
	for _, volume := range volumes {
		// TODO: handle anonymous volumes

		console.PrintWithColoredForeground(os.Stdout, console.INFO, "Saving contents of volume '%s'", volume.Volume.Name)

		err := docker.VolumeSave(volume.Volume.Name, volume.Volume.Name, outputDir) // TODO: handle saving of anonymous volumes correct
		if err != nil {
			return err
		}

		console.MoveCursorUpNLines(1)
		console.ClearCurrentLine()
		console.PrintWithColoredForeground(os.Stdout, console.SUCCESS, "Successfully saved contents of volume '%s'", volume.Volume.Name)
	}

	return nil
}
