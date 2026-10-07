package screenshot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/urfave/cli/v3"
)

const maxViews = 64
const maxBatchBytes = 64 * 1024

type viewRequest struct {
	pose   camera
	output string
}

type authoredView struct {
	Position []float64 `json:"position"`
	Yaw      *float64  `json:"yaw"`
	Pitch    *float64  `json:"pitch"`
	FOV      *float64  `json:"fov"`
	Output   string    `json:"output"`
}

type nativeView struct {
	Output string       `json:"output"`
	Camera nativeCamera `json:"camera"`
}

// Field names match the versioned host process interface, independently of SDK bindings.
type nativeCamera struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Z      float64 `json:"z"`
	Yaw    float64 `json:"yaw"`
	Pitch  float64 `json:"pitch"`
	FOV    float64 `json:"fov"`
	Near   float64 `json:"near"`
	Far    float64 `json:"far"`
	Width  int     `json:"width"`
	Height int     `json:"height"`
}

func viewOptions(command *cli.Command, defaults camera) ([]viewRequest, error) {
	views := []authoredView{{Output: command.String("output")}}
	if path := command.String("batch"); path != "" {
		if command.IsSet("output") {
			return nil, fmt.Errorf("use per-view output paths in a batch: %w", os.ErrInvalid)
		}

		var err error

		views, err = readAuthoredViews(path)
		if err != nil {
			return nil, err
		}
	}

	return resolveViews(views, defaults, command.Args().First())
}

func readAuthoredViews(path string) ([]authoredView, error) {
	var views []authoredView

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxBatchBytes+1))
	if err != nil {
		return nil, err
	}

	if len(data) > maxBatchBytes {
		return nil, fmt.Errorf("batch file exceeds 64 KiB: %w", os.ErrInvalid)
	}

	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&views); err != nil {
		return nil, err
	}

	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("batch must contain one JSON array: %w", os.ErrInvalid)
	}

	return views, nil
}

func resolveViews(views []authoredView, defaults camera, level string) ([]viewRequest, error) {
	if len(views) == 0 || len(views) > maxViews {
		return nil, fmt.Errorf("batch must contain 1–64 views: %w", os.ErrInvalid)
	}

	name := filepath.Base(level)
	if name == "." || name == ".." {
		return nil, fmt.Errorf("invalid level name: %w", os.ErrInvalid)
	}

	stamp := time.Now().UTC().Format("20060102-150405.000000000")
	requests := make([]viewRequest, 0, len(views))

	outputs := make(map[string]bool, len(views))
	for index, view := range views {
		pose := defaults
		if view.Position != nil {
			if len(view.Position) != len(pose.position) {
				return nil, fmt.Errorf("view %d position must have three coordinates: %w", index+1, os.ErrInvalid)
			}

			copy(pose.position[:], view.Position)
		}

		if view.Yaw != nil {
			pose.yaw = *view.Yaw
		}

		if view.Pitch != nil {
			pose.pitch = *view.Pitch
		}

		if view.FOV != nil {
			pose.fov = *view.FOV
		}

		if err := pose.validate(); err != nil {
			return nil, fmt.Errorf("view %d: %w", index+1, err)
		}

		output := view.Output
		if output == "" {
			output = filepath.Join("dist", "screenshots", fmt.Sprintf("%s-%s-%03d.png", name, stamp, index+1))
		}

		if !strings.EqualFold(filepath.Ext(output), ".png") {
			return nil, fmt.Errorf("output must have a .png extension: %w", os.ErrInvalid)
		}

		output, err := filepath.Abs(output)
		if err != nil {
			return nil, err
		}

		if outputs[output] {
			return nil, fmt.Errorf("duplicate screenshot output %q: %w", output, os.ErrInvalid)
		}

		outputs[output] = true
		requests = append(requests, viewRequest{pose: pose, output: output})
	}

	return requests, nil
}

func captureBatch(ctx context.Context, host, level string, features []string, requests []viewRequest, diagnostics io.Writer) error {
	views := make([]nativeView, 0, len(requests))
	defer func() {
		for _, view := range views {
			_ = os.Remove(view.Output)
		}
	}()

	for _, request := range requests {
		if err := os.MkdirAll(filepath.Dir(request.output), 0750); err != nil {
			return err
		}

		file, err := os.CreateTemp(filepath.Dir(request.output), ".capture-*.png")
		if err != nil {
			return err
		}

		views = append(views, nativeView{Output: file.Name(), Camera: nativeCamera{
			X: request.pose.position[0], Y: request.pose.position[1], Z: request.pose.position[2],
			Yaw: request.pose.yaw, Pitch: request.pose.pitch, FOV: request.pose.fov, Near: request.pose.near, Far: request.pose.far,
			Width: request.pose.width, Height: request.pose.height,
		}})
		if err := file.Close(); err != nil {
			return err
		}
	}

	file, err := os.CreateTemp("", "karty-screenshot-batch-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(views)
	closeErr := file.Close()

	if err != nil {
		return err
	}

	if closeErr != nil {
		return closeErr
	}

	command := exec.CommandContext(ctx, host, "--screenshot-protocol", "1", "--screenshot-level", level,
		"--screenshot-features", strings.Join(features, ","), "--screenshot-batch", file.Name())

	command.Stdout, command.Stderr = diagnostics, diagnostics
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("native screenshot batch: %w", ctx.Err())
		}

		return fmt.Errorf("native screenshot batch failed; use a host supporting screenshot-v1 batches: %w", err)
	}

	for index, view := range views {
		if err := validatePNG(view.Output, requests[index].pose); err != nil {
			return fmt.Errorf("view %d: %w", index+1, err)
		}
	}

	for index, view := range views {
		if err := os.Rename(view.Output, requests[index].output); err != nil {
			return err
		}
	}

	return nil
}
