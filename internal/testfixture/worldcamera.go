// Package testfixture prepares isolated sample inputs for public build tests.
package testfixture

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"
)

// WorldCameraGeometry retains the current sample's geometry, mapping and source
// textures while disabling its independent material/lighting/bake opt-ins. Tests
// can select each candidate capability without invoking external GPU tooling.
func WorldCameraGeometry(t *testing.T, sourceRoot string) string {
	t.Helper()

	root := t.TempDir()
	if err := os.CopyFS(filepath.Join(root, "levels"), os.DirFS(filepath.Join(sourceRoot, "levels"))); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(root, "levels", "showcase", "level.toml")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var manifest map[string]any
	if err := toml.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}

	delete(manifest, "lightmap")

	textures, ok := manifest["textures"].([]any)
	if !ok {
		t.Fatal("sample textures are missing")
	}

	for _, declaration := range textures {
		texture, ok := declaration.(map[string]any)
		if !ok {
			t.Fatal("sample texture declaration is invalid")
		}

		delete(texture, "material_strengths")
	}

	data, err = toml.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	path = filepath.Join(root, "levels", "showcase", "world.yaml")

	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}

	mapping := document.Content[0]
	for index := 0; index < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == "lighting" {
			mapping.Content = append(mapping.Content[:index], mapping.Content[index+2:]...)

			break
		}
	}

	data, err = yaml.Marshal(&document)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	return root
}
