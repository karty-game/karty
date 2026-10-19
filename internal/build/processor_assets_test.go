package build

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/processorassets"
)

func TestProcessorTexturesAppendIDsAndPublishAtomically(t *testing.T) {
	t.Parallel()
	project, set := processorFixture(t)

	directory := filepath.Join(project, "levels", "village")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(directory, "level.toml"),
		[]byte(
			"[level]\nname='levels.village'\nkind='2d'\n[[textures]]\nname='z'\nsource='z.png'\n[[textures]]\nname='a'\nsource='a.png'\n",
		),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	var pixels bytes.Buffer
	if err := png.Encode(&pixels, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}

	catalog, err := json.Marshal(
		processorassets.Catalog{
			Version: 1,
			Textures: []processorassets.Texture{
				{
					Level:   "village",
					Name:    "_generated",
					Source:  "generated/chunk.png",
					Key:     "generated/chunk.png#0,0,2,2",
					Profile: "sprite",
				},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	response, err := encodeProcessorFiles(
		[]processorFile{
			{Path: "assets.json", Data: catalog},
			{Path: "generated/chunk.png", Data: pixels.Bytes()},
			{Path: "map.bin", Data: []byte("map")},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	run := func(context.Context, string, []string, []byte) ([]byte, error) { return response, nil }
	if err := runProcessorsWith(t.Context(), project, set, run); err != nil {
		t.Fatal(err)
	}

	root, err := os.OpenRoot(project)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	textures, err := processorassets.Read(root)
	if err != nil || len(textures) != 1 || textures[0].ID != 3 {
		t.Fatal("generated assets shifted authored IDs", textures, err)
	}

	source, err := os.ReadFile(filepath.Join(project, ".karty/processed/processed.go"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(source), "return 3,true") || strings.Count(string(source), "//go:embed") != 1 {
		t.Fatal("generated image embedded in cartridge instead of level", string(source))
	}

	before, err := root.ReadFile(processorassets.RegistryName)
	if err != nil {
		t.Fatal(err)
	}

	response, err = encodeProcessorFiles(
		[]processorFile{{Path: "assets.json", Data: catalog}, {Path: "generated/chunk.png", Data: []byte("bad PNG")}},
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := runProcessorsWith(t.Context(), project, set, run); err == nil {
		t.Fatal("corrupt generated image accepted")
	}

	after, err := root.ReadFile(processorassets.RegistryName)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed generated asset transaction replaced valid registry", err)
	}
}

func TestRemovingProcessorRemovesGeneratedTextures(t *testing.T) {
	t.Parallel()
	project, set := processorFixture(t)

	directory := filepath.Join(project, "levels/village")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(directory, "level.toml"),
		[]byte("[level]\nname='levels.village'\nkind='2d'\n"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	var pixels bytes.Buffer
	if err := png.Encode(&pixels, image.NewNRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}

	catalog, err := json.Marshal(
		processorassets.Catalog{
			Version: 1,
			Textures: []processorassets.Texture{
				{Level: "village", Name: "chunk", Source: "chunk.png", Key: "chunk.png#0,0,1,1", Profile: "sprite"},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	response, err := encodeProcessorFiles([]processorFile{{Path: "assets.json", Data: catalog}, {Path: "chunk.png", Data: pixels.Bytes()}})
	if err != nil {
		t.Fatal(err)
	}

	run := func(context.Context, string, []string, []byte) ([]byte, error) { return response, nil }
	if err := runProcessorsWith(t.Context(), project, set, run); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(project, "karty.toml"),
		[]byte("[project]\nname='test'\n[sdk]\nversion='0.0.13'\n"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	if err := runProcessorsWith(t.Context(), project, set, func(context.Context, string, []string, []byte) ([]byte, error) {
		t.Fatal("removed processor executed")

		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}

	root, err := os.OpenRoot(project)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	textures, err := processorassets.Read(root)
	if err != nil || len(textures) != 0 {
		t.Fatal("removed processor left stale asset registry", textures, err)
	}

	if _, err := root.Stat(".karty/processed/assets"); !os.IsNotExist(err) {
		t.Fatal("removed processor left generated PNGs", err)
	}

	source, err := root.ReadFile(".karty/processed/processed.go")
	if err != nil || strings.Contains(string(source), "return 1,true") {
		t.Fatal("removed processor left stale ID lookup", err)
	}
}
