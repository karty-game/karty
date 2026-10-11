package assetpipeline

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty/internal/sdk"
)

func reflectanceFixtureManifest() sdk.Manifest {
	manifest := fixtureMaterialManifest()
	manifest.Assets.Capabilities.Runtime = append(manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialReflectanceV1)

	return manifest
}

func writeReflectanceFixture(directory string) error {
	bounds := image.Rect(0, 0, worldmaterial.TileSize, worldmaterial.TileSize)
	if err := fixtureMaterialMaps(directory, bounds); err != nil {
		return err
	}

	for _, name := range []string{"metallic", "smoothness"} {
		pixels := image.NewGray(bounds)
		for row := range bounds.Dy() {
			for column := range bounds.Dx() {
				value := uint8(32 + 64*(column%2))
				if name == "smoothness" {
					value = uint8(255 * (row % 2))
				}

				pixels.SetGray(column, row, color.Gray{Y: value})
			}
		}

		if err := writeMaterialPNG(filepath.Join(directory, "atlas_"+name+".png"), pixels); err != nil {
			return err
		}
	}

	return nil
}

func TestWorldMaterialReflectanceCacheAndLinearMips(t *testing.T) {
	t.Parallel()

	manifest, document, textures := reflectanceFixtureManifest(), fixtureMaterialWorld(), fixtureTextures(t)
	root := t.TempDir()
	calls := 0
	execute := func(_ context.Context, _ sdk.Manifest, directory string, arguments []string) error {
		calls++

		if len(arguments) < 5 || arguments[4] != "height,normal,ao,metallic,smoothness" {
			return ErrCacheRecipe
		}

		return writeReflectanceFixture(directory)
	}

	pair, cold, err := processWorldMaterials(t.Context(), root, manifest, document, textures, execute)
	if err != nil || cold.CacheHit || calls != 4 {
		t.Fatalf("reflectance cold build: calls=%d err=%v", calls, err)
	}

	_, reflectance, err := qoi.Decode(pair.Reflectance)
	if err != nil {
		t.Fatal(err)
	}

	_, tail, err := qoi.Decode(pair.ReflectanceMipTail)
	if err != nil {
		t.Fatal(err)
	}

	for slot, rect := range pair.Layout.Materials {
		if got := reflectance.NRGBAAt(rect.X, rect.Y); got != (color.NRGBA{R: 32, A: 255}) {
			t.Fatalf("packed metallic/smoothness: %v", got)
		}

		if got := reflectance.NRGBAAt(rect.X-1, rect.Y-1); got != reflectance.NRGBAAt(rect.X, rect.Y) {
			t.Fatalf("reflectance gutter differs: %v", got)
		}

		for level := 1; level <= worldmaterial.MipLevels; level++ {
			mipRect, _ := pair.Layout.MipRect(level, slot, true)

			want := color.NRGBA{R: 64, G: 128, A: 255}
			if got := tail.NRGBAAt(mipRect.X, mipRect.Y); got != want {
				t.Fatalf("reflectance must average scalar channels at level %d: %v != %v", level, got, want)
			}

			if got := tail.NRGBAAt(mipRect.X-1, mipRect.Y-1); got != want {
				t.Fatalf("reflectance mip gutter: %v", got)
			}

			unusedRect, _ := pair.Layout.MipRect(level, slot, false)
			if got := tail.NRGBAAt(unusedRect.X, unusedRect.Y); got != (color.NRGBA{A: 255}) {
				t.Fatalf("unused reflectance tail region: %v", got)
			}
		}
	}

	warmPair, warm, err := processWorldMaterials(t.Context(), root, manifest, document, textures, execute)
	if err != nil || !warm.CacheHit || calls != 4 || !bytes.Equal(pair.Reflectance, warmPair.Reflectance) ||
		!bytes.Equal(pair.ReflectanceMipTail, warmPair.ReflectanceMipTail) {
		t.Fatalf("warm reflectance atlas: calls=%d err=%v", calls, err)
	}
	// Reordering atlas positions must reuse both channels of every tile.
	document.Sectors[0].FloorMaterial, document.Sectors[0].CeilingMaterial = 3, 9

	_, reordered, err := processWorldMaterials(t.Context(), root, manifest, document, textures, execute)
	if err != nil || reordered.CacheHit || calls != 4 {
		t.Fatalf("atlas rearrangement reran native tool: calls=%d err=%v", calls, err)
	}

	textures[9] = fixtureQOI(t, 2, 2, color.NRGBA{R: 210, A: 255})

	_, _, err = processWorldMaterials(t.Context(), root, manifest, document, textures, execute)
	if err != nil || calls != 5 {
		t.Fatalf("changed texture must regenerate only its maps: calls=%d err=%v", calls, err)
	}
}

func TestWorldMaterialReflectanceSDKGateAndSecondaryNeutral(t *testing.T) {
	t.Parallel()
	root, textures := t.TempDir(), fixtureTextures(t)
	document := world.Document{Sectors: []world.Sector{{FloorMaterial: 9, CeilingMaterial: 9,
		FloorSecondary: &world.SurfaceSecondary{Material: 3}}}}
	calls := 0
	execute := func(_ context.Context, manifest sdk.Manifest, directory string, arguments []string) error {
		calls++

		if materialReflectanceEnabled(manifest) {
			return writeReflectanceFixture(directory)
		}

		if arguments[4] != "height,normal,ao" {
			return ErrCacheRecipe
		}

		return fixtureMaterialMaps(directory, image.Rect(0, 0, worldmaterial.TileSize, worldmaterial.TileSize))
	}

	legacy, _, err := processWorldMaterials(t.Context(), root, fixtureMaterialManifest(), document, textures, execute)
	if err != nil || calls != 1 || len(legacy.Reflectance) != 0 || len(legacy.ReflectanceMipTail) != 0 {
		t.Fatalf("legacy SDK reflectance gate: calls=%d err=%v", calls, err)
	}

	pair, _, err := processWorldMaterials(t.Context(), root, reflectanceFixtureManifest(), document, textures, execute)
	if err != nil || calls != 2 {
		t.Fatalf("reflectance policy must invalidate native cache: calls=%d err=%v", calls, err)
	}

	_, reflectance, err := qoi.Decode(pair.Reflectance)
	if err != nil {
		t.Fatal(err)
	}

	for _, rect := range pair.Layout.Materials {
		if rect.MaterialID == 3 && reflectance.NRGBAAt(rect.X, rect.Y) != (color.NRGBA{A: 255}) {
			t.Fatal("secondary-only material must retain neutral metallic/smoothness")
		}
	}
}

func TestWorldMaterialReflectanceMissingMapNeverPublishesAtlas(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	execute := func(_ context.Context, _ sdk.Manifest, directory string, _ []string) error {
		return fixtureMaterialMaps(directory, image.Rect(0, 0, worldmaterial.TileSize, worldmaterial.TileSize))
	}

	_, _, err := processWorldMaterials(t.Context(), root, reflectanceFixtureManifest(), fixtureMaterialWorld(), fixtureTextures(t), execute)
	if !errors.Is(err, ErrCacheOutput) {
		t.Fatalf("missing companion map accepted: %v", err)
	}

	payloads, err := filepath.Glob(filepath.Join(root, ".karty", "cache", "assets", "v3", "*", "payload"))
	if err != nil || len(payloads) != 0 {
		t.Fatalf("incomplete companion cache published: %v, %v", payloads, err)
	}
}

func TestWorldMaterialReflectanceTileCacheRejectsMalformedFraming(t *testing.T) {
	t.Parallel()

	manifest := reflectanceFixtureManifest()

	layout, err := worldmaterial.NewLayout([]uint32{0})
	if err != nil {
		t.Fatal(err)
	}

	recipe, err := worldMaterialRecipe(manifest, layout, nil, "linux-amd64", strings.Repeat("a", 64), nil)
	if err != nil {
		t.Fatal(err)
	}

	source, err := SnapshotBytes([]byte("fixture"))
	if err != nil {
		t.Fatal(err)
	}

	for _, length := range []uint32{0, 1, 0xffffffff} {
		payload := make([]byte, 9)
		binary.BigEndian.PutUint32(payload[:4], length)
		binary.BigEndian.PutUint32(payload[4:8], length)

		artifact, err := NewProjectCache(t.TempDir()).Resolve(t.Context(), source, recipe,
			func(context.Context, SourceSnapshot, Recipe) (Processed, error) {
				return Processed{Encoding: worldMaterialAssetProcessor, Payload: payload}, nil
			})
		if err != nil {
			t.Fatal(err)
		}

		if _, err := loadMaterialTile(t.Context(), artifact, image.Rect(0, 0, 256, 256), true); !errors.Is(err, ErrCacheOutput) {
			t.Fatalf("invalid companion cache framing accepted: %v", err)
		}
	}
}
