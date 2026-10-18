package levelbuild

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIncrementalBakeReusesOnlyValidatedCurrentInputs(t *testing.T) {
	t.Parallel()
	root := lightingLevelFixture(t, "version: 4\n"+lightingRoomYAML+directLightmapYAML)
	enableFixtureLightmap(t, root, automaticBakeSettings)

	options := BakeOptions{Workers: 1, Incremental: true}
	run := func(reused bool) BakeReport {
		t.Helper()

		reports, err := BakeAll(t.Context(), root, options, nil)
		if err != nil || len(reports) != 1 {
			t.Fatalf("incremental bake: %v, %v", reports, err)
		}

		r := reports[0]
		if r.Reused != reused || (r.Stats.Rays == 0) != reused || (r.OutputBytes == 0) != reused {
			t.Fatalf("reuse=%v: report %+v", reused, r)
		}

		return r
	}
	run(false)
	// A fresh checkout can restore generated bakes independently of its path,
	// file timestamps or Git history, as the CI cache action does.
	restored := lightingLevelFixture(t, "version: 4\n"+lightingRoomYAML+directLightmapYAML)
	enableFixtureLightmap(t, restored, automaticBakeSettings)

	if err := os.CopyFS(filepath.Join(restored, ".karty", "bakes"), os.DirFS(filepath.Join(root, ".karty", "bakes"))); err != nil {
		t.Fatal(err)
	}

	root = restored
	first := run(true)

	old := time.Unix(12345, 0)
	for _, path := range []string{first.Manifest, first.Image} {
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}

	options.Workers = 2 // Worker count does not change pixels.

	run(true)

	for _, path := range []string{first.Manifest, first.Image} {
		info, err := os.Stat(path)
		if err != nil || !info.ModTime().Equal(old) {
			t.Fatalf("cache hit rewrote %s: %v", path, err)
		}
	}

	// Overrides must replace authored defaults when deciding cache validity.
	samples, bounces, denoise := 2, 1, "low"
	options.Samples = &samples

	run(false)
	run(true)

	options.Bounces = &bounces

	run(false)

	options.Denoise = &denoise

	run(false)
	run(true)

	// An explicit nonincremental bake still regenerates matching inputs.
	options.Incremental = false

	run(false)

	options.Incremental = true

	texture := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	texture.SetNRGBA(0, 0, color.NRGBA{G: 180, A: 255})

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, texture); err != nil {
		t.Fatal(err)
	}

	writePrebakeFixture(t, filepath.Join(root, "levels", "lighting", "wall.png"), encoded.Bytes())
	run(false)

	worldPath := filepath.Join(root, "levels", "lighting", "world.yaml")

	source, err := os.ReadFile(worldPath)
	if err != nil {
		t.Fatal(err)
	}

	changed := strings.Replace(string(source), "color: {x: 1, y: 0, z: 0}", "color: {x: 0.8, y: 0, z: 0}", 1)
	if changed == string(source) {
		t.Fatal("fixture did not change a selected source")
	}

	writePrebakeFixture(t, worldPath, []byte(changed))

	current := run(false)
	writePrebakeFixture(t, current.Image, []byte("corrupt QOI"))
	run(false)
	run(true)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := BakeAll(ctx, root, options, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled incremental bake: %v", err)
	}
}
