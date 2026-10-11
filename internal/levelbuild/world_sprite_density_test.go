package levelbuild

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
	uicompiler "github.com/karty-game/karty-ui/compiler"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/pelletier/go-toml/v2"
)

func spriteDensityFixture(t *testing.T) (string, []byte, []byte) {
	t.Helper()
	root := t.TempDir()
	input := image.NewNRGBA(image.Rect(0, 0, 256, 256))

	for y := 32; y < 224; y++ {
		for x := 32; x < 224; x++ {
			input.SetNRGBA(x, y, color.NRGBA{R: 180, G: 90, B: 20, A: 255})
		}
	}

	var source bytes.Buffer
	if err := png.Encode(&source, input); err != nil {
		t.Fatal(err)
	}

	manifestBytes := []byte(`[level]
name = "levels.tiny"
kind = "world"
[world]
source = "world.yaml"
max_sprite_texels_per_unit = 64
[[textures]]
name = "surface"
source = "texture.png"
[[textures]]
name = "decal"
source = "texture.png"
`)
	worldBytes := []byte(schemaFixtureWorld + `contents:
  - id: decal
    kind: actor
    position: {x: 0.2, y: 0.2, z: 1}
    actor:
      sprite: {texture: decal, facing: fixed, alpha: blend, width: 0.85, height: 1.2, origin_x: 0.5, origin_y: 0.5}
`)

	for name, contents := range map[string][]byte{"level.toml": manifestBytes, "world.yaml": worldBytes, "texture.png": source.Bytes()} {
		if err := os.WriteFile(filepath.Join(root, name), contents, 0600); err != nil {
			t.Fatal(err)
		}
	}

	return root, manifestBytes, source.Bytes()
}

func TestSpriteDensityBuildResizesWithoutChangingArtwork(t *testing.T) {
	t.Parallel()
	root, manifestBytes, sourceBytes := spriteDensityFixture(t)

	selected, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	assets := &assetBuild{projectRoot: root, manifest: geometrySDK(selected)}

	var previous []byte

	for iteration := range 2 {
		artifact, err := build(context.Background(), root, 4, uicompiler.DefaultTheme(), assets)
		if err != nil {
			t.Fatal(err)
		}

		if iteration > 0 && !bytes.Equal(previous, artifact.Bytes) {
			t.Fatal("density processing changed deterministic packaging")
		}

		previous = artifact.Bytes
		for _, texture := range artifact.Textures {
			if texture.Name == "decal" && (texture.Width != 54 || texture.Height != 54) {
				t.Fatalf("decal density/aspect ratio: %dx%d", texture.Width, texture.Height)
			}

			if texture.Name == "surface" && (texture.Width != 256 || texture.Height != 256) {
				t.Fatal("density changed a surface material")
			}

			if iteration > 0 && !texture.CacheHit {
				t.Fatal("density recipe failed to reuse cached output")
			}
		}
	}

	unchanged, err := os.ReadFile(filepath.Join(root, "texture.png"))
	if err != nil || !bytes.Equal(unchanged, sourceBytes) {
		t.Fatal("density processing modified original artwork", err)
	}

	var definition manifest
	if err := toml.Unmarshal(manifestBytes, &definition); err != nil {
		t.Fatal(err)
	}

	definition.World.MaxSpriteTexelsPerUnit = nil

	defaulted, err := prepareSpriteDensity(root, definition, assets)
	if err != nil || defaulted.Textures[1].Transform.MaxWidth != 108 {
		t.Fatal("omitted density did not cap oversized sprite by default", err)
	}

	zero := 0.0
	definition.World.MaxSpriteTexelsPerUnit = &zero

	untouched, err := prepareSpriteDensity(root, definition, assets)
	if err != nil || untouched.Textures[1].Transform.MaxWidth != 0 {
		t.Fatal("explicit opt-out did not preserve existing transforms", err)
	}
}

func TestSpriteDensityUsesSmallestScaledPlacementAndFlipbookFrames(t *testing.T) {
	t.Parallel()

	document := world.Document{
		Contents: []world.Content{
			{Actor: &world.Actor{
				Scale:     world.Vec3{X: 2, Y: 0.5, Z: 0.25},
				Sprite:    &world.Sprite{AssetID: 1, Facing: world.SpriteCross, Width: 2, Height: 4},
				Animation: &world.AnimationBinding{Preset: "flip"},
			}},
			{Actor: &world.Actor{
				Scale:     world.Vec3{X: 1, Y: 1, Z: 1},
				Sprite:    &world.Sprite{AssetID: 1, Width: 0.75, Height: 2},
				Animation: &world.AnimationBinding{Preset: "flip"},
			}},
		},
		Animations: &world.Animations{Presets: []world.AnimationPreset{{Name: "flip", Kind: "flipbook", Frames: []uint32{1, 2}}}},
	}
	for id, bounds := range spriteTextureBounds(document) {
		if bounds.width != 0.75 || bounds.height != 1 {
			t.Fatalf("texture %d bounds=%+v", id, bounds)
		}
	}

	density, zero := 64.0, 0.0
	definition := manifest{Textures: []textureEntry{
		{Name: "first", Transform: textureTransform{MaxWidth: 32}},
		{Name: "second", MaxSpriteTexelsPerUnit: &zero},
	}}
	definition.World.MaxSpriteTexelsPerUnit = &density
	assets := &assetBuild{manifest: sdk.Manifest{}}
	assets.manifest.Assets.TextureProfiles = map[string]sdk.AssetProfile{
		"sprite": {Transform: asset.ImageRecipe{MaxWidth: 4096, MaxHeight: 40}},
	}

	result, err := applySpriteDensity(definition, document, map[string]uint32{"first": 1, "second": 2}, assets)
	if err != nil || result.Textures[0].Transform.MaxWidth != 32 ||
		result.Textures[0].Transform.MaxHeight != 40 || result.Textures[1].Transform.MaxWidth != 0 {
		t.Fatalf("authored/profile limits or opt-out lost: %+v %v", result.Textures, err)
	}
}

func TestSpriteDensityRejectsInvalidAndUnsafeSettings(t *testing.T) {
	t.Parallel()

	for _, value := range []float64{-1, math.NaN(), math.Inf(1), 4097} {
		definition := manifest{}
		definition.World.MaxSpriteTexelsPerUnit = &value

		if _, err := prepareSpriteDensity("", definition, nil); !errors.Is(err, ErrManifest) {
			t.Fatalf("invalid density %v accepted: %v", value, err)
		}
	}

	density := 64.0
	definition := manifest{Textures: []textureEntry{{Name: "shared", MaxSpriteTexelsPerUnit: &density}}}
	document := world.Document{Sectors: []world.Sector{{FloorMaterial: 1}}}

	if _, err := applySpriteDensity(definition, document, map[string]uint32{"shared": 1}, &assetBuild{}); !errors.Is(err, ErrManifest) {
		t.Fatal("explicit sprite density accepted for a surface material", err)
	}

	if actual := spriteDensityLimit(0, 0, 1e30); actual != asset.MaxTextureDimension {
		t.Fatal("large world scale overflowed pixel bounds", actual)
	}
}
