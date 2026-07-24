package docker

import (
	"fmt"
	"os"

	"github.com/heedlesssoap325/bluecorridor/internal/printing"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/client"
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
			printing.MoveCursorUpNLines(printed)
		}

		// Print the actual MEssage
		for _, line := range window {
			printing.ClearCurrentLine()
			printing.PrintWithColoredForeground(os.Stdout, printing.GRAY, "    %s", line)
		}

		printed = len(window)
	}

	if printed > 0 {
		printing.ClearNLinesAndPositionCursorAtStart(printed)
	}

	printing.MoveCursorUpNLines(1)
}
