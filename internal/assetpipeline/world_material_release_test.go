package assetpipeline

import (
	"bytes"
	"context"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

func TestWorldMaterialReleaseAtlas(t *testing.T) {
	t.Parallel()

	if os.Getenv("KARTY_TEST_MATERIALIZE_ATLAS") != "1" {
		t.Skip("set KARTY_TEST_MATERIALIZE_ATLAS=1 for real native atlas generation")
	}

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	if !materialReflectanceEnabled(manifest) {
		t.Fatal("selected SDK must retain material reflectance support")
	}

	if _, supported := manifest.Artifacts.Materialize[runtime.GOOS+"-"+runtime.GOARCH]; !supported {
		t.Skip("no released Materialize for this platform")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()

	calls := 0
	execute := func(ctx context.Context, manifest sdk.Manifest, directory string, args []string) error {
		calls++

		if err := runManagedMaterialize(ctx, manifest, directory, args); err != nil {
			return err
		}

		for _, name := range []string{"normal", "height", "ao", "metallic", "smoothness"} {
			encoded, err := readBoundedMaterialFile(filepath.Join(directory, "atlas_"+name+".png"))
			if err != nil {
				return err
			}

			config, err := png.DecodeConfig(bytes.NewReader(encoded))
			if err != nil {
				return err
			}

			t.Logf("actual Materialize %s: %dx%d, PNG bit depth %d, color model %T", name,
				config.Width, config.Height, encoded[24], config.ColorModel)
		}

		return nil
	}
	root := t.TempDir()
	document := world.Document{Sectors: []world.Sector{{FloorMaterial: 9, CeilingMaterial: 9}}}
	textures := fixtureTextures(t)

	normal, height, metallic, ao := 0.8, 1.0, 1.0, 0.5
	options := map[uint32]MaterializeOptions{
		9: {Preset: "metal", NormalStrength: &normal, HeightContrast: &height, MetallicScale: &metallic, AODepthScale: &ao},
	}

	pair, cold, err := processWorldMaterials(ctx, root, manifest, document, textures, execute, options)
	if err != nil {
		t.Fatal(err)
	}

	warmPair, warm, err := processWorldMaterials(ctx, root, manifest, document, textures, execute, options)
	if err != nil || calls != 1 || cold.CacheHit || !warm.CacheHit || !bytes.Equal(pair.Data, warmPair.Data) ||
		len(pair.Reflectance) == 0 || len(pair.ReflectanceMipTail) == 0 ||
		!bytes.Equal(pair.Reflectance, warmPair.Reflectance) || !bytes.Equal(pair.ReflectanceMipTail, warmPair.ReflectanceMipTail) {
		t.Fatalf("actual native cold/warm atlas: calls=%d warm=%v err=%v", calls, warm.CacheHit, err)
	}
}
