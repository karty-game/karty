//nolint:wsl_v5 // The build fixtures keep their setup and deterministic-output assertions together.
package levelbuild_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/karty-game/karty-sdk/format/level"
	sdkworld "github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty/internal/levelbuild"
)

func TestBuildAllCompilesWorldYAMLIntoCanonicalLevelEntry(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	directory := filepath.Join(root, "levels", "camera")
	if err := os.MkdirAll(directory, 0o750); err != nil {
		t.Fatal(err)
	}

	var texture bytes.Buffer
	if err := png.Encode(&texture, image.NewNRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"floor", "ceiling", "wall"} {
		imageBytes := append([]byte(nil), texture.Bytes()...)
		if name == "wall" {
			texture.Reset()
			colored := image.NewNRGBA(image.Rect(0, 0, 1, 1))
			colored.SetNRGBA(0, 0, color.NRGBA{R: 200, G: 100, B: 50, A: 255})
			if err := png.Encode(&texture, colored); err != nil {
				t.Fatal(err)
			}
			imageBytes = texture.Bytes()
		}
		if err := os.WriteFile(filepath.Join(directory, name+".png"), imageBytes, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	manifest := `[level]
name = "levels.camera"
kind = "world"
[world]
source = "world.yaml"
[[textures]]
name = "floor"
source = "floor.png"
[[textures]]
name = "ceiling"
source = "ceiling.png"
[[textures]]
name = "wall"
source = "wall.png"
`
	if err := os.WriteFile(filepath.Join(directory, "level.toml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	yaml := `version: 1
rooms:
  - id: hall
    boundary:
      - {id: south, start: {x: 0, y: 0}, end: {x: 4, y: 0}, material: wall}
      - {id: east, start: {x: 4, y: 0}, end: {x: 4, y: 4}, material: wall}
      - {id: north, start: {x: 4, y: 4}, end: {x: 0, y: 4}, material: wall}
      - {id: west, start: {x: 0, y: 4}, end: {x: 0, y: 0}, material: wall}
    floor: {a: 0.05, b: 0, c: 0}
    ceiling: {a: 0, b: -0.05, c: 4}
    floor_material: floor
    ceiling_material: ceiling
    contents:
      - {id: spawn, kind: spawn, position: {x: 2, y: 2, z: 1}}
`
	if err := os.WriteFile(filepath.Join(directory, "world.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}

	first, err := levelbuild.BuildAll(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := levelbuild.BuildAll(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || !slices.Equal(first[0].Features, []string{sdkworld.Feature}) ||
		first[0].ContentSHA256 != second[0].ContentSHA256 {
		t.Fatalf("world artifacts = %+v", first)
	}

	envelope, err := level.Decode(unwrapLevelModule(t, first[0].Bytes))
	if err != nil {
		t.Fatal(err)
	}
	encoded, _, found := envelope.Read(sdkworld.EntryName, 0, sdkworld.MaxEncodedSize)
	if !found {
		t.Fatal("compiled world entry is missing")
	}
	document, err := sdkworld.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Sectors) != 1 || len(document.Contents) != 1 || document.Contents[0].SourceID != "spawn" {
		t.Fatalf("compiled world = %+v", document)
	}
}

func TestWorldCameraSampleLevelCompiles(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "samples", "world-camera")
	first, err := levelbuild.BuildAll(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := levelbuild.BuildAll(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].Name != "levels.camera-showcase" ||
		!slices.Equal(first[0].Features, []string{sdkworld.Feature}) ||
		first[0].ContentSHA256 != second[0].ContentSHA256 {
		t.Fatalf("sample artifacts = %+v", first)
	}

	envelope, err := level.Decode(unwrapLevelModule(t, first[0].Bytes))
	if err != nil {
		t.Fatal(err)
	}
	encoded, _, found := envelope.Read(sdkworld.EntryName, 0, sdkworld.MaxEncodedSize)
	if !found {
		t.Fatal("sample compiled world entry is missing")
	}
	document, err := sdkworld.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if document.Version != sdkworld.Version || len(document.Sectors) != 4 || len(document.Contents) != 8 {
		t.Fatalf("sample world v%d = %d sectors, %d contents", document.Version, len(document.Sectors), len(document.Contents))
	}

	actors := make(map[string]sdkworld.Content)
	for _, content := range document.Contents {
		if content.Actor != nil {
			actors[content.ID] = content
		}
	}
	if len(actors) != 7 {
		t.Fatalf("sample authored actors = %d, want 7", len(actors))
	}
	for id, facing := range map[string]sdkworld.SpriteFacing{
		"hall/camera-facing":    sdkworld.SpriteCameraFacing,
		"hall/upright":          sdkworld.SpriteUpright,
		"hall/fixed-sign":       sdkworld.SpriteFixed,
		"hall/slope-decal":      sdkworld.SpriteFixed,
		"gallery-a/room/marker": sdkworld.SpriteCross,
		"gallery-b/room/marker": sdkworld.SpriteCross,
	} {
		actor, ok := actors[id]
		if !ok || actor.Actor.Sprite == nil || actor.Actor.Sprite.Facing != facing || actor.Actor.Sprite.AssetID != 1 {
			t.Fatalf("sample actor %q = %+v, exists %v", id, actor, ok)
		}
	}
	decal := actors["hall/slope-decal"].Actor
	wantYaw := math.Atan2(-.03, .04)
	wantPitch := math.Atan2(1, math.Hypot(.03, .04))
	if math.Abs(decal.Yaw-wantYaw) > 1e-8 || math.Abs(decal.Pitch-wantPitch) > 1e-8 ||
		decal.Sprite.Alpha != sdkworld.SpriteBlend {
		t.Fatalf("slope decal = %+v, want yaw=%g pitch=%g", decal, wantYaw, wantPitch)
	}
	if got := actors["gallery-a/room/marker"].Actor.Tags; !slices.Equal(got, []string{"gallery", "gallery-a"}) {
		t.Fatalf("gallery-a tag override = %v", got)
	}
	if got := actors["gallery-b/room/marker"].Actor.Tags; !slices.Equal(got, []string{"gallery", "gallery-b"}) {
		t.Fatalf("gallery-b tag override = %v", got)
	}
}
