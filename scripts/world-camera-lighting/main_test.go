package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/worldbuild/source"
)

//nolint:gosec,paralleltest // Fixed fixture paths under t.TempDir; t.Chdir cannot run in parallel.
func TestLightingPreparationRetainsRomanSolidSource(t *testing.T) {
	original, err := os.ReadFile("../../samples/world-camera/levels/showcase/world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	preset, err := os.ReadFile("../../samples/world-camera/lighting.yaml")
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	t.Chdir(root)

	path := filepath.Join("dist", "world-camera-lighting", "levels", "showcase", "world.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}

	basePath := filepath.Join("samples", "world-camera", "levels", "showcase", "world.yaml")
	if err := os.MkdirAll(filepath.Dir(basePath), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(basePath, original, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll("samples/world-camera", 0o750); err != nil {
		t.Fatal(err)
	}

	// The normal source already has lighting: this derived preset must replace
	// it without changing the base sample or duplicating root YAML fields.
	preset = []byte(strings.Replace(string(preset), "ambient: {x: 0.08", "ambient: {x: 0.18", 1))
	if err := os.WriteFile("samples/world-camera/lighting.yaml", preset, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := prepareWorldLighting(); err != nil {
		t.Fatal(err)
	}

	prepared, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	expanded, err := source.Decode(prepared)
	if err != nil {
		t.Fatal(err)
	}

	if len(expanded.Solids) != 8 || expanded.Lighting == nil || len(expanded.Lighting.Lights) != 7 || expanded.UV == nil {
		t.Fatal("lighting generator lost solids, light preset or mapping")
	}

	motion := expanded.Lighting.Lights[6].Motion
	if motion == nil || motion.Offset.X != 12 || motion.PeriodSeconds != 12 {
		t.Fatal("derived preview lost corridor light motion")
	}

	if expanded.Lighting.Ambient.X != 0.18 {
		t.Fatal("derived preset did not override existing lighting")
	}

	if err := prepareWorldLighting(); err != nil {
		t.Fatal(err)
	}

	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(prepared, second) {
		t.Fatal("repeated lighting preparation changed the derived source")
	}

	unchanged, err := os.ReadFile(basePath)
	if err != nil || !bytes.Equal(original, unchanged) {
		t.Fatal("lighting preparation modified the base source")
	}

	profiles := worldMaterialProfiles()
	if len(profiles) != 10 || profiles["z-court-marble"].Height != 0.03 {
		t.Fatal("material profiles changed")
	}
}
