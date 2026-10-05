// world-camera-lighting prepares an ignored candidate-SDK preview from the
// candidate sample, retaining its source-v6 static solids.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"
)

var errCandidateMaterials = errors.New("candidate material declarations are invalid")

func main() {
	if err := prepare(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func prepare() error {
	source := filepath.Join("samples", "world-camera")

	destination := filepath.Join("dist", "world-camera-lighting")
	if err := os.RemoveAll(destination); err != nil {
		return err
	}

	if err := os.MkdirAll(destination, 0o750); err != nil {
		return err
	}

	for _, name := range []string{"src", "levels", "ui"} {
		if err := os.CopyFS(filepath.Join(destination, name), os.DirFS(filepath.Join(source, name))); err != nil {
			return err
		}
	}

	module, err := os.ReadFile(filepath.Join(source, "go.mod"))
	if err != nil {
		return err
	}

	//nolint:gosec // The output path is a fixed file inside the derived preview.
	if err := os.WriteFile("dist/world-camera-lighting/go.mod", module, 0o600); err != nil {
		return err
	}

	contents, err := os.ReadFile(filepath.Join(source, "karty.toml"))
	if err != nil {
		return err
	}

	var config map[string]any
	if err := toml.Unmarshal(contents, &config); err != nil {
		return err
	}

	config["sdk"] = map[string]any{"version": "0.0.8"}

	contents, err = toml.Marshal(config)
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(destination, "karty.toml"), contents, 0o600); err != nil {
		return err
	}

	if err := prepareWorldLighting(); err != nil {
		return err
	}

	return prepareWorldMaterialStrengths()
}

type materialStrengthProfile struct {
	Normal float64 `toml:"normal"`
	Height float64 `toml:"height"`
	AO     float64 `toml:"ao"`
	Rim    float64 `toml:"rim"`
}

func prepareWorldMaterialStrengths() error {
	levelPath := filepath.Join("dist", "world-camera-lighting", "levels", "showcase", "level.toml")

	contents, err := os.ReadFile(levelPath)
	if err != nil {
		return err
	}

	var level map[string]any
	if err := toml.Unmarshal(contents, &level); err != nil {
		return err
	}

	profiles := worldMaterialProfiles()

	textures, hasTextures := level["textures"].([]any)
	if !hasTextures {
		return fmt.Errorf("candidate level lacks a texture list: %w", errCandidateMaterials)
	}

	applied := 0

	for _, item := range textures {
		texture, validTexture := item.(map[string]any)
		if !validTexture {
			return fmt.Errorf("candidate level has an invalid texture declaration: %w", errCandidateMaterials)
		}

		name, hasName := texture["name"].(string)
		if !hasName {
			return fmt.Errorf("candidate level has an unnamed texture: %w", errCandidateMaterials)
		}

		if profile, found := profiles[name]; found {
			texture["material_strengths"] = profile
			applied++
		}
	}

	if applied != len(profiles) {
		return fmt.Errorf("candidate level has %d of %d expected material profiles: %w", applied, len(profiles), errCandidateMaterials)
	}

	contents, err = toml.Marshal(level)
	if err != nil {
		return err
	}

	return os.WriteFile(levelPath, contents, 0o600)
}

// Pigment differences in marble and plaster should not become deep relief.
// Grout-bearing surfaces retain moderate normals and stronger cavity AO.
//
//nolint:mnd // These authored artistic strengths are the candidate preview's material palette.
func worldMaterialProfiles() map[string]materialStrengthProfile {
	return map[string]materialStrengthProfile{
		"floor":           {Normal: 0.9, Height: 0.8, AO: 0.9, Rim: 0.6},
		"ceiling":         {Normal: 0.8, Height: 0.5, AO: 0.7, Rim: 0.4},
		"wall":            {Normal: 1, Height: 1.4, AO: 0.8, Rim: 0},
		"z-court-marble":  {Normal: 0.25, Height: 0.03, AO: 0.25, Rim: 0.15},
		"z-court-paving":  {Normal: 0.65, Height: 0, AO: 3.5, Rim: 0.2},
		"z-court-mosaic":  {Normal: 0.4, Height: 0.25, AO: 3.5, Rim: 0.2},
		"z-court-plaster": {Normal: 0.15, Height: 0.03, AO: 0.25, Rim: 0},
		"z-court-coffer":  {Normal: 0.65, Height: 0.45, AO: 3, Rim: 0.25},
		"z-court-sky":     {},
		"z-court-water":   {Normal: 0.75, Height: 0.1, AO: 0, Rim: 0.25},
	}
}

func prepareWorldLighting() error {
	worldPath := filepath.Join("dist", "world-camera-lighting", "levels", "showcase", "world.yaml")

	contents, err := os.ReadFile(worldPath)
	if err != nil {
		return err
	}

	var world yaml.Node
	if err := yaml.Unmarshal(contents, &world); err != nil {
		return err
	}

	contents, err = os.ReadFile(filepath.Join("samples", "world-camera", "lighting.yaml"))
	if err != nil {
		return err
	}

	var lighting yaml.Node
	if err := yaml.Unmarshal(contents, &lighting); err != nil {
		return err
	}

	root := world.Content[0]

	// Source v6 retains static solids and opts every surface into compiler-baked mapping. Explicit world
	// units per repeat keep texture density independent of prefab geometry size.
	var mapping yaml.Node
	if err := yaml.Unmarshal([]byte("uv: {mode: triplanar, scale: {x: 1, y: 1}}\n"), &mapping); err != nil {
		return err
	}

	replaceWorldRootFields(root, mapping.Content[0])
	replaceWorldRootFields(root, lighting.Content[0])

	contents, err = yaml.Marshal(&world)
	if err != nil {
		return err
	}

	return os.WriteFile(worldPath, contents, 0o600)
}

// A normal sample may already contain the preview settings. Replace matching
// root fields so repeated preparation remains valid and presets still override.
func replaceWorldRootFields(root, replacement *yaml.Node) {
	for index := 0; index < len(replacement.Content); index += 2 {
		key, value := replacement.Content[index], replacement.Content[index+1]
		found := false

		for existing := 0; existing < len(root.Content); existing += 2 {
			if root.Content[existing].Value == key.Value {
				root.Content[existing+1] = value
				found = true

				break
			}
		}

		if !found {
			root.Content = append(root.Content, key, value)
		}
	}
}
