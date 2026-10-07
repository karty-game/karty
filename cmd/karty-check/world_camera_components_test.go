package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty-ui/codegen"
	"github.com/karty-game/karty-ui/compiler"
	"github.com/karty-game/karty/internal/actionbuild"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

// Compile only authored Go behavior with real public SDK bindings. No sample
// levels, textures, bake, graphical host or browser are loaded by this fixture.
func TestWorldCameraComponents(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	files, err := sdk.ClientFiles(manifest, "tinygo")
	if err != nil {
		t.Fatal(err)
	}

	directory := t.TempDir()

	for name, data := range files {
		native, err := componentNativeImports(data)
		if err != nil {
			t.Fatal(err)
		}

		writeComponentFixture(t, directory, filepath.Join(".karty", name), native)
	}

	writeComponentFixture(t, directory, "go.mod", []byte("module example.com/world-camera\n\ngo 1.27.0\n"))
	writeComponentFixture(t, directory, ".karty/engine/fixture.go", []byte(componentEngineFixture))
	writeComponentFixture(t, directory, ".karty/config/project.go", []byte(componentConfigFixture))

	sample := filepath.Join("..", "..", "samples", "world-camera")
	for _, name := range []string{"cameras.go", "hud.go", "actors.go"} {
		copyComponentFixture(t, filepath.Join(sample, "src", name), directory, filepath.Join("src", name))
	}

	copyComponentFixture(t, "testdata/world-camera-components_test.go.tmpl", directory, "src/components_test.go")

	prepareCameraHooksFixture(t, directory, sample, manifest)

	command := exec.CommandContext(t.Context(), "go", "test", "-timeout=9s", "-count=1", "-v", "./src")
	command.Dir = directory

	command.Env = append(os.Environ(), "GOWORK=off")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("camera component fixture: %v\n%s", err, output)
	} else {
		t.Logf("native component fixture:\n%s", output)
	}
}

func writeComponentFixture(t *testing.T, directory, name string, data []byte) {
	t.Helper()

	if !fs.ValidPath(filepath.ToSlash(name)) {
		t.Fatal("invalid component fixture path", name)
	}

	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	if err := root.MkdirAll(filepath.Dir(name), 0750); err != nil {
		t.Fatal(err)
	}

	if err := root.WriteFile(name, data, 0600); err != nil {
		t.Fatal(err)
	}
}
func copyComponentFixture(t *testing.T, source, directory, name string) {
	t.Helper()

	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}

	writeComponentFixture(t, directory, name, data)
}

// Replace only WASM host imports with a command capture/no-op transport in
// temporary test output. SDK components, codecs and hook dispatch are unchanged.
func componentNativeImports(contents []byte) ([]byte, error) {
	set := token.NewFileSet()

	file, err := parser.ParseFile(set, "binding.go", contents, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || !componentWASMImport(function.Doc) {
			continue
		}

		if function.Type.Results != nil {
			return nil, fmt.Errorf("unsupported fixture import %s: %w", function.Name.Name, os.ErrInvalid)
		}

		function.Doc = nil

		function.Body = &ast.BlockStmt{}
		if function.Name.Name == "hostSubmitCommands" {
			stub, err := parser.ParseFile(
				token.NewFileSet(),
				"stub.go",
				"package engine\nfunc capture(){ fixtureCommands=append(fixtureCommands[:0],commandBatchBuffer[:length]...) }",
				0,
			)
			if err != nil {
				return nil, err
			}

			stubFunction, valid := stub.Decls[0].(*ast.FuncDecl)
			if !valid {
				return nil, fmt.Errorf("invalid capture function: %w", os.ErrInvalid)
			}

			function.Body = stubFunction.Body
		}
	}

	comments := file.Comments[:0]
	for _, comment := range file.Comments {
		if !componentWASMImport(comment) {
			comments = append(comments, comment)
		}
	}

	file.Comments = comments

	var output bytes.Buffer
	if err := format.Node(&output, set, file); err != nil {
		return nil, err
	}

	return output.Bytes(), nil
}

func prepareCameraHooksFixture(t *testing.T, directory, sample string, manifest sdk.Manifest) {
	t.Helper()
	copyComponentFixture(t, filepath.Join(sample, "src/main.go"), directory, "src/main.go")
	copyComponentFixture(t, "testdata/world-camera-hooks_test.go.tmpl", directory, "src/hooks_test.go")

	contents, err := os.ReadFile(filepath.Join(sample, "ui/views/controls.kui"))
	if err != nil {
		t.Fatal(err)
	}

	view, err := uicompiler.Compile("ui/views/controls.kui", contents)
	if err != nil {
		t.Fatal(err)
	}

	view.Asset = "ui.controls"

	files, err := codegen.UIPackageFiles([]uicompiler.Component{view}, "example.com/world-camera")
	if err != nil {
		t.Fatal(err)
	}

	for name, data := range files {
		writeComponentFixture(t, directory, filepath.Join(".karty/ui", filepath.Base(name)+".go"), data)
	}

	copyComponentFixture(t, filepath.Join(sample, "levels/showcase/actions.json"), directory, "levels/showcase/actions.json")

	declarations, err := actionbuild.Discover(directory, "example.com/world-camera")
	if err != nil {
		t.Fatal(err)
	}

	resources, err := sdk.Resources(manifest)
	if err != nil {
		t.Fatal(err)
	}

	contract, err := fs.ReadFile(resources, "contracts/actions-v1.schema.json")
	if err != nil {
		t.Fatal(err)
	}

	result, err := actionbuild.Compile(
		directory,
		"example.com/world-camera",
		declarations,
		[]actionbuild.Level{
			{
				Name:      "levels.camera-showcase",
				Directory: filepath.Join(directory, "levels/showcase"),
				Actors:    map[string]bool{"hall/player-marker": true},
			},
		},
		contract,
	)
	if err != nil {
		t.Fatal(err)
	}

	writeComponentFixture(t, directory, "src/karty_actions.go", result.Source)
	writeComponentFixture(t, directory, ".karty/engine/hook_fixture.go", []byte(cameraHookEngineFixture))
}

const componentConfigFixture = `package config
const (CameraFOVY=1.0471976;CameraOrthoHeight=30;CameraNear=.05;CameraFar=80;ResolutionWidth=320;ResolutionHeight=180)
`
const componentEngineFixture = `package engine
var fixtureCommands []byte
func FixtureBegin(){beginCommands(1)}
func FixtureCommands()[]byte{finishCommands();return fixtureCommands}
`
const cameraHookEngineFixture = `package engine
func FixtureStart(){initialize()}
func FixtureFrame(frame Frame){beginCommands(frame.Number);client.Update(frame);flushUI();finishCommands()}
func FixtureStop(){shutdown()}
`

func componentWASMImport(group *ast.CommentGroup) bool {
	if group == nil {
		return false
	}

	for _, comment := range group.List {
		if strings.HasPrefix(comment.Text, "//go:wasmimport ") {
			return true
		}
	}

	return false
}
