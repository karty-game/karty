package screenshot

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
	"github.com/urfave/cli/v3"
)

const maxViews = 64
const maxBatchBytes = 64 * 1024

type viewRequest struct {
	pose   camera
	output string
}

type authoredView struct {
	Position []float64 `toml:"position"`
	Yaw      *float64  `toml:"yaw"`
	Pitch    *float64  `toml:"pitch"`
	FOV      *float64  `toml:"fov"`
	Output   string    `toml:"output"`
}

type nativeView struct {
	Output string       `toml:"output"`
	Camera nativeCamera `toml:"camera"`
}

// Field names match the versioned host process interface, independently of SDK bindings.
type nativeCamera struct {
	X      float64 `toml:"x"`
	Y      float64 `toml:"y"`
	Z      float64 `toml:"z"`
	Yaw    float64 `toml:"yaw"`
	Pitch  float64 `toml:"pitch"`
	FOV    float64 `toml:"fov"`
	Near   float64 `toml:"near"`
	Far    float64 `toml:"far"`
	Width  int     `toml:"width"`
	Height int     `toml:"height"`
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

	if err := validateAuthoredViewTables(data); err != nil {
		return nil, err
	}

	decoder := toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields()

	var batch struct {
		Views []authoredView `toml:"views"`
	}
	if err := decoder.Decode(&batch); err != nil {
		return nil, err
	}

	return batch.Views, nil
}

func validateAuthoredViewTables(contents []byte) error {
	parser := unstable.Parser{}
	parser.Reset(contents)

	count := 0

	for parser.NextExpression() {
		expression := parser.Expression()
		if expression.Kind != unstable.Table && expression.Kind != unstable.ArrayTable {
			continue
		}

		key := expression.Key()
		if !key.Next() || !bytes.Equal(key.Node().Data, []byte("views")) || key.Next() {
			continue
		}

		if expression.Kind != unstable.ArrayTable || count == maxViews {
			return fmt.Errorf("batch requires 1–64 [[views]] entries: %w", os.ErrInvalid)
		}

		count++
	}

	if err := parser.Error(); err != nil {
		return err
	}

	if count == 0 {
		return fmt.Errorf("batch requires 1–64 [[views]] entries: %w", os.ErrInvalid)
	}

	return nil
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

	file, err := os.CreateTemp("", "karty-screenshot-batch-*.toml")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())

	err = toml.NewEncoder(file).Encode(struct {
		Views []nativeView `toml:"views"`
	}{Views: views})
	closeErr := file.Close()

	if err != nil {
		return err
	}

	if closeErr != nil {
		return closeErr
	}

	command := exec.CommandContext(ctx, host, "--screenshot-protocol", "2", "--screenshot-level", level,
		"--screenshot-features", strings.Join(features, ","), "--screenshot-batch", file.Name())

	command.Stdout, command.Stderr = diagnostics, diagnostics
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("native screenshot batch: %w", ctx.Err())
		}

		return fmt.Errorf("native screenshot batch failed; use a host supporting screenshot-v2 TOML batches: %w", err)
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
