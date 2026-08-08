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
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	output := fs.String("output", "docker-export", "The path in which to place the export file (noe extension required)")
	help := fs.Bool("help", false, "Print this message")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s export [options]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Options:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("error occured while parsing flags: %s", err)
	}

	if *help {
		fs.Usage()
	}

	/// REQUEST TEMPORARY DIRECTORY
	tmpDir, metadataFile, volumeDir, err := getTempPaths()
	if err != nil {
		return err
	}

	defer os.RemoveAll(tmpDir) // CLEANUP

	outputFile := fmt.Sprintf("%s.tar.gz", *output)

	/// CREATE METADATA FILE
	var state dockerState
	if err := extractDockerState(&state); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "    ")
	if err != nil {
		return fmt.Errorf("Error occured while creating JSON: %s", err)
	}

	if err = os.WriteFile(metadataFile, data, 0644); err != nil {
		return fmt.Errorf("Error occured while creating file %s: %s", metadataFile, err)
	}

	/// SAVE VOLUMES
	if err = saveVolumes(state.Volumes, volumeDir); err != nil {
		return err
	}

	/// ASSEMBLE FINAL OUTPUT FILE
	if err = compression.Tar(tmpDir, outputFile); err != nil {
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
				inspect.Volume.Labels["dev.heedlesssoap.bluecorridor.volume.reference"] = inspect.Volume.Name
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
		if docker.VolumeDataless(volume.Volume.Labels) {
			continue // If a volume doesn't have data, it doesn't need to be exported
		}

		volumeName := volume.Volume.Name
		saveName := volume.Volume.Name

		if isReference, reference := docker.VolumeReference(volume.Volume.Labels); isReference {
			volumeName = reference
			saveName = reference
		}

		console.PrintWithColoredForeground(os.Stdout, console.INFO, "Saving contents of volume '%s'", volumeName)

		err := docker.VolumeSave(volumeName, saveName, outputDir)
		if err != nil {
			return err
		}

		console.MoveCursorUpNLines(1)
		console.ClearCurrentLine()
		console.PrintWithColoredForeground(os.Stdout, console.SUCCESS, "Successfully saved contents of volume '%s'", volumeName)
	}

	return nil
}
