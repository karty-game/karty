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
	"slices"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"
)

func TestCaptureBackendAdmission(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"", "gpu", "Software", "hardware --other"} {
		if _, err := CaptureBackend(value, true); !errors.Is(err, os.ErrInvalid) {
			t.Fatal("invalid backend accepted", value, err)
		}
	}

	if _, err := CaptureBackend(
		"software",
		false,
	); !errors.Is(err, os.ErrInvalid) ||
		!strings.Contains(err.Error(), "WebGPU native host bundle") {
		t.Fatal("unverified host selection was not rejected before capture preparation", err)
	}

	if _, err := CaptureBackend("hardware", false); !errors.Is(err, os.ErrInvalid) {
		t.Fatal("unverified hardware host admitted", err)
	}

	for _, mode := range []string{"hardware", "software"} {
		options, err := CaptureBackend(mode, true)
		if err != nil || !slices.Equal(options.Arguments(), []string{"--backend", mode}) {
			t.Fatal("WebGPU backend was not preserved", mode, err)
		}
	}
}

func TestScreenshotBackendFlag(t *testing.T) {
	t.Parallel()

	command := Command()
	command.Action = func(_ context.Context, command *cli.Command) error {
		if command.String("backend") != "software" {
			t.Fatal("software flag lost")
		}

		return nil
	}

	if err := command.Run(t.Context(), []string{"screenshot", "room", "--backend", "software"}); err != nil {
		t.Fatal(err)
	}

	command = Command()
	called := false

	command.Action = func(context.Context, *cli.Command) error {
		called = true

		return nil
	}

	if err := command.Run(t.Context(), []string{"screenshot", "room", "--backend", "invalid"}); err == nil || called {
		t.Fatal("invalid backend reached project/build execution", err)
	}
}

func TestScreenshotAndBatchForwardOnlyAdmittedBackend(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("POSIX argv recorder; backend admission is platform independent")
	}

	for _, test := range []struct {
		name, mode string
		webgpu     bool
	}{
		{name: "hardware", mode: "hardware", webgpu: true},
		{name: "software", mode: "software", webgpu: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			host, arguments := backendHostFixture(t)

			options, err := CaptureBackend(test.mode, test.webgpu)
			if err != nil {
				t.Fatal(err)
			}

			pose := camera{width: 8, height: 6, fov: 68, near: .05, far: 120}
			output := filepath.Join(t.TempDir(), "capture.png")
			level := "level $literal with spaces.kld"

			if err = captureWithOptions(t.Context(), host, level, output, nil, pose, io.Discard, options); err != nil {
				t.Fatal(err)
			}

			assertBackendArguments(t, arguments, test.mode, "1")

			if err = captureBatchWithOptions(
				t.Context(),
				host,
				level,
				nil,
				[]viewRequest{{pose: pose, output: output}},
				io.Discard,
				options,
			); err != nil {
				t.Fatal(err)
			}

			assertBackendArguments(t, arguments, test.mode, "2")
		})
	}
}

func backendHostFixture(t *testing.T) (string, string) {
	t.Helper()

	root := t.TempDir()
	fixture := filepath.Join(root, "fixture.png")

	file, err := os.Create(fixture)
	if err != nil {
		t.Fatal(err)
	}

	if err = png.Encode(file, image.NewRGBA(image.Rect(0, 0, 8, 6))); err != nil {
		t.Fatal(err)
	}

	if err = file.Close(); err != nil {
		t.Fatal(err)
	}

	host, arguments := filepath.Join(root, "fake host"), filepath.Join(root, "arguments")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > '" + arguments + "'\nwhile [ $# -gt 0 ]; do\nif [ \"$1\" = --screenshot-output ]; then cp '" + fixture + "' \"$2\"; fi\nif [ \"$1\" = --screenshot-batch ]; then\nsed -n \"s/^output = [\\\"']\\\\([^\\\"']*\\\\)[\\\"']$/\\1/p\" \"$2\" | while IFS= read -r output; do cp '" + fixture + "' \"$output\"; done\nfi\nshift\ndone\n"

	// #nosec G306 -- The private argv fixture must be executable; it does not run a renderer.
	if err = os.WriteFile(host, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}

	return host, arguments
}

func assertBackendArguments(t *testing.T, path, mode string, protocol string) {
	t.Helper()

	// #nosec G304 -- The path is the argv record in this test's private temporary directory.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	arguments := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")

	expected := []string{"--backend", mode, "--screenshot-protocol", protocol}

	if len(arguments) < len(expected) || !slices.Equal(arguments[:len(expected)], expected) {
		t.Fatal("backend/protocol arguments changed", arguments)
	}

	if !strings.Contains(string(data), "--screenshot-level\nlevel $literal with spaces.kld\n") {
		t.Fatal("level argument was expanded or split", arguments)
	}
}
