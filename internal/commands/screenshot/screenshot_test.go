//nolint:gosec // The test creates owner-only executable shell fixtures in its private temporary directory.
package screenshot

import (
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/karty-game/karty/internal/project"
	"github.com/urfave/cli/v3"
)

func TestCameraFlags(t *testing.T) {
	t.Parallel()

	for _, position := range []string{"1,2", "NaN,0,1", "0,Inf,1", "1000001,0,0", "a,b,c"} {
		command := Command()

		command.Action = func(_ context.Context, command *cli.Command) error {
			_, err := cameraOptions(command, configFixture())

			return err
		}
		if err := command.Run(t.Context(), []string{"screenshot", "test-level", "--position", position}); err == nil {
			t.Fatal("invalid position accepted", position)
		}
	}

	command := Command()

	command.Action = func(_ context.Context, command *cli.Command) error {
		pose, err := cameraOptions(command, configFixture())
		if err != nil {
			return err
		}

		if pose.position != [3]float64{-11, 18, 2.9} || pose.yaw != 49 || pose.width != 640 || pose.height != 720 || pose.fov != 68 {
			t.Fatal("camera parameters were not retained", pose)
		}

		return nil
	}
	if err := command.Run(
		t.Context(),
		[]string{"screenshot", "test-level", "--position=-11,18,2.9", "--yaw", "49", "--width", "640"},
	); err != nil {
		t.Fatal(err)
	}
}

func configFixture() project.Config {
	var config project.Config

	config.Project.Resolution.Width, config.Project.Resolution.Height = 1280, 720
	config.Project.Camera.FOVYDegrees, config.Project.Camera.Near, config.Project.Camera.Far = 68, .05, 120

	return config
}

func TestCaptureArgumentsAndFailedOutputPreservation(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("shell fixture; native integration covers the host process separately")
	}

	root := t.TempDir()
	fixture := filepath.Join(root, "fixture.png")

	file, err := os.Create(fixture)
	if err != nil {
		t.Fatal(err)
	}

	if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, 8, 6))); err != nil {
		t.Fatal(err)
	}

	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	host := filepath.Join(root, "host with spaces")
	arguments := filepath.Join(root, "arguments")

	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > '" + arguments + "'\nwhile [ $# -gt 0 ]; do\nif [ \"$1\" = --screenshot-output ]; then cp '" + fixture + "' \"$2\"; fi\nshift\ndone\n"
	if err := os.WriteFile(host, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}

	output := filepath.Join(root, "photo.png")

	pose := camera{position: [3]float64{4, 5, 1.7}, yaw: 24, pitch: 3, fov: 68, near: .05, far: 120, width: 8, height: 6}
	if err := capture(t.Context(), host, "level with spaces.kld", output, []string{"world/sectors@1"}, pose, io.Discard); err != nil {
		t.Fatal(err)
	}

	args, err := os.ReadFile(arguments)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(args), "--camera-yaw\n24\n") ||
		!strings.Contains(string(args), "--screenshot-level\nlevel with spaces.kld\n") {
		t.Fatal("host arguments changed", string(args))
	}

	before, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(host, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}

	if err := capture(t.Context(), host, "level", output, nil, pose, io.Discard); err == nil {
		t.Fatal("stale screenshot accepted as fresh output")
	}

	after, err := os.ReadFile(output)
	if err != nil || string(before) != string(after) {
		t.Fatal("failed capture replaced the previous image")
	}

	if err := os.WriteFile(host, []byte("#!/bin/sh\nexec sleep 3\n"), 0700); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()

	if err := capture(ctx, host, "level", output, nil, pose, io.Discard); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("capture deadline was not preserved", err)
	}
}
