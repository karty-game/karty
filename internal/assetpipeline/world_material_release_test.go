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
	"github.com/karty-game/karty/internal/sdk"
)

func TestWorldMaterialReleaseAtlas(t *testing.T) {
	if os.Getenv("KARTY_TEST_MATERIALIZE_ATLAS") != "1" {
		t.Skip("set KARTY_TEST_MATERIALIZE_ATLAS=1 for real native atlas generation")
	}

	manifest := fixtureMaterialManifest()
	manifest.Tools.MaterializeRevision = "3ad39f1308e3b2b62da81f557adc6d32697b616a"

	for platform, checksum := range map[string]string{
		"linux-amd64":   "676e0adfdec3945f9ffd47585faaabf7307cce0204d0cfc35eef6eb126ff41e6",
		"linux-arm64":   "09d93b61eca2d7f24139294a2900a86bfef911c8d7cf4535431738d292df7b92",
		"darwin-arm64":  "1afd52bff3129506e7b3708b244532ba24476af1d231189e908f343cde8d8d6a",
		"windows-amd64": "3dc10e0ef85471ca7846205619a511e891d2edfc9999a634a7adcd373d5b0a14",
	} {
		artifact := manifest.Artifacts.Materialize[platform]
		artifact.SHA256 = checksum
		manifest.Artifacts.Materialize[platform] = artifact
	}

	if _, supported := manifest.Artifacts.Materialize[runtime.GOOS+"-"+runtime.GOARCH]; !supported {
		t.Skip("no released Materialize for this platform")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()

	calls := 0
	execute := func(ctx context.Context, manifest sdk.Manifest, directory string, args []string) error {
		calls++

		if err := runManagedMaterialize(ctx, manifest, directory, args); err != nil {
			return err
		}

		for _, name := range []string{"normal", "height", "ao"} {
			encoded, err := readBoundedMaterialFile(filepath.Join(directory, "atlas_"+name+".png"), MaxCachePayloadBytes)
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

	pair, cold, err := processWorldMaterials(ctx, root, manifest, document, textures, execute)
	if err != nil {
		t.Fatal(err)
	}

	warmPair, warm, err := processWorldMaterials(ctx, root, manifest, document, textures, execute)
	if err != nil || calls != 1 || cold.CacheHit || !warm.CacheHit || !bytes.Equal(pair.Data, warmPair.Data) {
		t.Fatalf("actual native cold/warm atlas: calls=%d warm=%v err=%v", calls, warm.CacheHit, err)
	}
}
