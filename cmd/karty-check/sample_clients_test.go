package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty-ui/codegen"
	uicompiler "github.com/karty-game/karty-ui/compiler"
	"github.com/karty-game/karty/internal/project"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

func currentSamples() []string {
	return []string{"ui-demo", "media-lab", "world-camera", "hangar", "environment"}
}

// Read source and metadata only; never build a sample's assets or run its baker.
func TestSampleUISources(t *testing.T) {
	t.Parallel()

	for _, name := range currentSamples() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			checkSampleUISources(t, name)
		})
	}
}

func checkSampleUISources(t *testing.T, name string) {
	t.Helper()

	directory, config, views := sampleUISources(t, name)
	if config.SDK.Version != release.SampleSDKVersion(name) {
		t.Fatal("sample is not pinned to current SDK")
	}

	for _, view := range views {
		if filepath.Ext(view.Source) != ".kui" {
			t.Fatal("legacy component", view.Source)
		}

		if warnings := view.Diagnostics(); len(warnings) != 0 {
			t.Fatalf("%s: %+v", view.Source, warnings)
		}

		if view.Template.Version > 11 {
			t.Fatal("unsupported UI schema")
		}
	}

	sources, err := filepath.Glob(filepath.Join(directory, "levels", "*", "*.kui"))
	if err != nil {
		t.Fatal(err)
	}

	for _, source := range sources {
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := uicompiler.Compile(source, data); err != nil {
			t.Fatal(err)
		}
	}
}

// Type-check actual clients against their pinned SDK and exercise the UI
// demo through generated callbacks with crafted input events, without resources.
func TestSampleClients(t *testing.T) {
	t.Parallel()

	for _, name := range currentSamples() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			version := release.SampleSDKVersion(name)

			manifest, err := sdk.Resolve(version)
			if errors.Is(err, os.ErrNotExist) && version != release.SDKVersion() {
				t.Skip("install candidate SDK " + version + " to qualify this sample client; public checks never download it")
			}

			if err != nil {
				t.Fatal(err)
			}

			bindings, err := sdk.ClientFiles(manifest, "tinygo")
			if err != nil {
				t.Fatal(err)
			}

			directory := prepareSampleClient(t, name, manifest, bindings)

			packages := []string{"./src"}
			if name == "hangar" || name == "environment" {
				packages = append(packages, "./internal/fps")
			}

			// #nosec G204 -- executable and package list are fixed fixture commands, independent of project input.
			command := exec.CommandContext(t.Context(), "go", append([]string{"test", "-timeout=9s", "-count=1", "-v"}, packages...)...)
			command.Dir = directory

			command.Env = append(os.Environ(), "GOWORK=off")

			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("client source fixture: %v\n%s", err, output)
			}

			t.Logf("native source fixture:\n%s", output)
		})
	}
}

func prepareSampleClient(t *testing.T, name string, manifest sdk.Manifest, bindings map[string][]byte) string {
	t.Helper()
	source, config, views := sampleUISources(t, name)
	directory := t.TempDir()

	module, err := project.ModulePath(source)
	if err != nil {
		t.Fatal(err)
	}

	for file, contents := range bindings {
		native, err := componentNativeImports(contents)
		if err != nil {
			t.Fatal(err)
		}

		writeComponentFixture(t, directory, filepath.Join(".karty", file), native)
	}

	writeComponentFixture(t, directory, "go.mod", []byte("module "+module+"\n\ngo 1.27.0\n"))
	writeComponentFixture(
		t,
		directory,
		".karty/engine/fixture.go",
		[]byte(componentEngineFixture+"\nfunc FixtureUIID()uint32{if activeUI==nil{return 0};return activeUI.id}\n"),
	)

	if name != "world-camera" {
		writeComponentFixture(t, directory, ".karty/engine/hook_fixture.go", []byte(cameraHookEngineFixture))
	}

	writeComponentFixture(t, directory, ".karty/config/project.go", []byte(componentConfigFixture))

	entries, err := os.ReadDir(filepath.Join(source, "src"))
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".go") {
			copyComponentFixture(t, filepath.Join(source, "src", entry.Name()), directory, filepath.Join("src", entry.Name()))
		}
	}

	files, err := codegen.UIPackageFiles(views, module)
	if err != nil {
		t.Fatal(err)
	}

	for file, contents := range files {
		writeComponentFixture(t, directory, filepath.Join(".karty/ui", filepath.Base(file)+".go"), contents)
	}

	textures := make([]string, 0, len(config.Assets.Textures))
	for _, texture := range config.Assets.Textures {
		textures = append(textures, texture.Name)
	}

	textureSource, err := codegen.TextureAssetPackageFile(textures, module+"/.karty/engine")
	if err != nil {
		t.Fatal(err)
	}

	writeComponentFixture(t, directory, ".karty/assets/textures.go", textureSource)

	switch name {
	case "environment":
		files, err := filepath.Glob(filepath.Join(source, "internal/fps/*.go"))
		if err != nil {
			t.Fatal(err)
		}

		for _, file := range files {
			copyComponentFixture(t, file, directory, filepath.Join("internal/fps", filepath.Base(file)))
		}
	case "ui-demo":
		prepareFlightTests(t, directory, views)
	case "world-camera":
		prepareCameraHooksFixture(t, directory, source, manifest)
		copyComponentFixture(t, "testdata/world-camera-components_test.go.tmpl", directory, "src/components_test.go")
	case "media-lab":
		writeComponentFixture(t, directory, ".karty/assets/media.go", []byte(`package assets
import "example.com/media-lab/.karty/engine"
const (SoundEffectsClick engine.SoundID=1; SoundEffectsMachineGun engine.SoundID=2; MusicLabLoop engine.MusicID=1;VideoDemoPattern engine.VideoID=1)
`))
	case "hangar":
		writeComponentFixture(t, directory, ".karty/assets/sounds.go", []byte(`package assets
import "example.com/hangar/.karty/engine"
const (SoundReactorHum engine.SoundID=1; SoundComputerIdle engine.SoundID=2; SoundCrateScrape engine.SoundID=3; SoundFireAmbience engine.SoundID=4; SoundAcidAmbience engine.SoundID=5)
`))

		files, err := filepath.Glob(filepath.Join(source, "internal/fps/*.go"))
		if err != nil {
			t.Fatal(err)
		}

		for _, file := range files {
			copyComponentFixture(t, file, directory, filepath.Join("internal/fps", filepath.Base(file)))
		}

		copyComponentFixture(t, "testdata/hangar-lights_test.go.tmpl", directory, "src/lights_test.go")
		copyComponentFixture(t, "testdata/hangar-audio_test.go.tmpl", directory, "src/audio_test.go")

		prepareHangarLightControls(t, directory, views)
	}

	return directory
}

func sampleUISources(t *testing.T, name string) (string, project.Config, []uicompiler.Component) {
	t.Helper()

	directory := filepath.Join("..", "..", "samples", name)

	config, err := project.Load(directory)
	if err != nil {
		t.Fatal(err)
	}

	views, err := project.CompileUI(directory, config.Assets.UI, config.Assets.Layouts, config.Assets.Theme.Source)
	if err != nil {
		t.Fatal(err)
	}

	return directory, config, views
}

func prepareFlightTests(t *testing.T, directory string, views []uicompiler.Component) {
	t.Helper()

	var identifiers strings.Builder
	identifiers.WriteString("package config\nconst (\n")

	found := 0

	for _, view := range views {
		if view.Name != "GameScreen" {
			continue
		}

		for _, element := range view.Template.Elements {
			name := map[string]string{"tabs": "FlightTabs", "combo": "FlightShip", "input": "FlightName", "slider": "FlightPower", "checkbox": "FlightAutopilot"}[element.Kind]
			if name != "" {
				fmt.Fprintf(&identifiers, "%s = %d\n", name, element.ID)

				found++
			}
		}
	}

	if found != 5 {
		t.Fatalf("flight console must showcase five widget kinds; got %d", found)
	}

	identifiers.WriteString(")\n")
	writeComponentFixture(t, directory, ".karty/config/flight.go", []byte(identifiers.String()))
	copyComponentFixture(t, "testdata/flight-console_test.go.tmpl", directory, "src/flight_test.go")
}

func prepareHangarLightControls(t *testing.T, directory string, views []uicompiler.Component) {
	t.Helper()

	var identifiers strings.Builder
	identifiers.WriteString("package config\nconst (\n")

	found := 0

	for _, view := range views {
		for _, binding := range view.Bindings {
			if binding.Click == "props.LaunchLight" {
				t.Fatal("hangar point-light demo must remain disabled")
			}

			name := map[string]string{"props.ToggleTorch": "TorchButton"}[binding.Click]
			if name != "" {
				fmt.Fprintf(&identifiers, "%s = %d\n", name, binding.ID)

				found++
			}
		}

		for _, element := range view.Template.Elements {
			if element.Text == "Light bolt" {
				t.Fatal("hangar must not show the paused point-light demo")
			}
		}
	}

	if found != 1 {
		t.Fatal("hangar must expose the torch control")
	}

	identifiers.WriteString(")\n")
	writeComponentFixture(t, directory, ".karty/config/lights.go", []byte(identifiers.String()))
}
