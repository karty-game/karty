package project_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/project"
)

func TestLoadReadsProjectConfig(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()

	contents := "[project]\nname = \"pong\"\n\n[sdk]\nversion = \"0.0.1\"\n"
	if err := os.WriteFile(filepath.Join(directory, "karty.toml"), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := project.Load(directory)
	if err != nil {
		t.Fatal(err)
	}

	if config.Project.Name != "pong" || config.SDK.Version != "0.0.1" {
		t.Fatalf("Load() = %+v, want pong/0.0.1", config)
	}

	if config.Project.Compiler != "tinygo" {
		t.Errorf("Load() compiler = %q, want tinygo", config.Project.Compiler)
	}
}

func TestLoadReadsGoCompiler(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()

	contents := "[project]\nname = \"pong\"\ncompiler = \"go\"\n\n[sdk]\nversion = \"0.0.1\"\n"
	if err := os.WriteFile(filepath.Join(directory, "karty.toml"), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := project.Load(directory)
	if err != nil {
		t.Fatal(err)
	}

	if config.Project.Compiler != "go" {
		t.Fatalf("Load() compiler = %q, want go", config.Project.Compiler)
	}
}

func TestLoadProjectFontRoles(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "display.ttf"), []byte("font"), 0o600); err != nil {
		t.Fatal(err)
	}

	contents := `[project]
name = "demo"
[sdk]
version = "0.0.1"
[[assets.font]]
role = "display"
source = "display.ttf"
`
	if err := os.WriteFile(filepath.Join(directory, "karty.toml"), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := project.Load(directory)
	if err != nil {
		t.Fatal(err)
	}

	if len(config.Assets.Fonts) != 1 || config.Assets.Fonts[0].Role != "display" {
		t.Fatalf("font roles = %+v", config.Assets.Fonts)
	}
}

func TestLoadRejectsInvalidProjectFontRole(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "display.ttf"), []byte("font"), 0o600); err != nil {
		t.Fatal(err)
	}

	contents := `[project]
name = "demo"
[sdk]
version = "0.0.1"
[[assets.font]]
role = "heading"
source = "display.ttf"
`
	if err := os.WriteFile(filepath.Join(directory, "karty.toml"), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := project.Load(directory); err == nil || !strings.Contains(err.Error(), "project font") {
		t.Fatalf("invalid font role error = %v", err)
	}
}

func TestLoadRejectsIncompleteProjectConfig(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()

	if err := os.WriteFile(filepath.Join(directory, "karty.toml"), []byte("[project]\nname = \"pong\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := project.Load(directory)
	if err == nil || !strings.Contains(err.Error(), "project must define project.name and sdk.version") {
		t.Fatalf("Load() error = %v, want incomplete project error", err)
	}
}

func TestLoadRejectsDuplicateTextureNames(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "player.png"), []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}

	contents := `[project]
name = "pong"

[sdk]
version = "0.0.1"

[[assets.texture]]
name = "sprites.player"
source = "player.png"

[[assets.texture]]
name = "sprites.player"
source = "player.png"
`
	if err := os.WriteFile(filepath.Join(directory, "karty.toml"), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := project.Load(directory); err == nil || !strings.Contains(err.Error(), "logical name is duplicated") {
		t.Fatalf("Load() duplicate texture error = %v", err)
	}
}

func TestLoadTextureProfiles(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "player.png"), []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}

	manifest := func(profile string) string {
		return `[project]
name = "pong"

[sdk]
version = "0.0.1"

[[assets.texture]]
name = "sprites.player"
source = "player.png"
` + profile
	}

	if err := os.WriteFile(filepath.Join(directory, "karty.toml"), []byte(manifest("")), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := project.Load(directory)
	if err != nil {
		t.Fatal(err)
	}

	if profile := config.Assets.Textures[0].Profile; profile != project.DefaultTextureProfile {
		t.Errorf("default texture profile = %q, want sprite", profile)
	}
}

func TestLoadLayoutSources(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	if err := os.MkdirAll(filepath.Join(directory, "ui", "layouts"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(directory, "ui", "layouts", "window.ui"), []byte("layout Window {}"), 0o600); err != nil {
		t.Fatal(err)
	}

	manifest := `[project]
name = "demo"
[sdk]
version = "0.0.1"
[[assets.layout]]
source = "ui/layouts/window.ui"
`
	if err := os.WriteFile(filepath.Join(directory, "karty.toml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := project.Load(directory)
	if err != nil || len(config.Assets.Layouts) != 1 || config.Assets.Layouts[0].Source != "ui/layouts/window.ui" {
		t.Fatalf("layout config = %+v, %v", config.Assets.Layouts, err)
	}

	duplicated := manifest + `[[assets.layout]]
source = "ui/layouts/window.ui"
`
	if err := os.WriteFile(filepath.Join(directory, "karty.toml"), []byte(duplicated), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := project.Load(directory); err == nil || !strings.Contains(err.Error(), "project layout") {
		t.Fatalf("accepted duplicate layout: %v", err)
	}
}

//nolint:wsl_v5 // The fixture deliberately groups filesystem setup and assertions.
func TestLoadDiscoversConventionalAssetsAndMergesSparseOverrides(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	for _, path := range []string{
		"assets/textures/sprites/player.png", "ui/views/menu.ui", "ui/components/item-row.ui",
		"ui/layouts/window.ui", "ui/theme.toml",
	} {
		fullPath := filepath.Join(directory, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte("source"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	manifest := `[project]
name = "demo"
[sdk]
version = "0.0.1"
[[assets.texture]]
source = "assets/textures/sprites/player.png"
profile = "interface"
keep = true
`
	if err := os.WriteFile(filepath.Join(directory, "karty.toml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := project.Load(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Assets.Textures) != 1 || config.Assets.Textures[0].Name != "sprites.player" ||
		config.Assets.Textures[0].Profile != "interface" || !config.Assets.Textures[0].Keep {
		t.Fatalf("resolved textures = %+v", config.Assets.Textures)
	}
	if len(config.Assets.UI) != 2 || config.Assets.UI[0].Name != "ui.item-row" ||
		config.Assets.UI[1].Name != "ui.menu" {
		t.Fatalf("resolved UI = %+v", config.Assets.UI)
	}
	if len(config.Assets.Layouts) != 1 || config.Assets.Theme.Source != "ui/theme.toml" {
		t.Fatalf("resolved layouts/theme = %+v / %+v", config.Assets.Layouts, config.Assets.Theme)
	}
}
