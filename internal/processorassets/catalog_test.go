package processorassets

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedCatalogRejectsEscapesDuplicatesAndStaleIDs(t *testing.T) {
	t.Parallel()

	base := Texture{Level: "village", Name: "chunk", Source: "chunk.png", Key: "chunk.png#0,0,2,2", Profile: "sprite"}
	for _, invalid := range []Texture{{Level: "../village", Name: "chunk", Source: "chunk.png", Key: "k", Profile: "sprite"}, {Level: "village", Name: "chunk", Source: "../chunk.png", Key: "k", Profile: "sprite"}} {
		data, err := json.Marshal(Catalog{Version: 1, Textures: []Texture{invalid}})
		if err != nil {
			t.Fatal(err)
		}

		if _, err := Decode(data); err == nil {
			t.Fatal("escape accepted", invalid)
		}
	}

	duplicate, err := json.Marshal(Catalog{Version: 1, Textures: []Texture{base, base}})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Decode(duplicate); err == nil {
		t.Fatal("duplicate accepted")
	}

	if _, err := Decode([]byte(`{"version":1,"textures":[]} {}`)); err == nil {
		t.Fatal("trailing catalog accepted")
	}

	directory := t.TempDir()
	if err := os.MkdirAll(filepath.Join(directory, "levels/village"), 0700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(directory, "levels/village/level.toml"),
		[]byte("[[textures]]\nname='chunk'\n"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	if _, err := AssignIDs(root, []Texture{base}); err == nil {
		t.Fatal("generated texture replaced authored name")
	}
}

func TestRegistryRequiresOwnershipAndBoundsBeforeReading(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	if err := os.MkdirAll(filepath.Join(directory, ".karty/processed"), 0700); err != nil {
		t.Fatal(err)
	}

	registry := filepath.Join(directory, RegistryName)
	if err := os.WriteFile(registry, []byte(`{"version":1,"textures":[]}`), 0600); err != nil {
		t.Fatal(err)
	}

	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	if _, err := Read(root); err == nil {
		t.Fatal("unowned registry accepted")
	}

	if err := os.WriteFile(filepath.Join(directory, ".karty/processed/.owner"), []byte(Marker), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := Read(root); err != nil {
		t.Fatal("valid owned registry rejected", err)
	}

	if err := os.Truncate(registry, MaxCatalogBytes+1); err != nil {
		t.Fatal(err)
	}

	if _, err := Read(root); err == nil {
		t.Fatal("oversized registry accepted")
	}
}

func TestCatalogEncodingPreservesCanonicalBytesAndBounds(t *testing.T) {
	t.Parallel()

	catalog := Catalog{
		Version: 1,
		Textures: []Texture{
			{
				Level:     "village",
				Name:      "chunk",
				Source:    "generated/chunk.png",
				Key:       "chunk.png#0,0,2,2",
				Profile:   "sprite",
				Processor: "tiled.compile",
				ID:        3,
			},
		},
	}

	encoded, err := Encode(catalog)

	const expected = `{"version":1,"textures":[{"level":"village","name":"chunk","source":"generated/chunk.png","key":"chunk.png#0,0,2,2","profile":"sprite","processor":"tiled.compile","id":3}]}`

	if err != nil || string(encoded) != expected {
		t.Fatal("catalog encoding changed registry bytes", string(encoded), err)
	}

	catalog.Textures[0].Key = strings.Repeat("x", MaxCatalogBytes)
	if encoded, err := Encode(catalog); !errors.Is(err, ErrCatalog) || encoded != nil {
		t.Fatal("oversized registry encoding accepted", err)
	}
}
