package assetpipeline_test

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/project"
	"github.com/karty-game/karty/internal/sdk"
)

func TestAnalyzeTexturesIsDeterministicAndProfileSensitive(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	writePNG(t, filepath.Join(directory, "player.png"), color.RGBA{R: 255, A: 255})

	declaration := project.Texture{Name: "sprites.player", Source: "player.png", Profile: "sprite"}
	manifest := sdkManifest(t)

	first, err := assetpipeline.AnalyzeTextures(directory, manifest, []project.Texture{declaration})
	if err != nil {
		t.Fatal(err)
	}

	second, err := assetpipeline.AnalyzeTextures(directory, manifest, []project.Texture{declaration})
	if err != nil {
		t.Fatal(err)
	}

	if first.Textures[0].CacheKey != second.Textures[0].CacheKey || first.Textures[0].Output != second.Textures[0].Output {
		t.Fatalf("identical inputs produced different identities: %+v != %+v", first.Textures[0], second.Textures[0])
	}

	if first.Summary.EstimatedDecodedBytes != 16 || first.Textures[0].Width != 2 || first.Textures[0].Height != 2 {
		t.Fatalf("texture measurements = %+v", first)
	}

	if !strings.Contains(first.Textures[0].Output, first.Textures[0].ContentSHA256) {
		t.Fatalf("output %q is not content-addressed", first.Textures[0].Output)
	}

	declaration.Profile = "interface"

	profiled, err := assetpipeline.AnalyzeTextures(directory, manifest, []project.Texture{declaration})
	if err != nil {
		t.Fatal(err)
	}

	if profiled.Textures[0].CacheKey == first.Textures[0].CacheKey {
		t.Fatal("texture profile did not affect the pipeline cache key")
	}

	if profiled.Textures[0].Output != first.Textures[0].Output {
		t.Fatal("unchanged output bytes should retain the same content-addressed path")
	}
}

func TestAnalyzeTexturesRejectsInvalidPNG(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "player.png"), []byte("not png"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := assetpipeline.AnalyzeTextures(directory, sdkManifest(t), []project.Texture{{
		Name: "sprites.player", Source: "player.png", Profile: "sprite",
	}})
	if err == nil || !strings.Contains(err.Error(), `decode texture "sprites.player" as PNG`) {
		t.Fatalf("AnalyzeTextures() error = %v", err)
	}
}

func TestAnalyzeTexturesRejectsUnknownSDKProfile(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	writePNG(t, filepath.Join(directory, "player.png"), color.RGBA{A: 255})

	_, err := assetpipeline.AnalyzeTextures(directory, sdkManifest(t), []project.Texture{{
		Name: "sprites.player", Source: "player.png", Profile: "lossy",
	}})
	if err == nil || !strings.Contains(err.Error(), "not defined by the selected SDK") {
		t.Fatalf("AnalyzeTextures() profile error = %v", err)
	}
}

func TestPopulateCacheReusesAndRepairsEntries(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	writePNG(t, filepath.Join(directory, "player.png"), color.RGBA{G: 255, A: 255})

	declarations := []project.Texture{{Name: "sprites.player", Source: "player.png", Profile: "sprite"}}

	first, err := assetpipeline.AnalyzeTextures(directory, sdkManifest(t), declarations)
	if err != nil {
		t.Fatal(err)
	}

	if err := assetpipeline.PopulateCache(directory, &first); err != nil {
		t.Fatal(err)
	}

	if first.Textures[0].CacheHit {
		t.Fatal("first cache population reported a hit")
	}

	cachePath := first.Textures[0].SourcePath

	second, err := assetpipeline.AnalyzeTextures(directory, sdkManifest(t), declarations)
	if err != nil {
		t.Fatal(err)
	}

	if err := assetpipeline.PopulateCache(directory, &second); err != nil {
		t.Fatal(err)
	}

	if !second.Textures[0].CacheHit || second.Textures[0].SourcePath != cachePath {
		t.Fatalf("warm cache texture = %+v", second.Textures[0])
	}

	if err := os.WriteFile(cachePath, []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}

	third, err := assetpipeline.AnalyzeTextures(directory, sdkManifest(t), declarations)
	if err != nil {
		t.Fatal(err)
	}

	if err := assetpipeline.PopulateCache(directory, &third); err != nil {
		t.Fatal(err)
	}

	if third.Textures[0].CacheHit {
		t.Fatal("corrupt cache entry reported a hit")
	}

	repaired, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatal(err)
	}

	source, err := os.ReadFile(filepath.Join(directory, "player.png"))
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(repaired, source) {
		t.Fatal("corrupt cache entry was not repaired from source")
	}
}

func TestPopulateCacheConcurrentWriters(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	writePNG(t, filepath.Join(directory, "player.png"), color.RGBA{B: 255, A: 255})

	declarations := []project.Texture{{Name: "sprites.player", Source: "player.png", Profile: "sprite"}}
	manifest := sdkManifest(t)

	const writers = 8

	errors := make(chan error, writers)

	var group sync.WaitGroup
	for range writers {
		group.Go(func() {
			report, err := assetpipeline.AnalyzeTextures(directory, manifest, declarations)
			if err == nil {
				err = assetpipeline.PopulateCache(directory, &report)
			}

			errors <- err
		})
	}

	group.Wait()
	close(errors)

	for err := range errors {
		if err != nil {
			t.Errorf("concurrent PopulateCache() error = %v", err)
		}
	}

	report, err := assetpipeline.AnalyzeTextures(directory, manifest, declarations)
	if err != nil {
		t.Fatal(err)
	}

	if err := assetpipeline.PopulateCache(directory, &report); err != nil {
		t.Fatal(err)
	}

	if !report.Textures[0].CacheHit {
		t.Fatal("concurrent cache writers did not leave a valid entry")
	}
}

func sdkManifest(t *testing.T) sdk.Manifest {
	t.Helper()

	manifest, err := sdk.Resolve("0.0.1")
	if err != nil {
		t.Fatal(err)
	}

	return manifest
}

func writePNG(t *testing.T, path string, fill color.Color) {
	t.Helper()

	var output bytes.Buffer

	canvas := image.NewRGBA(image.Rect(0, 0, 2, 2))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(fill), image.Point{}, draw.Src)

	if err := png.Encode(&output, canvas); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, output.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}
