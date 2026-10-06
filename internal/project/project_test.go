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

//nolint:wsl_v5 // All asset kinds are asserted in one manifest-loading scenario.
func TestLoadDiscoversImageAndSoundSourcesWithTransformOverrides(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	for _, relative := range []string{
		"assets/textures/photo.jpg", "assets/textures/panel.webp", "assets/textures/icon.png", "assets/sounds/ui/open.wav",
		"assets/music/theme.wav", "assets/environment/rain.wav",
	} {
		path := filepath.Join(directory, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte("source"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	manifest := `[project]
name = "demo"
[sdk]
version = "0.0.7"
[[assets.texture]]
source = "assets/textures/photo.jpg"
profile = "environment"
[assets.texture.transform]
max_width = 1024
max_height = 512
filter = "smooth-lanczos3"
bit_depth = 8
[[assets.sound]]
source = "assets/sounds/ui/open.wav"
[assets.sound.transform]
sample_rate = 24000
channels = "mono"
[[assets.music]]
name = "theme"
source = "assets/music/theme.wav"
profile = "effect"
[assets.music.transform]
sample_rate = 48000
channels = "preserve"
[[assets.environment]]
name = "rain"
source = "assets/environment/rain.wav"
`
	if err := os.WriteFile(filepath.Join(directory, "karty.toml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := project.Load(directory)
	if err != nil {
		t.Fatal(err)
	}

	if len(config.Assets.Textures) != 3 {
		t.Fatalf("textures = %+v", config.Assets.Textures)
	}

	photo := config.Assets.Textures[0]
	for _, texture := range config.Assets.Textures {
		if texture.Source == "assets/textures/photo.jpg" {
			photo = texture
		}
	}

	if photo.Transform.MaxWidth != 1024 || photo.Transform.MaxHeight != 512 || photo.Transform.BitDepth != 8 ||
		photo.Transform.Filter != "smooth-lanczos3" {
		t.Fatalf("photo transform = %+v", photo.Transform)
	}

	if len(config.Assets.Sounds) != 1 || config.Assets.Sounds[0].Name != "ui.open" ||
		config.Assets.Sounds[0].Profile != project.DefaultSoundProfile ||
		config.Assets.Sounds[0].Transform.SampleRate != 24_000 || config.Assets.Sounds[0].Transform.Channels != "mono" {
		t.Fatalf("sounds = %+v", config.Assets.Sounds)
	}
	if len(config.Assets.Music) != 1 || config.Assets.Music[0].Name != "theme" || config.Assets.Music[0].Transform.SampleRate != 48_000 {
		t.Fatalf("music = %+v", config.Assets.Music)
	}
	if len(config.Assets.Environments) != 1 || config.Assets.Environments[0].Name != "rain" ||
		config.Assets.Environments[0].Profile != project.DefaultEnvironmentProfile {
		t.Fatalf("environments = %+v", config.Assets.Environments)
	}
}

func TestLoadDoesNotRediscoverDeclaredAudioStreamAsSound(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()

	path := filepath.Join(directory, "assets", "sounds", "music", "loop.wav")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("wave"), 0o600); err != nil {
		t.Fatal(err)
	}

	manifest := `[project]
name = "demo"
[sdk]
version = "0.0.7"
[[assets.music]]
name = "loop"
source = "assets/sounds/music/loop.wav"
`
	if err := os.WriteFile(filepath.Join(directory, "karty.toml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := project.Load(directory)
	if err != nil {
		t.Fatal(err)
	}

	if len(config.Assets.Sounds) != 0 || len(config.Assets.Music) != 1 {
		t.Fatalf("sounds=%+v music=%+v", config.Assets.Sounds, config.Assets.Music)
	}
}

func TestLoadRejectsInvalidAssetTransforms(t *testing.T) {
	t.Parallel()

	base := "[project]\nname='demo'\n[sdk]\nversion='0.0.7'\n"

	for name, declaration := range map[string]string{
		"texture": "[[assets.texture]]\nname='bad'\nsource='texture.png'\n[assets.texture.transform]\nfilter='magic'\n",
		"sound":   "[[assets.sound]]\nname='bad'\nsource='sound.wav'\n[assets.sound.transform]\nsample_rate=96000\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			for _, relative := range []string{"texture.png", "sound.wav"} {
				if err := os.WriteFile(filepath.Join(directory, relative), []byte("source"), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			if err := os.WriteFile(filepath.Join(directory, "karty.toml"), []byte(base+declaration), 0o600); err != nil {
				t.Fatal(err)
			}

			if _, err := project.Load(directory); err == nil {
				t.Fatal("Load() accepted an invalid transform")
			}
		})
	}
}

func TestLoadRejectsAssetSymlinkOutsideProject(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()

	outside := filepath.Join(t.TempDir(), "outside.png")
	if err := os.WriteFile(outside, []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(outside, filepath.Join(directory, "texture.png")); err != nil {
		t.Skipf("create symlink: %v", err)
	}

	manifest := `[project]
name = "demo"
[sdk]
version = "0.0.7"
[[assets.texture]]
name = "outside"
source = "texture.png"
`
	if err := os.WriteFile(filepath.Join(directory, "karty.toml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := project.Load(directory); err == nil || !strings.Contains(err.Error(), "project texture") {
		t.Fatalf("outside symlink error = %v", err)
	}
}

func TestLoadLayoutSources(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	if err := os.MkdirAll(filepath.Join(directory, "ui", "layouts"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(directory, "ui", "layouts", "window.kui"),
		[]byte(`<template><panel><slot/></panel></template>`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	manifest := `[project]
name = "demo"
[sdk]
version = "0.0.1"
[[assets.layout]]
source = "ui/layouts/window.kui"
`
	if err := os.WriteFile(filepath.Join(directory, "karty.toml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := project.Load(directory)
	if err != nil || len(config.Assets.Layouts) != 1 || config.Assets.Layouts[0].Source != "ui/layouts/window.kui" {
		t.Fatalf("layout config = %+v, %v", config.Assets.Layouts, err)
	}

	duplicated := manifest + `[[assets.layout]]
source = "ui/layouts/window.kui"
`
	if err := os.WriteFile(filepath.Join(directory, "karty.toml"), []byte(duplicated), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := project.Load(directory); err == nil || !strings.Contains(err.Error(), "project layout") {
		t.Fatalf("accepted duplicate layout: %v", err)
	}
}

func TestLoadDiscoversConventionalAssetsAndMergesSparseOverrides(t *testing.T) {
	t.Parallel()

	for _, extension := range []string{".kui"} {
		t.Run(extension, func(t *testing.T) {
			t.Parallel()
			checkConventionalAssetDiscovery(t, extension)
		})
	}
}

//nolint:wsl_v5 // Conventional UI discovery shares sparse override rules.
func checkConventionalAssetDiscovery(t *testing.T, extension string) {
	t.Helper()

	directory := t.TempDir()
	for _, path := range []string{
		"assets/textures/sprites/player.png", "ui/views/menu" + extension, "ui/components/item-row.kui",
		"ui/layouts/window" + extension, "ui/theme.toml",
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
