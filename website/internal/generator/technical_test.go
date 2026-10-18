package generator

import (
	"bytes"
	"go/importer"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldparticles"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/worldbuild/geometry"
	worldsource "github.com/karty-game/karty/internal/worldbuild/source"
	"github.com/karty-game/karty/website"
)

func TestTechnicalPagesAndDownloads(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for name, generate := range map[string]func(string) error{
		"guide": Generate, "design": GenerateDesign, "technical": GenerateTechnical,
	} {
		if err := generate(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}

	output := filepath.Join(root, "technical")

	pages, err := technicalPages()
	if err != nil {
		t.Fatal(err)
	}

	before := map[string][]byte{}

	for _, page := range pages {
		html := readGenerated(t, output, page.name+".html")
		checkGuideLinks(t, output, html)
		checkGuideImages(t, output, html)

		if !bytes.Contains(html, []byte(`>API</a`)) ||
			!bytes.Contains(html, []byte(`aria-label="API topics"`)) ||
			!bytes.Contains(html, []byte(`href="`+page.name+`.md" download`)) ||
			bytes.Contains(html, []byte(`aria-label="Shared topics"`)) {
			t.Fatalf("incorrect Technical navigation or download: %s", page.name)
		}

		if page.redirect != "" && !bytes.Contains(html, []byte(`content="0; url=`+page.redirect+`"`)) {
			t.Fatalf("missing API redirect: %s", page.name)
		}

		before[page.name+".html"] = html
		before[page.name+".md"] = readGenerated(t, output, page.name+".md")
	}

	if err := GenerateTechnical(output); err != nil {
		t.Fatal(err)
	}

	for name, want := range before {
		if !bytes.Equal(want, readGenerated(t, output, name)) {
			t.Fatalf("Technical generation changed between runs: %s", name)
		}
	}
}

//nolint:paralleltest // Examples share a Go type importer and package cache.
func TestTechnicalExamples(t *testing.T) {
	version := os.Getenv("KARTY_TECHNICAL_SDK")
	if version == "" {
		version = release.SDKVersion()
	}

	manifest, err := sdk.Resolve(version)
	if err != nil {
		t.Fatal(err)
	}

	bindings, err := sdk.ClientFiles(manifest, "go")
	if err != nil {
		t.Fatal(err)
	}

	fileSet := token.NewFileSet()
	packageImporter := guideImporter{packages: map[string]*types.Package{}, standard: importer.Default()}
	settings := types.Config{Importer: packageImporter, Sizes: types.SizesFor("gc", "wasm")}
	enginePath := "example.com/my-game/.karty/engine"
	enginePackage := checkExample(t, &settings, fileSet, enginePath, bindings)
	packageImporter.packages[enginePath] = enginePackage
	assetPath := "example.com/my-game/.karty/assets"
	packageImporter.packages[assetPath] = checkExample(t, &settings, fileSet, assetPath, map[string][]byte{
		"assets.go": []byte(`package assets
import "example.com/my-game/.karty/engine"
const TexturePlayer engine.TextureID = "player"
const SoundUiClick engine.SoundID = 1
const SoundReactorHum engine.SoundID = 2
const MusicTheme engine.MusicID = 1
const EnvironmentWind engine.EnvironmentID = 1
`),
	})

	pages, err := technicalPages()
	if err != nil {
		t.Fatal(err)
	}

	for _, page := range pages {
		t.Run(page.name, func(t *testing.T) {
			for index, example := range codeBlocks(page.markdown, "go") {
				t.Run(strconv.Itoa(index), func(t *testing.T) {
					if strings.Contains(example, "engine.Camera2DSettings") && enginePackage.Scope().Lookup("Camera2DSettings") == nil {
						if os.Getenv("KARTY_TECHNICAL_SDK") != "" {
							t.Fatal("explicit API SDK lacks Camera2DSettings")
						}

						t.Skip("selected SDK lacks Camera 2D; check-technical-candidate validates this snippet")
					}

					path := "example.com/my-game/technical/" + page.name + "/" + strconv.Itoa(index)
					// Reference snippets omit file boilerplate. Only imports are supplied;
					// every displayed function body is checked against the actual SDK.
					prefix := "package main\nimport \"" + enginePath + "\"\n"
					if strings.Contains(example, "assets.") {
						prefix += "import \"" + assetPath + "\"\n"
					}

					checkExample(t, &settings, fileSet, path, map[string][]byte{"main.go": []byte(prefix + example)})
				})
			}
		})
	}
}

func TestTechnicalParticleWorldCompiles(t *testing.T) {
	t.Parallel()

	markdown, err := website.Files.ReadFile("content/api/graphics.md")
	if err != nil {
		t.Fatal(err)
	}

	blocks := codeBlocks(string(markdown), "yaml")
	if len(blocks) != 1 {
		t.Fatal("particle topic must contain one particle section")
	}

	// Compile the section in a room fixture so schedules, texture references
	// and capacity still pass the normal world and particle validation.
	worldSource := `version: 19
rooms:
  - id: room
    rect: {width: 8, depth: 8, material: surface}
    floor: {c: 0}
    ceiling: {c: 3}
    floor_material: surface
    ceiling_material: surface
` + blocks[0]

	expanded, err := worldsource.Decode([]byte(worldSource))
	if err != nil {
		t.Fatal(err)
	}

	document, err := geometry.Compile(expanded, map[string]uint32{"surface": 1, "spark": 2})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := world.Encode(document); err != nil {
		t.Fatal(err)
	}

	particles, err := geometry.CompileParticles(expanded, &document, map[string]uint32{"surface": 1, "spark": 2})
	if err != nil {
		t.Fatal(err)
	}

	if particles == nil {
		t.Fatal("particle example did not compile an effect")
	}

	encoded, err := worldparticles.Encode(*particles)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := worldparticles.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	if err := worldparticles.ValidateForWorld(&decoded, &document); err != nil {
		t.Fatal(err)
	}
}

func TestTechnicalRejectsLinkedDestinations(t *testing.T) {
	t.Parallel()

	root, outside := t.TempDir(), t.TempDir()

	output := filepath.Join(root, "technical")
	if err := os.Mkdir(output, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(filepath.Join(outside, "untouched"), filepath.Join(output, "input.html")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	if err := GenerateTechnical(output); err == nil {
		t.Fatal("linked Technical page accepted")
	}

	if _, err := os.Stat(filepath.Join(output, "index.html")); !os.IsNotExist(err) {
		t.Fatalf("published pages before checking destinations: %v", err)
	}
}
