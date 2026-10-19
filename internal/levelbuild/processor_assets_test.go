package levelbuild_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty/internal/levelbuild"
	"github.com/karty-game/karty/internal/processorassets"
)

func TestBuildPackagesProcessorTexturesAfterAuthoredIDs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeLevel(t, root, "village", "levels.village", "map")

	var imageData bytes.Buffer

	pixels := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	pixels.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})

	if err := png.Encode(&imageData, pixels); err != nil {
		t.Fatal(err)
	}

	source := filepath.Join(root, ".karty/processed/assets")
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{filepath.Join(source, "chunk.png"), filepath.Join(root, "levels/village/original.png")} {
		if err := os.WriteFile(name, imageData.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}

	definition := "[level]\nname='levels.village'\nkind='level'\n[[textures]]\nname='original'\nsource='original.png'\n"
	if err := os.WriteFile(filepath.Join(root, "levels/village/level.toml"), []byte(definition), 0600); err != nil {
		t.Fatal(err)
	}

	catalog, err := json.Marshal(
		processorassets.Catalog{
			Version: 1,
			Textures: []processorassets.Texture{
				{
					Level:     "village",
					Name:      "_chunk",
					Source:    ".karty/processed/assets/chunk.png",
					Key:       "generated/chunk.png#0,0,2,2",
					Profile:   "sprite",
					Processor: "tiled.compile",
					ID:        2,
				},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, ".karty/processed/assets.json"), catalog, 0600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, ".karty/processed/.owner"), []byte(processorassets.Marker), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := levelbuild.BuildAllWithAssets(t.Context(), root, 9, "", textureSDK(asset.ProcessorQOIv1))
	if err != nil {
		t.Fatal(err)
	}

	archive, err := level.DecodePassiveModule(result[0].Bytes)
	if err != nil {
		t.Fatal(err)
	}

	for _, identifier := range []uint32{1, 2} {
		found := false

		for _, entry := range archive.Entries {
			if entry.Name == level.TextureEntryName(identifier) {
				found = true
			}
		}

		if !found {
			t.Fatal("generated image absent from level", identifier)
		}
	}
	// A catalog authored before another texture was added cannot silently remap IDs.
	if err := os.WriteFile(
		filepath.Join(root, "levels/village/level.toml"),
		[]byte(definition+"[[textures]]\nname='other'\nsource='original.png'\n"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	if _, err := levelbuild.BuildAllWithAssets(t.Context(), root, 9, "", textureSDK(asset.ProcessorQOIv1)); err == nil {
		t.Fatal("stale generated texture IDs accepted")
	}
}
