package levelbuild

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/character"
	"github.com/karty-game/karty/internal/packages"
)

func TestCharacterPackageInputsAreStagedWithoutWorldSource(t *testing.T) {
	t.Parallel()
	levelDirectory, packageDirectory := t.TempDir(), t.TempDir()

	contents := []byte("synthetic source bytes; compilation tested by characterimport")
	if err := os.WriteFile(filepath.Join(packageDirectory, "guard.glb"), contents, 0600); err != nil {
		t.Fatal(err)
	}

	root, err := os.OpenRoot(packageDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	set := &packages.Set{Items: map[string]*packages.Resolved{"actors": {Root: root}}}
	definition := manifest{Characters: []characterEntry{{ID: "guard", Model: "@actors/guard.glb", Animations: "@actors/guard.glb"}}}

	resolved, err := resolveCharacterPackages(t.Context(), levelDirectory, definition, set)
	if err != nil {
		t.Fatal(err)
	}

	name := resolved.Characters[0].Model
	if !strings.HasPrefix(name, ".karty/packages/") || name != resolved.Characters[0].Animations {
		t.Fatalf("unexpected staged paths: %+v", resolved.Characters)
	}

	actual, err := readConfinedFile(levelDirectory, name, 1024)
	if err != nil || !bytes.Equal(actual, contents) {
		t.Fatalf("staged source %q: %v", actual, err)
	}

	if resolved.Characters[0].ID != "guard" || resolved.World.Source != "" {
		t.Fatal("changed authored identity")
	}
}

func TestCharacterSourcesProduceDeclaredLevelResource(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()

	for _, name := range []string{"model.glb", "animations.glb"} {
		data, err := os.ReadFile(filepath.Join("../../testdata/characters", name))
		if err != nil {
			t.Fatal(err)
		}

		//nolint:gosec // name comes from the fixed fixture filename list above and directory is test-owned.
		if err = os.WriteFile(filepath.Join(directory, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}

	definition := manifest{Characters: []characterEntry{{ID: "guard", Model: "model.glb", Animations: "animations.glb"}}}
	definition.Level.Kind = "world"
	definition.World.Source = "world.yaml"

	entries, features, err := appendCharacters(t.Context(), directory, definition, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 || entries[0].Name != character.EntryName || len(features) != 1 || features[0] != character.Feature {
		t.Fatalf("incorrect character packaging: %+v %v", entries, features)
	}

	document, err := character.Decode(entries[0].Data)
	if err != nil {
		t.Fatal(err)
	}

	if document.Models[0].ID != "guard" || document.Banks[0].Clips[0].Name != "Walk" {
		t.Fatal("lost model or clip identity")
	}

	definition.Characters[0].Model = "../outside.glb"
	if _, _, err = appendCharacters(t.Context(), directory, definition, nil, nil); err == nil {
		t.Fatal("accepted unconfined model path")
	}
}

func TestCharacterPackageSourcesRejectTraversalAndMissingPackages(t *testing.T) {
	t.Parallel()

	for _, source := range []string{"@actors/../outside.glb", "@actors//absolute.glb", "@missing/file.glb", "@actors/C:/file.glb"} {
		definition := manifest{Characters: []characterEntry{{ID: "guard", Model: source, Animations: source}}}
		if _, err := resolveCharacterPackages(
			t.Context(),
			t.TempDir(),
			definition,
			&packages.Set{Items: map[string]*packages.Resolved{}},
		); err == nil {
			t.Fatalf("accepted %q", source)
		}
	}
}
