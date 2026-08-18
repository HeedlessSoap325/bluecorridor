package docker

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/heedlesssoap325/bluecorridor/internal/console"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/client"
)

type TransferMethod int

const (
	MethodPull TransferMethod = iota
	MethodSaveLoad
)

// List all Images
//
// The image manifest and Identity are not returned
func ImageList(filters client.Filters) ([]image.Summary, error) {
	images, err := dockerClient.ImageList(ctx, client.ImageListOptions{
		All:       true,
		Filters:   filters,
		Manifests: false,
		Identity:  false,
	})

	if err != nil {
		return nil, fmt.Errorf("Error occured while listing docker images: %s", err)
	}

	return images.Items, nil
}

// Inspect an Image
//
// The Manifest is not returned
func ImageInspect(imageID string) (client.ImageInspectResult, error) {
	inspect, err := dockerClient.ImageInspect(ctx, imageID, client.ImageInspectWithManifests(false))

	if err != nil {
		return client.ImageInspectResult{}, fmt.Errorf("Error occured while inspecting docker image: %s", err)
	}

	return inspect, nil
}

// Pull a given Image
//
// If the prettyprint boolean is set to true, the progress will be printed to the console
// When the function returns, the console will be in the same state as before, and the progress text will have been cleared
//
// Otherwise, the function will return once the pull was successfull
func ImagePull(refStr string, prettyprint bool) error {
	res, err := dockerClient.ImagePull(ctx, refStr, client.ImagePullOptions{})

	if err != nil {
		return fmt.Errorf("Error occured while pulling docker image: %s\n", err)
	}

	if prettyprint {
		imagePullPrettyprint(res)
	} else {
		err := res.Wait(ctx)
		if err != nil {
			return fmt.Errorf("Error occured while pulling image %s: %s", refStr, err)
		}
	}

	return nil
}

func imagePullPrettyprint(pullResponse client.ImagePullResponse) {
	msgs := pullResponse.JSONMessages(ctx)
	windowHeight := 5
	window := make([]string, 0, windowHeight)
	printed := 0
	previousMsg := ""

	// This is a blocking loop that will continue until the pull is completed
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

			// Update the previous Message, if they indicate the progress for the same thing
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
			window = window[1:] // Delete first Message
		}

		if printed > 0 {
			console.MoveCursorUpNLines(printed)
		}

		// Print the actual MEssage
		for _, line := range window {
			console.ClearCurrentLine()
			console.Printlnf(console.BACKGROUND, "    %s", line)
		}

		printed = len(window)
	}

	if printed > 0 {
		console.ClearNLinesAndPositionCursorAtStart(printed)
	}
}

// Split image Refs like  "postgres:latest", "myrepo/app:v1", "ghcr.io/org/app:latest"
//
// returns (registry, repository, tag)
func parseImageRef(ref string) (registry, repo, tag string) {
	// Split off tag
	tagParts := strings.SplitN(ref, ":", 2)

	namePart := tagParts[0]

	// If no tag is provided, use latest tag
	if len(tagParts) == 2 {
		tag = tagParts[1]
	} else {
		tag = "latest"
	}

	// Check if first component looks like a registry host
	// (contains a dot, colon, or is "localhost")
	parts := strings.SplitN(namePart, "/", 2)
	if len(parts) == 2 && (strings.ContainsAny(parts[0], ".:") || parts[0] == "localhost") {
		registry = parts[0]
		repo = parts[1]
	} else {
		// Docker Hub
		registry = "registry-1.docker.io"
		repo = namePart

		// Docker Hub official images need "library/" prefix
		if !strings.Contains(repo, "/") {
			repo = "library/" + repo
		}
	}

	return
}

func getDockerHubToken(repo string) (string, error) {
	// Get a temporary token to pull images from the given repository.
	url := fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s:pull", repo)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Token string `json:"token"`
	}

	// Decode the raw response into the result structure
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Token, nil
}

func checkDockerHubManifest(repo, tag, token string) (bool, error) {
	// Get the Manifest of the given Image using the docker access token
	url := fmt.Sprintf("https://registry-1.docker.io/v2/%s/manifests/%s", repo, tag)
	req, _ := http.NewRequest("HEAD", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	// If the request to the manifest returns status 200, it is pullable with no further authentication
	// If the status is not 200 however, the image is not pullable
	return resp.StatusCode == http.StatusOK, nil
}

// Determin if a image can be pulled on the target or has to be saved and restored
//
// returns the TransferMethod, as well as a pullable tag, if it exists
func DetermineTransferMethod(repoTags []string, repoDigests []string) (TransferMethod, string) {
	// No usable tags -> local build -> not pullable
	usableTags := []string{}
	for _, tag := range repoTags {
		if tag != "<none>:<none>" {
			usableTags = append(usableTags, tag)
		}
	}

	// If the image has no tags (unnamed) it can't be pullable
	if len(usableTags) == 0 {
		return MethodSaveLoad, ""
	}

	// No digests -> never pushed to any registry -> not pullable
	if len(repoDigests) == 0 {
		return MethodSaveLoad, ""
	}

	// Check if image is pullable via Docker Hub
	for _, tag := range usableTags {
		registry, repo, imageTag := parseImageRef(tag)

		if registry != "registry-1.docker.io" {
			// Private registry, ghcr.io etc. -> possibly unauthorized on target -> save to be sure
			return MethodSaveLoad, ""
		}

		token, err := getDockerHubToken(repo)
		if err != nil {
			// When Docker is down or similar, save the image to be save
			return MethodSaveLoad, ""
		}

		available, err := checkDockerHubManifest(repo, imageTag, token)
		if err != nil {
			return MethodSaveLoad, ""
		}

		if available {
			return MethodPull, tag // found at least one pullable tag, done
		}
	}

	// All tags are exhausted and none of them returned a pullable image option -> save and load
	return MethodSaveLoad, ""
}

func ImageSize(imageID string) (int64, error) {
	res, err := dockerClient.ImageHistory(ctx, imageID)
	if err != nil {
		return 0, fmt.Errorf("Erroroccured while loading image history: %s", err)
	}

	var totalSize int64 = 0
	for _, item := range res.Items {
		if item.Size > 0 {
			totalSize += item.Size
		}
	}

	return totalSize, nil
}

func ImageSave(imageID string, imageRefs []string, outDir string) error {
	res, err := dockerClient.ImageSave(ctx, imageRefs)
	if err != nil {
		return fmt.Errorf("Error occured while saving image '%s': %s", imageID, err) // imageRef = "myrepo/myimage:tag"
	}

	defer res.Close()

	fileName := fmt.Sprintf("%s.tar", strings.ReplaceAll(imageID, ":", "_")) // Replace : with _ for Windows
	out, err := os.Create(filepath.Join(outDir, fileName))
	if err != nil {
		return fmt.Errorf("Error occured while creating File '%s.tar' to %s: %s", imageID, outDir, err)
	}

	defer out.Close()

	_, err = io.Copy(out, res)
	if err != nil {
		return fmt.Errorf("Error occured while copying tar archive: %s", err)
	}

	return nil
}

func ImageLoad(imageID string, inDir string) error {
	fileName := fmt.Sprintf("%s.tar", strings.ReplaceAll(imageID, ":", "_")) // Replace : with _ for Windows
	file, err := os.OpenFile(filepath.Join(inDir, fileName), os.O_RDONLY, 438)
	if err != nil {
		return fmt.Errorf("Error occured while reading file '%s.tar' from %s: %s", imageID, inDir, err)
	}

	defer file.Close()

	res, err := dockerClient.ImageLoad(ctx, file, client.ImageLoadWithQuiet(true))
	if err != nil {
		return fmt.Errorf("Error occured while loading image '%s': %s", imageID, err)
	}

	defer res.Close()

	return nil
}
