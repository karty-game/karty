package packages

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func materializeFixture(t *testing.T, profile, settings string) (string, string) {
	t.Helper()
	kit, project := fixture(t)
	write(t, filepath.Join(kit, ManifestName), []byte(`format_version = 1
name = "placeholder-kit"
version = "0.1.0"
[[textures]]
name = "plain"
source = "assets/checker.png"
profile = "environment"
[[textures]]
name = "steel"
source = "assets/frame.png"
profile = "`+profile+`"
[textures.materialize]
`+settings))

	return kit, project
}

func TestPackageMaterializeValidatesCompleteManifest(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name, profile, settings, message string
	}{
		{"unknown-preset", "environment", `preset="not-a-preset"`, "unknown Materialize preset"},
		{"unknown-key", "environment", `roughness=1`, "strict mode"},
		{"sprite", "sprite", `preset="metal"`, "environment profile"},
		{"empty-sprite-settings", "sprite", "", "environment profile"},
		{"negative", "environment", `normal_strength=-0.1`, "normal-strength"},
		{"too-large", "environment", `height_contrast=16.01`, "height-contrast"},
		{"nonfinite-metallic", "environment", `metallic_scale=inf`, "metallic-scale"},
		{"nonfinite-ao", "environment", `ao_depth_scale=nan`, "ao-depth-scale"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			kit, project := materializeFixture(t, test.profile, test.settings)
			// The game never selects steel. Its bad settings must still prevent
			// import/export rather than waiting for a later texture selection.
			if _, err := Import(t.Context(), project, kit, false); err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("import accepted invalid unselected metadata or lost context: %v", err)
			}

			if _, _, err := Export(t.Context(), kit, filepath.Join(t.TempDir(), "bad.kpkg")); err == nil {
				t.Fatal("export accepted invalid metadata")
			}
		})
	}
}

func TestPackageMaterializeArchivePreservesSettingsAndPruning(t *testing.T) {
	t.Parallel()
	kit, project := materializeFixture(t, "environment", `preset="metal"
normal_strength=0
height_contrast=1.25
metallic_scale=16
`)
	archive := filepath.Join(t.TempDir(), "materials.kpkg")
	export(t, kit, archive)

	if _, err := Import(t.Context(), project, archive, false); err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(kit); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(archive); err != nil {
		t.Fatal(err)
	}

	set := restore(t, project)
	contents := read(t, "../../testdata/local-packages/games/walk/authoring/room/world.yaml")

	marker := strings.Index(string(contents), "# Placements")
	if marker < 0 {
		t.Fatal("world fixture is missing placements marker")
	}

	contents = contents[:marker]
	contents = []byte(strings.ReplaceAll(string(contents), "std.checker", "placeholder-kit.steel"))

	_, textures, err := ResolveWorld(t.Context(), set, contents, nil)
	if err != nil || len(textures) != 1 {
		t.Fatalf("resolve selected steel: textures=%v err=%v", textures, err)
	}

	settings := textures[0].Materialize
	if settings == nil || settings.Preset != "metal" || settings.NormalStrength == nil || *settings.NormalStrength != 0 ||
		settings.HeightContrast == nil || *settings.HeightContrast != 1.25 || settings.MetallicScale == nil ||
		*settings.MetallicScale != 16 || settings.AODepthScale != nil {
		t.Fatalf("archive lost generation settings or omitted/zero distinction: %+v", settings)
	}

	contents = []byte(strings.ReplaceAll(string(contents), "placeholder-kit.steel", "placeholder-kit.plain"))

	_, textures, err = ResolveWorld(t.Context(), set, contents, nil)
	if err != nil || len(textures) != 1 || textures[0].Name != "placeholder-kit.plain" || textures[0].Materialize != nil {
		t.Fatalf("unused steel retained or settings leaked to plain surface: textures=%v err=%v", textures, err)
	}
}
