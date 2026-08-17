package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"maps"
	"os"
	"strings"

	"github.com/heedlesssoap325/bluecorridor/internal/compression"
	"github.com/heedlesssoap325/bluecorridor/internal/console"
	"github.com/heedlesssoap325/bluecorridor/internal/docker"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client"
)

func handleImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	file := fs.String("file", "docker-export.tar.gz", "The file from which to import docker")
	help := fs.Bool("help", false, "Print this message")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s import [options]\n\n", os.Args[0])
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

	tmpDir, metadataFile, volumeDir, imageDir, err := getTempPaths()
	if err != nil {
		return err
	}

	defer os.RemoveAll(tmpDir)

	if err := compression.Untar(*file, tmpDir); err != nil {
		return fmt.Errorf("Error occured while untaring file: %s", err)
	}

	var state dockerState
	if err := readAndValidateMetadataFile(metadataFile, &state); err != nil {
		return err
	}

	// Images have to be present when creating containers so this has to be called before importing the docker-state
	if err := loadImages(state.Images, imageDir); err != nil {
		return err
	}

	if err := importDockerState(&state); err != nil {
		return err
	}

	if err := restoreVolumes(state.Volumes, volumeDir); err != nil {
		return err
	}

	return nil
}

func readAndValidateMetadataFile(metadataFile string, state *dockerState) error {
	raw, err := os.ReadFile(metadataFile)
	if err != nil {
		return fmt.Errorf("Error occured while reading imageMetadata file %s: %s", metadataFile, err)
	}

	if err := json.Unmarshal(raw, &state); err != nil {
		return fmt.Errorf("Error occured while parsing JSON: %s", err)
	}

	if state.Version != exportVersion {
		return fmt.Errorf("Incompatible imageMetadata versions. Export: %s, this program: %s\nThe file provided is either to new or to old for this program", state.Version, exportVersion)
	}

	return nil
}

func importDockerState(state *dockerState) error {
	// During import, state is progressively rewritten to contain identifiers assigned by the destination Docker daemon.
	if err := importImages(state); err != nil {
		return err
	}

	volumeMap, err := importVolumes(state)
	if err != nil {
		return err
	}

	networkNameToID, err := importNetworks(state)
	if err != nil {
		return err
	}

	if err := importContainers(state, volumeMap, networkNameToID); err != nil {
		return err
	}

	return nil
}

func importImages(state *dockerState) error {
	for _, imageMetadata := range state.Images {
		if imageMetadata.Method != docker.MethodPull {
			// This function is only concerned with pulling Images
			// Images that have to be loaded should have already been loaded at this point
			continue
		}

		console.Printlnf(console.INFO, "Pulling Image '%s'", imageMetadata.Name)

		err := docker.ImagePull(imageMetadata.RepoTag, true)
		if err != nil {
			return err
		}

		console.ClearNLinesAndPositionCursorAtStart(1) // Clear the "Pulling Image ..." line
		console.Printlnf(console.SUCCESS, "Successfully pulled image '%s'", imageMetadata.Name)
	}

	return nil
}

// importVolumes recreates named volumes and records mappings from exported volume names to their names on the target Docker host.
// Anonymous volumes are intentionally left for Docker to create when the containers are created.
func importVolumes(state *dockerState) (map[string]string, error) {
	volumeMap := make(map[string]string)
	for _, volumeInspect := range state.Volumes {
		originalName := volumeInspect.Volume.Name
		if isReference, reference := docker.VolumeReference(volumeInspect.Volume.Labels); isReference {
			originalName = reference
		}

		if docker.VolumeAnonymous(volumeInspect.Volume.Labels) {
			// When creating the container, the old mounts will be updated and each old mount name will be maped to the new mount name.
			// To create a anonymous volume, the name the old volume will be mapped to must be empty to force docker to create a anonymous volume.
			// In a later step, the names of the new anonymous volumes will be back-patched in the state
			volumeMap[originalName] = ""
			continue
		}

		console.Printlnf(console.INFO, "Re-Creating volume '%s'", volumeInspect.Volume.Name)

		var clusterVolumeSpec *volume.ClusterVolumeSpec
		if volumeInspect.Volume.ClusterVolume != nil {
			clusterVolumeSpec = &volumeInspect.Volume.ClusterVolume.Spec
		}

		volume, err := docker.VolumeCreate(client.VolumeCreateOptions{
			Name:              volumeInspect.Volume.Name,
			Driver:            volumeInspect.Volume.Driver,
			DriverOpts:        volumeInspect.Volume.Options,
			Labels:            cleanupVolumeLabels(volumeInspect.Volume.Labels),
			ClusterVolumeSpec: clusterVolumeSpec,
		})

		if err != nil {
			return nil, err
		}

		volumeMap[originalName] = volume.Name // Map the original name present in the exports to the new name present on the host

		console.ClearNLinesAndPositionCursorAtStart(1) // Clear "Creating volume ..." line
		console.Printlnf(console.SUCCESS, "Successfully created volume '%s'", volume.Name)
	}

	return volumeMap, nil
}

func cleanupVolumeLabels(labels map[string]string) map[string]string {
	volumeLabels := make(map[string]string, len(labels))
	maps.Copy(volumeLabels, labels)

	delete(volumeLabels, "dev.heedlesssoap.bluecorridor.volume.dataless")
	delete(volumeLabels, "dev.heedlesssoap.bluecorridor.volume.reference")

	return volumeLabels
}

func importNetworks(state *dockerState) (map[string]string, error) {
	networkNameToID := make(map[string]string)
	for _, networkInspect := range state.Networks {
		if docker.NetworkNameReserved(networkInspect.Network.Name) {
			console.Printlnf(console.WARNING, "Network '%s' is a built-in network. Can't use that name. Skipping", networkInspect.Network.Name)
			continue
		}

		console.Printlnf(console.INFO, "Creating network '%s'", networkInspect.Network.Name)

		id, err := docker.NetworkCreate(networkInspect.Network.Name, client.NetworkCreateOptions{
			Driver:     networkInspect.Network.Driver,
			Scope:      networkInspect.Network.Scope,
			EnableIPv4: &networkInspect.Network.EnableIPv4,
			EnableIPv6: &networkInspect.Network.EnableIPv6,
			IPAM:       &networkInspect.Network.IPAM,
			Internal:   networkInspect.Network.Internal,
			Attachable: networkInspect.Network.Attachable,
			Ingress:    networkInspect.Network.Ingress,
			ConfigOnly: networkInspect.Network.ConfigOnly,
			ConfigFrom: networkInspect.Network.ConfigFrom.Network,
			Options:    networkInspect.Network.Options,
			Labels:     networkInspect.Network.Labels,
		})

		if err != nil {
			return nil, err
		}

		networkNameToID[networkInspect.Network.Name] = id

		console.ClearNLinesAndPositionCursorAtStart(1) // Clear "Creating network ..." line
		console.Printlnf(console.SUCCESS, "Successfully created network '%s': %s", networkInspect.Network.Name, id)
	}

	return networkNameToID, nil
}

func importContainers(state *dockerState, volumeMap map[string]string, networkNameToID map[string]string) error {
	for _, containerInspect := range state.Containers {
		containerName := strings.TrimPrefix(containerInspect.Container.Name, "/")

		// This updates all container mounts.
		// This has to be done because the export allows te user to potentially alter anonymous volumes and convert them to named volumes.
		// Simply using the export would cause the container to rereate a new anonymous volume instead of using the newly created named volume.
		anonymousMounts, err := updateContainerMounts(&containerInspect, volumeMap)
		if err != nil {
			return err
		}

		id, err := docker.ContainerCreate(client.ContainerCreateOptions{
			Config:           containerInspect.Container.Config,
			HostConfig:       containerInspect.Container.HostConfig,
			NetworkingConfig: nil, // Connect to networks later
			Platform:         nil,
			Name:             containerName,
		})

		if err != nil {
			return err
		}

		console.Printlnf(console.SUCCESS, "Successfully created container '%s': %s", containerName, id)

		newContainerInspect, err := docker.ContainerInspect(id)
		if err != nil {
			return err
		}

		// This searches for all newly created anonymous volumes (identified by their destination)
		// and then replaces the old name from the export with the name of the newly created truly anonymous volume
		// This is important, because it is possible that the user chose to persist the data in a anonymous volume.
		// To be able to do this, the restoreVolume function needs the name of the new anonymous volume.
		patchAnonymousVolumes(state, anonymousMounts, newContainerInspect.Container.Mounts)

		// If the client or daemon version were below 1.44, passing multiple networks for container creation would result in a error or wrong configuration of the container
		// This approach of iterating all networks the container was connected to and re-connecting them works with all versions, and is therefore more compatible, even though probably never actually necessary
		if err := connectContainerNetworks(newContainerInspect.Container.NetworkSettings.Networks, networkNameToID, id); err != nil {
			return err
		}
	}

	return nil
}

func updateContainerMounts(containerInspect *client.ContainerInspectResult, volumeMap map[string]string) (map[string]string, error) {
	anonymousMounts := make(map[string]string)
	newMounts := make([]mount.Mount, 0, len(containerInspect.Container.Mounts))

	// Docker assigns a new name when an anonymous volume is created.
	// Track the mount destination so we can replace the exported volume name with the newly assigned name after container creation.
	for _, mountPoint := range containerInspect.Container.Mounts {
		switch mountPoint.Type {
		case mount.TypeVolume:
			newName, ok := volumeMap[mountPoint.Name] // old name (or old anonymous ID) -> new volume name (or empty for anonymous volumes)
			if !ok {
				return map[string]string{}, fmt.Errorf("no mapping found for volume %q (dest %s)", mountPoint.Name, mountPoint.Destination)
			}
			if newName == "" {
				anonymousMounts[mountPoint.Name] = mountPoint.Destination
			}

			newMounts = append(newMounts, mount.Mount{
				Type:   mount.TypeVolume,
				Source: newName, // new volume name (docker will create a anonymous volume when this is empty)
				Target: mountPoint.Destination,
			})
		case mount.TypeBind:
			// keep as-is
			newMounts = append(newMounts, mount.Mount{
				Type:     mount.TypeBind,
				Source:   mountPoint.Source,
				Target:   mountPoint.Destination,
				ReadOnly: !mountPoint.RW,
			})
		default:
			return map[string]string{}, fmt.Errorf("UNSUPPORTED: volume of type %s can't be re-created (yet)", mountPoint.Type)
		}
	}

	containerInspect.Container.HostConfig.Mounts = newMounts
	containerInspect.Container.HostConfig.Binds = nil // clear legacy Binds so it doesn't fight with Mounts

	return anonymousMounts, nil
}

func patchAnonymousVolumes(state *dockerState, anonymousMounts map[string]string, containerMounts []container.MountPoint) {
	// Go through all the anonymous Mounts that docker should have created after the container creation
	for originalName, dest := range anonymousMounts {
		// Go through all new mounts of the container
		for _, mount := range containerMounts {
			// When the destinations match, the current mount must be the new anonymous volume that docker created
			if mount.Destination == dest {
				// Replace the name of the old anonymous volume in the state with the new one
				for idx := range state.Volumes {
					if state.Volumes[idx].Volume.Name == originalName {
						state.Volumes[idx].Volume.Name = mount.Name
					}
				}
			}
		}
	}
}

func connectContainerNetworks(networks map[string]*network.EndpointSettings, networkNameToID map[string]string, containerID string) error {
	for networkName, endpointSettings := range networks {
		newNetworkID, ok := networkNameToID[networkName]
		if !ok {
			// e.g. it was a reserved/skipped network like "bridge"/"host"/"none"
			newNetworkID = networkName // fall back to connecting by name
		}

		// EndpointSettings comes from the exported Docker state and contains IDs belonging to the source daemon.
		// Those IDs are invalid on the destination daemon, so clear them before reconnecting.
		// These fields will be resolved by the docker daemon when the container starts up for the first time
		endpointSettings.NetworkID = ""
		endpointSettings.EndpointID = ""

		err := docker.NetworkConnect(newNetworkID, client.NetworkConnectOptions{
			Container:      containerID,
			EndpointConfig: endpointSettings,
		})

		if err != nil {
			return err
		}
	}

	return nil
}

func restoreVolumes(volumeInspects []client.VolumeInspectResult, inDir string) error {
	for _, volume := range volumeInspects {
		if docker.VolumeDataless(volume.Volume.Labels) {
			continue // No data to import so just skip it
		}

		volumeName := volume.Volume.Name
		saveName := volume.Volume.Name

		if isReference, reference := docker.VolumeReference(volume.Volume.Labels); isReference {
			saveName = reference
		}

		console.Printlnf(console.INFO, "Restoring contents of volume '%s'", saveName)

		err := docker.VolumeRestore(volumeName, saveName, inDir)
		if err != nil {
			return err
		}

		console.ClearNLinesAndPositionCursorAtStart(1) // Clear "Restoring contents of volume ..." line
		console.Printlnf(console.SUCCESS, "Successfully restored contents of volume '%s'", saveName)
	}

	return nil
}

func loadImages(images []imageMetadata, inDir string) error {
	for _, imageMetadata := range images {
		if imageMetadata.Method != docker.MethodSaveLoad {
			continue // This function is only concerned with loading Images
		}

		console.Printlnf(console.INFO, "Loading non-pullable image '%s'", imageMetadata.Name)

		if err := docker.ImageLoad(imageMetadata.ID, inDir); err != nil {
			return err
		}

		console.ClearNLinesAndPositionCursorAtStart(1) // Clear "Loading non-pullable image ..." line
		console.Printlnf(console.SUCCESS, "Successfully loaded non-pullable image '%s'", imageMetadata.Name)
	}

	return nil
}
