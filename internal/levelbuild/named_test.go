package levelbuild_test

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty/internal/levelbuild"
	"github.com/karty-game/karty/internal/sdk"
)

func TestNamedWorldBuildSkipsUnrelatedAssets(t *testing.T) {
	t.Parallel()

	selected, err := sdk.Resolve("0.0.7")
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()

	directory := filepath.Join(root, "levels", "gallery")
	if err := os.MkdirAll(directory, 0750); err != nil {
		t.Fatal(err)
	}

	manifest := "[level]\nname = 'levels.gallery'\nkind = 'world'\n[world]\nsource = 'world.yaml'\n[[textures]]\nname = 'wall'\nsource = 'wall.png'\n"
	if err := os.WriteFile(filepath.Join(directory, "level.toml"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}

	file, err := os.Create(filepath.Join(directory, "wall.png"))
	if err != nil {
		t.Fatal(err)
	}

	pixels := image.NewRGBA(image.Rect(0, 0, 2, 2))
	pixels.SetRGBA(0, 0, color.RGBA{R: 120, G: 100, B: 80, A: 255})

	if err := png.Encode(file, pixels); err != nil {
		t.Fatal(err)
	}

	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	world := `version: 1
rooms:
  - id: gallery
    boundary:
      - {id: south, start: {x: 0, y: 0}, end: {x: 4, y: 0}, material: wall}
      - {id: east, start: {x: 4, y: 0}, end: {x: 4, y: 4}, material: wall}
      - {id: north, start: {x: 4, y: 4}, end: {x: 0, y: 4}, material: wall}
      - {id: west, start: {x: 0, y: 4}, end: {x: 0, y: 0}, material: wall}
    floor: {c: 0}
    ceiling: {c: 4}
    floor_material: wall
    ceiling_material: wall
`
	if err := os.WriteFile(filepath.Join(directory, "world.yaml"), []byte(world), 0600); err != nil {
		t.Fatal(err)
	}

	other := filepath.Join(root, "levels", "broken")
	if err := os.MkdirAll(other, 0750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(other, "level.toml"),
		[]byte("[level]\nname = 'levels.broken'\nkind = 'world'\n[world]\nsource = 'missing.yaml'\n"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"gallery", "levels.gallery"} {
		artifact, err := levelbuild.BuildNamedWithAssets(t.Context(), root, name, 4, "", selected)
		if err != nil || artifact.Name != "levels.gallery" {
			t.Fatal("selected world did not build", name, err)
		}
	}

	for _, name := range []string{"unknown", "../../gallery"} {
		if _, err := levelbuild.BuildNamedWithAssets(t.Context(), root, name, 4, "", selected); err == nil {
			t.Fatal("unknown or escaping name accepted", name)
		}
	}
}
