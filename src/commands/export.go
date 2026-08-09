package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/heedlesssoap325/bluecorridor/internal/compression"
	"github.com/heedlesssoap325/bluecorridor/internal/console"
	"github.com/heedlesssoap325/bluecorridor/internal/docker"
	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client"
)

func handleExport(args []string) error {
	/// HANDLE FLAGS
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	output := fs.String("output", "docker-export", "The path in which to place the export file (noe extension required)")
	quiet := fs.Bool("quiet", false, "Discard all output except for errors")
	anonymousVolumeOption := fs.String("anonymous_volume", "", "Specify the dafault option for any anonymous volume\nA) Keep volume anonymous and drop data\nB) Keep volume anonymous and keep data\nC) Convert to named volume and drop data\nD) Convert to named volume and keep data")
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
		return nil
	}

	console.Configure(console.Config{
		Quiet: *quiet,
	})
	defer console.Reset()

	/// REQUEST TEMPORARY DIRECTORY
	tmpDir, metadataFile, volumeDir, err := getTempPaths()
	if err != nil {
		return err
	}

	defer os.RemoveAll(tmpDir) // CLEANUP

	outputFile := fmt.Sprintf("%s.tar.gz", *output)

	/// CREATE METADATA FILE
	var state dockerState
	if err := extractDockerState(&state, *anonymousVolumeOption); err != nil {
		return err
	}

	if err := writeMetadataFile(state, metadataFile); err != nil {
		return err
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

func extractDockerState(state *dockerState, anonymousVolumeOption string) error {
	state.Version = exportVersion

	if err := saveImageMetadata(state); err != nil {
		return err
	}

	if err := saveVolumeMetadata(state, anonymousVolumeOption); err != nil {
		return err
	}

	if err := saveNetworkMetadata(state); err != nil {
		return err
	}

	if err := saveContainerMetadata(state); err != nil {
		return err
	}

	return nil
}

func saveImageMetadata(state *dockerState) error {
	images, err := docker.ImageList(nil)
	if err != nil {
		return err
	}

	for _, image := range images {
		imageName := imageName(image)
		console.Printlnf(console.INFO, "Saving metadata of image '%s'", imageName)

		inspect, err := docker.ImageInspect(image.ID)
		if err != nil {
			return err
		}

		state.Images = append(state.Images, inspect)

		console.ClearNLinesAndPositionCursorAtStart(1)
		console.Printlnf(console.SUCCESS, "Successfully saved metadata of image '%s'", imageName)
	}

	return nil
}

func saveVolumeMetadata(state *dockerState, anonymousVolumeOption string) error {
	volumes, err := docker.VolumeList(nil)
	if err != nil {
		return err
	}

	for _, volume := range volumes {
		console.Printlnf(console.INFO, "Saving metadata of volume '%s'", volume.Name)

		inspect, err := docker.VolumeInspect(volume.Name)
		if err != nil {
			return err
		}

		if docker.VolumeAnonymous(volume.Labels) {
			if err := handleAnonymousVolume(volume, &inspect, anonymousVolumeOption); err != nil {
				return err
			}
		}

		state.Volumes = append(state.Volumes, inspect)

		console.ClearNLinesAndPositionCursorAtStart(1)
		console.Printlnf(console.SUCCESS, "Successfully saved metadata of volume '%s'", inspect.Volume.Name)
	}

	return nil
}

func handleAnonymousVolume(vol volume.Volume, inspect *client.VolumeInspectResult, anonymousVolumeOption string) error {
	option := ""
	switch anonymousVolumeOption {
	case "A", "B", "C", "D":
		option = anonymousVolumeOption
	default:
		option = promptAnonymousVolumeOption(vol)
	}

	switch option {
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

		inspect.Volume.Name = promptAnonymousVolumeNewName(vol)
	case "D":
		inspect.Volume.Labels["dev.heedlesssoap.bluecorridor.volume.reference"] = inspect.Volume.Name
		delete(inspect.Volume.Labels, "com.docker.volume.anonymous")

		inspect.Volume.Name = promptAnonymousVolumeNewName(vol)
	case "E":
		return fmt.Errorf("User abort")
	}

	return nil
}

func promptAnonymousVolumeOption(vol volume.Volume) (option string) {
	console.Printlnf(console.INFO, "Volume '%s' is anonymous, please choose how to handle this:", vol.Name)
	console.Printlnf(console.INFO, "    A) Keep volume anonymous and drop data")
	console.Printlnf(console.INFO, "    B) Keep volume anonymous and keep data")
	console.Printlnf(console.INFO, "    C) Convert to named volume and drop data")
	console.Printlnf(console.INFO, "    D) Convert to named volume and keep data")
	console.Printlnf(console.INFO, "    E) Abort")

	defer console.ClearNLinesAndPositionCursorAtStart(7)

	prompt := fmt.Sprintf("Choose an option for volume '%s' (A/B/C/D/E): ", vol.Name)
	option = console.Prompt(prompt, []string{"A", "B", "C", "D", "E"})
	return
}

func promptAnonymousVolumeNewName(vol volume.Volume) (name string) {
	prompt := fmt.Sprintf("Choose a new name for volume '%s': ", vol.Name)
	name = console.Prompt(prompt, []string{})
	console.ClearNLinesAndPositionCursorAtStart(1)
	return
}

func saveNetworkMetadata(state *dockerState) error {
	networks, err := docker.NetworkList(nil)
	if err != nil {
		return err
	}

	for _, network := range networks {
		if docker.NetworkNameReserved(network.Name) {
			// Don't print a warning here because these networks will always be present, just dont add them to the export
			continue // Don't export networks "bridge", "host", and "none" as the will always exist on the other device, because they are built-in
		}

		console.Printlnf(console.INFO, "Saving metadata of network '%s'", network.Name)

		inspect, err := docker.NetworkInspect(network.ID)
		if err != nil {
			return err
		}

		state.Networks = append(state.Networks, inspect)

		console.ClearNLinesAndPositionCursorAtStart(1)
		console.Printlnf(console.SUCCESS, "Successfully saved metadata of network '%s'", network.Name)
	}

	return nil
}

func saveContainerMetadata(state *dockerState) error {
	containers, err := docker.ContainerList(nil)
	if err != nil {
		return err
	}

	for _, container := range containers {
		console.Printlnf(console.INFO, "Saving metadata of container '%s'", container.Names[0])

		inspect, err := docker.ContainerInspect(container.ID)
		if err != nil {
			return err
		}

		state.Containers = append(state.Containers, inspect)

		console.ClearNLinesAndPositionCursorAtStart(1)
		console.Printlnf(console.SUCCESS, "Successfully saved metadata of container '%s'", container.Names[0])
	}

	return nil
}

func writeMetadataFile(state dockerState, metadataFile string) error {
	data, err := json.MarshalIndent(state, "", "    ")
	if err != nil {
		return fmt.Errorf("Error occured while creating JSON: %s", err)
	}

	if err = os.WriteFile(metadataFile, data, 0644); err != nil {
		return fmt.Errorf("Error occured while creating file %s: %s", metadataFile, err)
	}

	return nil
}

func saveVolumes(volumes []client.VolumeInspectResult, outputDir string) error {
	for _, volume := range volumes {
		if docker.VolumeDataless(volume.Volume.Labels) {
			continue // If a volume doesn't have data, it doesn't need to be exported
		}

		volumeName, saveName := getVolumeAndSaveNames(volume.Volume)

		console.Printlnf(console.INFO, "Saving contents of volume '%s'", volumeName)

		err := docker.VolumeSave(volumeName, saveName, outputDir)
		if err != nil {
			return err
		}

		console.ClearNLinesAndPositionCursorAtStart(1)
		console.Printlnf(console.SUCCESS, "Successfully saved contents of volume '%s'", volumeName)
	}

	return nil
}

func getVolumeAndSaveNames(volume volume.Volume) (volumeName string, saveName string) {
	volumeName = volume.Name
	saveName = volume.Name

	if isReference, reference := docker.VolumeReference(volume.Labels); isReference {
		volumeName = reference
		saveName = reference
	}

	return
}
