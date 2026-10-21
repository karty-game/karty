package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
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

func TestSampleNoticesArePreparedOncePerSDKBeforePackaging(t *testing.T) {
	t.Parallel()

	calls := 0

	notices, err := prepareSampleNotices([]string{"0.0.15", "0.0.15", "0.0.16"}, func(version, name string) ([]byte, error) {
		calls++

		return []byte(version + "/" + name), nil
	})
	if err != nil || calls != 4 {
		t.Fatalf("notice preparation made %d requests: %v", calls, err)
	}

	// Each sample receives complete local copies; packaging needs no download.
	for _, version := range []string{"0.0.15", "0.0.15", "0.0.16"} {
		directory := t.TempDir()
		if err := writeSampleNotices(directory, notices[version]); err != nil {
			t.Fatal(err)
		}

		for _, name := range []string{"RUNTIME_LICENSE.md", "SDK_LICENSE.md"} {
			contents, err := os.ReadFile(filepath.Join(directory, name))
			if err != nil || string(contents) != version+"/"+name {
				t.Fatalf("packaged notice %s: %q, %v", name, contents, err)
			}
		}
	}

	calls = 0

	notices, err = prepareSampleNotices([]string{"0.0.15", "0.0.16"}, func(string, string) ([]byte, error) {
		calls++

		return nil, os.ErrPermission
	})
	if notices != nil || calls != 1 || !errors.Is(err, os.ErrPermission) ||
		!strings.Contains(err.Error(), "0.0.15 notice RUNTIME_LICENSE.md") {
		t.Fatalf("partial notice preparation was accepted: %v, %d calls, %v", notices, calls, err)
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

func TestSampleBuildKeepsRepositoryAndGuestCompilersSeparate(t *testing.T) {
	t.Parallel()

	directory := filepath.Join(t.TempDir(), "samples", "particle-lab")
	writeFixture(t, filepath.Join(directory, "levels", "lab", "level.toml"), "[lightmap]\nenabled = true\noffline = true\n")

	var commands []string

	run := sampleRunner("fixture-karty", "guest-go", func(cwd string, environment []string, executable string, args ...string) error {
		commands = append(commands, executable)
		if executable == "mise" {
			if cwd != filepath.Dir(filepath.Dir(directory)) || len(environment) != 0 ||
				!slices.Equal(args, []string{"run", "generate-particle-lab"}) {
				t.Fatalf("repository generation inherited guest compiler: %q %q %q", cwd, environment, args)
			}
		} else if cwd != directory || !slices.Equal(environment, []string{"KARTY_GO=guest-go"}) {
			t.Fatalf("sample command missing guest compiler: %q %q", cwd, environment)
		}

		return nil
	})
	if err := buildSampleWithRunner(directory, "fixture-karty", "", run); err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(commands, []string{"mise", "fixture-karty", "fixture-karty"}) {
		t.Fatalf("generation, bake and build commands: %q", commands)
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
