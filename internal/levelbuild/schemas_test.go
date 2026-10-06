package levelbuild

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	worldschema "github.com/karty-game/karty/internal/worldbuild/schema"
)

const schemaFixtureWorld = `version: 6
rooms:
  - id: room
    boundary:
      - {id: south, start: {x: 0, y: 0}, end: {x: 1, y: 0}, material: surface}
      - {id: east, start: {x: 1, y: 0}, end: {x: 0, y: 1}, material: surface}
      - {id: west, start: {x: 0, y: 1}, end: {x: 0, y: 0}, material: surface}
    ceiling: {c: 2}
    floor_material: surface
    ceiling_material: surface
`

func schemaFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	directory := filepath.Join(root, "levels", "tiny")
	if err := os.MkdirAll(directory, 0750); err != nil {
		t.Fatal(err)
	}

	for name, contents := range map[string]string{
		"level.toml": "[level]\nname='levels.tiny'\nkind='world'\n[world]\nsource='world.yaml'\n[[textures]]\nname='surface'\nsource='does-not-exist.png'\n",
		"world.yaml": schemaFixtureWorld,
	} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}

	return root
}

func TestGenerateSchemasWithoutAssetsAndRefreshCatalog(t *testing.T) {
	t.Parallel()
	root := schemaFixture(t)

	for _, texture := range []string{"surface", "renamed"} {
		if texture != "surface" {
			for _, file := range []string{"level.toml", "world.yaml"} {
				path := filepath.Join(root, "levels", "tiny", file)

				contents, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				//nolint:gosec // Both path components are fixed fixture filenames inside t.TempDir.
				if err := os.WriteFile(path, bytes.ReplaceAll(contents, []byte("surface"), []byte(texture)), 0600); err != nil {
					t.Fatal(err)
				}
			}
		}

		if count, err := GenerateSchemas(root, true); err != nil || count != 1 {
			t.Fatalf("schema generation read missing assets: %d, %v", count, err)
		}

		path := filepath.Join(root, ".karty", "schemas", "levels", "tiny.schema.json")

		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Contains(contents, []byte(`"../worldsource.base.schema.json"`)) || !bytes.Contains(contents, []byte(`"`+texture+`"`)) ||
			texture == "renamed" && bytes.Contains(contents, []byte(`"surface"`)) {
			t.Fatalf("stale editor schema: %s", contents)
		}

		if _, err := os.Stat(filepath.Join(root, ".karty", "schemas", worldschema.BaseFilename)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSchemaChecksRejectTyposBeforeAssetsAndIgnoreEditedOutput(t *testing.T) {
	t.Parallel()
	root := schemaFixture(t)
	path := filepath.Join(root, "levels", "tiny", "world.yaml")

	input := strings.Replace(schemaFixtureWorld, "floor_material: surface", "floor_material: typo", 1)
	if err := os.WriteFile(path, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := GenerateSchemas(root, false); err != nil {
		t.Fatal("editor schemas must be available while fields are invalid:", err)
	}

	if err := os.WriteFile(filepath.Join(root, ".karty", "schemas", "levels", "tiny.schema.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := GenerateSchemas(root, true); !errors.Is(err, worldschema.ErrValidation) || !strings.Contains(err.Error(), "world.yaml") {
		t.Fatalf("build trusted edited schema: %v", err)
	}

	var definition manifest

	definition.World.Source = "world.yaml"

	definition.Textures = []textureEntry{{Name: "surface", Source: "does-not-exist.png"}}
	if err := validateWorldSchema(filepath.Dir(path), definition); !errors.Is(err, worldschema.ErrValidation) {
		t.Fatalf("standalone level validation accepted typo: %v", err)
	}
}

func TestSchemaInputsStayWithinLevel(t *testing.T) {
	t.Parallel()
	root := schemaFixture(t)

	outside := filepath.Join(t.TempDir(), "outside.yaml")
	if err := os.WriteFile(outside, []byte(schemaFixtureWorld), 0600); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(root, "levels", "tiny", "world.yaml")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(outside, path); err != nil {
		t.Fatal(err)
	}

	if _, err := GenerateSchemas(root, true); !errors.Is(err, ErrSource) {
		t.Fatalf("read source outside level: %v", err)
	}
}

func TestSchemaOutputsRejectSymlinks(t *testing.T) {
	t.Parallel()

	for _, target := range []string{".karty", ".karty/schemas", ".karty/schemas/levels", ".karty/schemas/worldsource.base.schema.json", ".karty/schemas/levels/tiny.schema.json"} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			root := schemaFixture(t)

			outside := t.TempDir()
			if strings.HasSuffix(target, ".json") {
				outside = filepath.Join(outside, "untouched.json")
				if err := os.WriteFile(outside, []byte("untouched"), 0600); err != nil {
					t.Fatal(err)
				}
			}

			path := filepath.Join(root, filepath.FromSlash(target))
			if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
				t.Fatal(err)
			}

			if err := os.Symlink(outside, path); err != nil {
				t.Fatal(err)
			}

			if _, err := GenerateSchemas(root, true); !errors.Is(err, ErrSchemaOutput) {
				t.Fatalf("followed output symlink: %v", err)
			}

			if strings.HasSuffix(target, ".json") {
				contents, err := os.ReadFile(outside)
				if err != nil || string(contents) != "untouched" {
					t.Fatalf("overwrote external file: %s, %v", contents, err)
				}
			}
		})
	}
}
