package levelbuild

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty/internal/packages"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

func TestPackageInputsPreserveAuthoredLevelAndShareSchemaBuildAndBakeSources(t *testing.T) {
	t.Parallel()
	project := t.TempDir()

	kit, err := filepath.Abs("../../testdata/local-packages/library")
	if err != nil {
		t.Fatal(err)
	}

	writePrebakeFixture(
		t,
		filepath.Join(project, "karty.toml"),
		[]byte("[sdk]\nversion = '0.0.13'\n[packages.placeholder-kit]\nfolder = '"+filepath.ToSlash(kit)+"'\n"),
	)

	directory := filepath.Join(project, "levels/room")
	if err := os.MkdirAll(directory, 0750); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"level.toml", "world.yaml"} {
		data, err := os.ReadFile(filepath.Join("../../testdata/local-packages/games/walk/authoring/room", name))
		if err != nil {
			t.Fatal(err)
		}

		writePrebakeFixture(t, filepath.Join(directory, name), bytes.ReplaceAll(data, []byte("std."), []byte("placeholder-kit.")))
	}
	// The game keeps its local material declaration; it can be the same bytes
	// as another texture without sharing logical material identity.
	data, err := os.ReadFile(filepath.Join(kit, "assets/frame.png"))
	if err != nil {
		t.Fatal(err)
	}

	writePrebakeFixture(t, filepath.Join(directory, "accent.png"), data)

	before, err := os.ReadFile(filepath.Join(directory, "world.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	if count, err := GenerateSchemasContext(t.Context(), project, true); err != nil || count != 1 {
		t.Fatalf("schema count %d: %v", count, err)
	}

	set, err := packages.Restore(t.Context(), project)
	if err != nil {
		t.Fatal(err)
	}
	defer set.Close()

	definition, err := readLevelManifest(directory, nil)
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := resolveLevelPackages(t.Context(), directory, definition, set)
	if err != nil {
		t.Fatal(err)
	}

	if len(resolved.Textures) != 4 || resolved.World.Source == definition.World.Source {
		t.Fatalf("wrong package texture/source resolution: %+v", resolved)
	}

	world, err := source.Load(directory, resolved.World.Source)
	if err != nil {
		t.Fatal(err)
	}

	if len(world.Solids) != 6 {
		t.Fatalf("nested crate stack not expanded: %d", len(world.Solids))
	}

	if _, _, err := loadBakeMaterials(directory, resolved); err != nil {
		t.Fatalf("baker could not consume package materials: %v", err)
	}

	after, err := os.ReadFile(filepath.Join(directory, "world.yaml"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("rewrote authored world")
	}

	again, err := resolveLevelPackages(t.Context(), directory, definition, set)
	if err != nil || again.World.Source != resolved.World.Source {
		t.Fatalf("non-deterministic staged input: %v", err)
	}
}
