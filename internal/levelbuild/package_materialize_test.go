package levelbuild

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty/internal/packages"
)

func TestPackageMaterializeSettingsReachWorldGeneration(t *testing.T) {
	t.Parallel()
	project := t.TempDir()

	kit := filepath.Join(project, "packages/materials")
	if err := os.MkdirAll(kit, 0750); err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile("../../testdata/local-packages/library/assets/frame.png")
	if err != nil {
		t.Fatal(err)
	}

	writePrebakeFixture(t, filepath.Join(kit, "steel.png"), contents)
	sourcePNG := contents

	writePrebakeFixture(t, filepath.Join(kit, packages.ManifestName), []byte(`format_version=1
name="materials"
version="0.1.0"
[[textures]]
name="steel"
source="steel.png"
profile="environment"
[textures.materialize]
preset="metal"
normal_strength=0
height_contrast=1.25
metallic_scale=2
ao_depth_scale=0
[[textures]]
name="unused"
source="steel.png"
profile="environment"
[textures.materialize]
preset="concrete"
`))
	writePrebakeFixture(t, filepath.Join(project, "karty.toml"), []byte("[packages.materials]\nfolder='packages/materials'\n"))

	directory := filepath.Join(project, "levels/room")
	if err := os.MkdirAll(directory, 0750); err != nil {
		t.Fatal(err)
	}

	contents, err = os.ReadFile("../../testdata/local-packages/games/walk/authoring/room/world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	marker := bytes.Index(contents, []byte("# Placements"))
	if marker < 0 {
		t.Fatal("world fixture is missing placements marker")
	}

	contents = contents[:marker]
	contents = bytes.ReplaceAll(contents, []byte("std.checker"), []byte("materials.steel"))
	writePrebakeFixture(t, filepath.Join(directory, "world.yaml"), contents)

	set, err := packages.Restore(t.Context(), project)
	if err != nil {
		t.Fatal(err)
	}
	defer set.Close()

	definition, err := resolveLevelPackages(t.Context(), directory, manifest{World: worldSettings{Source: "world.yaml"}}, set)
	if err != nil || len(definition.Textures) != 1 {
		t.Fatalf("selected package texture staging: %+v: %v", definition.Textures, err)
	}

	entry := definition.Textures[0]

	staged, err := os.ReadFile(filepath.Join(directory, filepath.FromSlash(entry.Source)))
	if err != nil || !bytes.Equal(staged, sourcePNG) {
		t.Fatalf("staged input differs from package source: %v", err)
	}

	// Use the ordinary material ID resolution step: the final generation
	// recipe receives package settings exactly like authored level settings.
	resolved, err := resolveMaterializeSettings(definition.Textures,
		[]metadataTexture{{Name: "materials.steel", ID: 7}},
		world.Document{Sectors: []world.Sector{{FloorMaterial: 7}}})
	if err != nil {
		t.Fatal(err)
	}

	options := resolved[7]
	if options.Preset != "metal" || options.NormalStrength == nil || *options.NormalStrength != 0 ||
		options.HeightContrast == nil || *options.HeightContrast != 1.25 ||
		options.MetallicScale == nil || *options.MetallicScale != 2 || options.AODepthScale == nil || *options.AODepthScale != 0 {
		t.Fatalf("generation settings changed during staging: %+v", options)
	}

	if definition.World.Source == "world.yaml" || !strings.HasPrefix(entry.Source, ".karty/packages/") {
		t.Fatal("package inputs escaped ordinary generated staging")
	}
}
