package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"text/template"

	"github.com/karty-game/karty-ui/codegen"
	uicompiler "github.com/karty-game/karty-ui/compiler"
	localpackages "github.com/karty-game/karty/internal/packages"
	"github.com/karty-game/karty/internal/project"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/testpackages"
)

func currentSamples() []string {
	return []string{"ui-demo", "media-lab", "hangar", "terrain", "crate-room", "vigil", "effect-lab", "particle-lab"}
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
	if config.SDK.Version != release.SDKVersion() {
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

			version := release.SDKVersion()

			manifest, err := sdk.Resolve(version)
			if err != nil {
				t.Fatal(err)
			}

			bindings, err := sdk.ClientFiles(manifest, "tinygo")
			if err != nil {
				t.Fatal(err)
			}

			directory := prepareSampleClient(t, name, bindings, manifest.Compatibility.ProjectCodegen)

			packages := []string{"./src"}

			switch name {
			case "hangar":
				packages = append(packages, "./internal/elevator")
			case "terrain":
				packages = append(packages, "./internal/fps")
			}

			runSampleClient(t, directory, name, packages)
		})
	}
}

func runSampleClient(t *testing.T, directory, name string, packages []string) {
	t.Helper()

	if name != "crate-room" {
		runNativeFixture(t, directory, "go", append([]string{"test", "-timeout=9s", "-count=1", "-v"}, packages...))

		return
	}

	fixture := filepath.Join(directory, "sample-client.test.exe")
	runNativeFixture(t, directory, "go", []string{"test", "-c", "-o", fixture, "./src"})

	for _, pattern := range sampleClientRuns(t, directory) {
		runNativeFixture(t, directory, fixture, []string{"-test.timeout=9s", "-test.v", "-test.run", pattern})
	}
}

func runNativeFixture(t *testing.T, directory, executable string, arguments []string) {
	t.Helper()

	// #nosec G204 -- the executable is Go or its just-compiled maintained fixture binary.
	command := exec.CommandContext(t.Context(), executable, arguments...)
	command.Dir = directory

	command.Env = append(os.Environ(), "GOWORK=off")

	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("client source fixture: %v\n%s", err, output)
	}

	t.Logf("native source fixture:\n%s", output)
}

// Crate Room's scenarios each end the cartridge lifecycle. Give each a fresh
// native process, just as a host creates a fresh WASM instance after shutdown.
func sampleClientRuns(t *testing.T, directory string) []string {
	t.Helper()

	files, err := filepath.Glob(filepath.Join(directory, "src", "*_test.go"))
	if err != nil {
		t.Fatal(err)
	}

	var runs []string

	for _, name := range files {
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}

		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Recv == nil && strings.HasPrefix(function.Name.Name, "Test") {
				runs = append(runs, "^"+regexp.QuoteMeta(function.Name.Name)+"$")
			}
		}
	}

	if len(runs) == 0 {
		t.Fatal("cartridge fixture contains no test scenarios")
	}

	return runs
}

func prepareSampleClient(t *testing.T, name string, bindings map[string][]byte, version int) string {
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

	writeSampleModule(t, source, directory, module)

	writeComponentFixture(
		t,
		directory,
		".karty/engine/fixture.go",
		[]byte(componentEngineFixture+"\nfunc FixtureUIID()uint32{if activeUI==nil{return 0};return activeUI.id}\n"),
	)

	writeComponentFixture(t, directory, ".karty/engine/hook_fixture.go", []byte(cameraHookEngineFixture))

	switch name {
	case "hangar", "terrain":
		prepareFirstPersonBridge(t, directory, module)
		prepareFirstPersonTests(t, directory, module, name)
	case "crate-room", "effect-lab":
		prepareFirstPersonBridge(t, directory, module)
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

	files, err := codegen.UIPackageFilesForVersion(views, module, version)
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

	levelSource, err := project.LevelAssetPackageFile(t.Context(), source, module+"/.karty/engine")
	if err != nil {
		t.Fatal(err)
	}

	writeComponentFixture(t, directory, ".karty/assets/levels.go", levelSource)

	switch name {
	case "vigil":
		writeComponentFixture(t, directory, ".karty/assets/sounds.go", []byte(`package assets
import "example.com/vigil/.karty/engine"
const (SoundEmberCrackle engine.SoundID=1; SoundFireballCast engine.SoundID=2; SoundFireballImpact engine.SoundID=3; SoundShrineChime engine.SoundID=4; SoundStoneFootstep engine.SoundID=5; MusicChurchAmbience engine.MusicID=1)
`))

		files, err := filepath.Glob(filepath.Join(source, "internal/dungeon/*.go"))
		if err != nil {
			t.Fatal(err)
		}

		for _, file := range files {
			copyComponentFixture(t, file, directory, filepath.Join("internal/dungeon", filepath.Base(file)))
		}
	case "effect-lab":
		copyComponentFixture(t, "testdata/effect-lab-controls_test.go.tmpl", directory, "src/controls_test.go")
	case "particle-lab":
		copyComponentFixture(t, "testdata/particle-lab-lifecycle_test.go.tmpl", directory, "src/lifecycle_test.go")
	case "crate-room":
		copyComponentFixture(t, "testdata/crate-room-layout_test.go.tmpl", directory, "src/layout_test.go")
		writeComponentFixture(t, directory, ".karty/assets/sounds.go", []byte(`package assets
import "example.com/crate-room/.karty/engine"
const (SoundShoot engine.SoundID=1; SoundBlast engine.SoundID=2; SoundCrateHit engine.SoundID=3)
`))
	case "ui-demo":
		prepareFlightTests(t, directory, views)
	case "media-lab":
		writeComponentFixture(t, directory, ".karty/assets/media.go", []byte(`package assets
import "example.com/media-lab/.karty/engine"
const (SoundEffectsClick engine.SoundID=1; SoundEffectsMachineGun engine.SoundID=2; MusicLabLoop engine.MusicID=1;VideoDemoPattern engine.VideoID=1)
`))
	case "terrain":
		files, err := filepath.Glob(filepath.Join(source, "internal/fps/*.go"))
		if err != nil {
			t.Fatal(err)
		}

		for _, file := range files {
			copyComponentFixture(t, file, directory, filepath.Join("internal/fps", filepath.Base(file)))
		}

		copyComponentFixture(t, "testdata/terrain-swimming_test.go.tmpl", directory, "src/swimming_test.go")
	case "hangar":
		writeComponentFixture(t, directory, ".karty/assets/sounds.go", []byte(`package assets
import "example.com/hangar/.karty/engine"
const (SoundReactorHum engine.SoundID=1; SoundComputerIdle engine.SoundID=2; SoundCrateScrape engine.SoundID=3; SoundFireAmbience engine.SoundID=4; SoundAcidAmbience engine.SoundID=5)
`))

		files, err := filepath.Glob(filepath.Join(source, "internal/elevator/*.go"))
		if err != nil {
			t.Fatal(err)
		}

		for _, file := range files {
			copyComponentFixture(t, file, directory, filepath.Join("internal/elevator", filepath.Base(file)))
		}

		copyComponentFixture(t, "testdata/hangar-lights_test.go.tmpl", directory, "src/lights_test.go")
		copyComponentFixture(t, "testdata/hangar-audio_test.go.tmpl", directory, "src/audio_test.go")

		prepareHangarLightControls(t, directory, views)
	}

	return directory
}

// Copy published package sources into isolated fixture modules, retaining actual
// consumer type checks without depending on a game's generated cache.
func writeSampleModule(t *testing.T, source, directory, module string) {
	t.Helper()

	var moduleSource strings.Builder
	moduleSource.WriteString("module " + module + "\n\ngo 1.27.0\n")

	sources, _, err := localpackages.LoadSources(t.Context(), source)
	if err != nil {
		t.Fatal(err)
	}

	pins, err := testpackages.Pins()
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"controllers-3d", "controllers-topdown", "controllers-isometric"} {
		dependency, used := sources[name]
		if !used {
			continue
		}

		pin := pins[name]
		if dependency.Folder != "" || dependency.Archive != pin.Archive || dependency.SHA256 != pin.SHA256 {
			t.Fatalf("sample must pin the published %s archive and checksum: %+v", name, dependency)
		}

		packageSource, err := testpackages.Source(name)
		if err != nil {
			t.Fatal(err)
		}

		root, err := os.OpenRoot(packageSource)
		if err != nil {
			t.Fatal(err)
		}

		manifest, err := localpackages.ReadManifest(t.Context(), root)
		root.Close()

		if err != nil {
			t.Fatal(err)
		}

		fmt.Fprintf(&moduleSource, "\nrequire %s v%s\nreplace %s => ./%s\n", manifest.GoModule, manifest.Version, manifest.GoModule, name)

		entries, err := os.ReadDir(packageSource)
		if err != nil {
			t.Fatal(err)
		}

		for _, entry := range entries {
			if entry.Name() == "go.mod" || strings.HasSuffix(entry.Name(), ".go") {
				copyComponentFixture(t, filepath.Join(packageSource, entry.Name()), directory, filepath.Join(name, entry.Name()))
			}
		}
	}

	writeComponentFixture(t, directory, "go.mod", []byte(moduleSource.String()))
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

// Render the package's real SDK adapter from the checksum-pinned source fixture.
// This mirrors its build processor without invoking a compiler inside test timers.
func prepareFirstPersonBridge(t *testing.T, directory, module string) {
	t.Helper()

	source, err := testpackages.Source("controllers-3d")
	if err != nil {
		t.Fatal(err)
	}

	files, err := filepath.Glob(filepath.Join(source, "bridge/templates/*.go.tmpl"))
	if err != nil {
		t.Fatal(err)
	}

	if len(files) == 0 {
		t.Fatal("published controller package has no SDK adapter templates")
	}

	for _, file := range files {
		owned, err := template.ParseFiles(file)
		if err != nil {
			t.Fatal(err)
		}

		var output bytes.Buffer
		if err := owned.Execute(&output, struct{ Module string }{module}); err != nil {
			t.Fatal(err)
		}

		formatted, err := format.Source(output.Bytes())
		if err != nil {
			t.Fatal(err)
		}

		name := strings.TrimSuffix(filepath.Base(file), ".tmpl")
		writeComponentFixture(t, directory,
			filepath.Join(".karty/processed/code/controllers-3d.sdk/controller", name), formatted)
	}
}

// Exercise the package's input through its normal frame dispatch, while exposing
// only the private stick state needed to retain the samples' gesture assertions.
func prepareFirstPersonTests(t *testing.T, directory, module, name string) {
	t.Helper()

	source, err := os.ReadFile("testdata/first-person-input_test.go.tmpl")
	if err != nil {
		t.Fatal(err)
	}

	radius := "72"
	if name == "terrain" {
		radius = "96"
	}

	text := strings.ReplaceAll(string(source), "example.com/fps-fixture", module)
	text = strings.ReplaceAll(text, "__STICK_RADIUS__", radius)

	if name == "terrain" {
		// Retain Terrain's original float tolerance; Hangar checks its exact rim.
		text = strings.ReplaceAll(text, "got != StickRadius", "math.Abs(float64(got-StickRadius)) > .0001")
	}

	writeComponentFixture(t, directory, "src/input_test.go", []byte(text))
	writeComponentFixture(t, directory, ".karty/processed/code/controllers-3d.sdk/controller/fixture.go", []byte(`package controller
 type FixtureFirstPersonStickState struct{ ID uint32; X,Y,DX,DY float32 }
 func FixtureFirstPersonStick(c *FirstPersonController, side int) FixtureFirstPersonStickState {
  s := c.sticks[side]
  return FixtureFirstPersonStickState{s.id,s.x,s.y,s.dx,s.dy}
 }
 `))
}
