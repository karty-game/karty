//nolint:gosec // Shell fixtures are owner-only files in the test's private temporary directory.
package screenshot

import (
	"context"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"
)

func TestBatchOptions(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "views.json")
	for _, input := range []string{
		`[]`, `null`, `[{"position":[1,2]}]`, `[{"position":[1,2,3,4]}]`,
		`[{"pitch":90}]`, `[{"unknown":1}]`, `[] []`,
		`[{"output":"same.png"},{"output":"./same.png"}]`,
		"[" + strings.Repeat("{},", maxViews) + "{}]",
		strings.Repeat(" ", maxBatchBytes+1),
	} {
		if err := os.WriteFile(path, []byte(input), 0600); err != nil {
			t.Fatal(err)
		}

		command := Command()

		command.Action = func(_ context.Context, cmd *cli.Command) error {
			_, err := viewOptions(cmd, camera{width: 640, height: 360, fov: 68})

			return err
		}
		if err := command.Run(t.Context(), []string{"screenshot", "test-level", "--batch", path}); err == nil {
			t.Fatal("bad batch accepted", input)
		}
	}

	if err := os.WriteFile(path, []byte(`[{"position":[-11,18,2.9],"yaw":49},{"pitch":3,"output":"reactor.png"}]`), 0600); err != nil {
		t.Fatal(err)
	}

	command := Command()

	command.Action = func(_ context.Context, cmd *cli.Command) error {
		views, err := viewOptions(cmd, camera{position: [3]float64{4, 5, 1.7}, yaw: 24, width: 640, height: 360, fov: 68})
		if err != nil {
			return err
		}

		if len(views) != 2 || views[0].pose.position[0] != -11 || views[0].pose.yaw != 49 || views[1].pose.yaw != 24 ||
			views[1].pose.pitch != 3 {
			t.Fatal("batch overrides or defaults lost", views)
		}

		return nil
	}
	if err := command.Run(t.Context(), []string{"screenshot", "test-level", "--batch", path}); err != nil {
		t.Fatal(err)
	}
}

func TestBatchLaunchesOneHostAndValidatesAllOutputs(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("shell transport fixture")
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

	host := filepath.Join(root, "host")
	calls := filepath.Join(root, "calls")

	script := "#!/bin/sh\nprintf x >> '" + calls + "'\nwhile [ $# -gt 0 ]; do\nif [ \"$1\" = --screenshot-batch ]; then\nsed -n 's/.*\"output\": \"\\([^\"]*\\)\".*/\\1/p' \"$2\" | while IFS= read -r output; do cp '" + fixture + "' \"$output\"; done\nfi\nshift\ndone\n"
	if err := os.WriteFile(host, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}

	pose := camera{width: 8, height: 6, fov: 68, near: .05, far: 120}

	requests := []viewRequest{
		{pose: pose, output: filepath.Join(root, "first.png")},
		{pose: pose, output: filepath.Join(root, "second.png")},
	}
	if err := captureBatch(t.Context(), host, "level.kld", nil, requests, io.Discard); err != nil {
		t.Fatal(err)
	}

	count, err := os.ReadFile(calls)
	if err != nil || string(count) != "x" {
		t.Fatal("batch started more than one host", err, string(count))
	}

	before, err := os.ReadFile(requests[0].output)
	if err != nil {
		t.Fatal(err)
	}
	// A host that saves only one view must never publish a partially valid batch.
	script = strings.Replace(script, "| while IFS=", "| head -n 1 | while IFS=", 1)
	if err := os.WriteFile(host, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}

	if err := captureBatch(t.Context(), host, "level.kld", nil, requests, io.Discard); err == nil {
		t.Fatal("incomplete batch accepted")
	}

	after, err := os.ReadFile(requests[0].output)
	if err != nil || string(after) != string(before) {
		t.Fatal("incomplete batch replaced an old image")
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".capture-") {
			t.Fatal("batch left staging files")
		}
	}
}
