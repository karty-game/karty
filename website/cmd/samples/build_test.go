package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/karty-game/karty/internal/release"
)

func TestSampleBakeLevelsSelectConfiguredOfflineRecipes(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	for name, settings := range map[string]string{
		"offline-z": "enabled = true\noffline = true\n",
		"offline-a": "enabled = true\noffline = true\n",
		"runtime":   "enabled = true\n",
		"disabled":  "enabled = false\noffline = true\n",
		"unlit":     "",
		"chunked":   "enabled = true\noffline = true\n[chunks]\nenabled = true\n",
	} {
		writeFixture(t, filepath.Join(directory, "levels", name, "level.toml"), "[lightmap]\n"+settings)
	}

	writeFixture(t, filepath.Join(directory, "levels", "artwork", "texture.png"), "not a level")

	names, err := sampleBakeLevels(directory)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(names, []string{"offline-a", "offline-z"}) {
		t.Fatalf("offline recipes: %v", names)
	}

	names, err = sampleBakeLevels(t.TempDir())
	if err != nil || len(names) != 0 {
		t.Fatalf("project without levels: %v, %v", names, err)
	}
}

func TestSampleBuildBakesBeforeWebPackaging(t *testing.T) {
	t.Parallel()

	for _, scenario := range []struct {
		name    string
		levels  []string
		want    [][]string
		failure []string
		cdnURL  string
	}{
		{name: "offline", levels: []string{"a", "z"}, want: [][]string{{"render", "bake", "--incremental", "--level", "a"}, {"render", "bake", "--incremental", "--level", "z"}, {"build", "--target", "web"}}},
		{name: "no bake", want: [][]string{{"build", "--target", "web"}}},
		{name: "CDN", cdnURL: "../runtime/" + release.SDKVersion() + "/host.wasm", levels: []string{"a"}, want: [][]string{{"render", "bake", "--incremental", "--level", "a"}, {"build", "--target", "web", "--cdn", "../runtime/" + release.SDKVersion() + "/host.wasm"}}},
		{name: "failed bake", levels: []string{"broken", "z"}, want: [][]string{{"render", "bake", "--incremental", "--level", "broken"}}, failure: []string{"render", "bake", "--incremental", "--level", "broken"}},
		{name: "failed build", levels: []string{"a"}, want: [][]string{{"render", "bake", "--incremental", "--level", "a"}, {"build", "--target", "web"}}, failure: []string{"build", "--target", "web"}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			for _, name := range scenario.levels {
				writeFixture(t, filepath.Join(directory, "levels", name, "level.toml"), "[lightmap]\nenabled = true\noffline = true\n")
			}

			var commands [][]string

			failure := os.ErrPermission

			run := func(cwd, executable string, args ...string) error {
				if cwd != directory || executable != "fixture-karty" {
					t.Fatalf("command location = %q, %q", cwd, executable)
				}

				commands = append(commands, slices.Clone(args))

				if slices.Equal(args, scenario.failure) {
					return failure
				}

				return nil
			}

			err := buildSampleWithRunner(directory, "fixture-karty", scenario.cdnURL, run)
			if scenario.failure == nil && err != nil || scenario.failure != nil && !errors.Is(err, failure) {
				t.Fatalf("build result = %v, expected failure = %v", err, scenario.failure)
			}

			if !slices.EqualFunc(commands, scenario.want, slices.Equal[[]string]) {
				t.Fatalf("command order: %q, want %q", commands, scenario.want)
			}
		})
	}
}

func TestSampleBakeLevelsRejectInvalidManifest(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	writeFixture(t, filepath.Join(directory, "levels", "broken", "level.toml"), "[lightmap\n")

	if _, err := sampleBakeLevels(directory); err == nil {
		t.Fatal("invalid level manifest accepted")
	}
}

func TestSampleBakeLevelsRejectSymlinkEscapes(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("symlink fixtures require permission on Windows")
	}

	for _, scenario := range []string{"levels directory", "level manifest"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()

			directory, outside := t.TempDir(), t.TempDir()
			manifest := filepath.Join(outside, "outside", "level.toml")
			writeFixture(t, manifest, "[lightmap]\nenabled = true\noffline = true\n")

			link, target := filepath.Join(directory, "levels"), outside
			if scenario == "level manifest" {
				link = filepath.Join(directory, "levels", "offline", "level.toml")
				target = manifest

				if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
					t.Fatal(err)
				}
			}

			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}

			if _, err := sampleBakeLevels(directory); !errors.Is(err, os.ErrInvalid) {
				t.Fatalf("symlink escape accepted: %v", err)
			}
		})
	}
}

func TestSampleBakeLevelsMatchCurrentProjects(t *testing.T) {
	t.Parallel()

	for name, want := range map[string][]string{
		"hangar":       {"hangar"},
		"crate-room":   {"room"},
		"ui-demo":      nil,
		"media-lab":    nil,
		"particle-lab": nil,
	} {
		names, err := sampleBakeLevels(filepath.Join("..", "..", "..", "samples", name))
		if err != nil || !slices.Equal(names, want) {
			t.Fatalf("%s offline recipes: %v, %v", name, names, err)
		}
	}
}
