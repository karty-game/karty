package generator

import (
	"bytes"
	"errors"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
	"github.com/karty-game/karty-ui/codegen"
	uicompiler "github.com/karty-game/karty-ui/compiler"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/worldbuild/geometry"
	worldsource "github.com/karty-game/karty/internal/worldbuild/source"
	"github.com/karty-game/karty/website"
	"github.com/pelletier/go-toml/v2"
)

func TestGenerateDeterministicGuideAndLinks(t *testing.T) {
	t.Parallel()

	output := filepath.Join(t.TempDir(), "site", "guide")
	if err := Generate(output); err != nil {
		t.Fatal(err)
	}

	if err := GenerateDesign(filepath.Join(filepath.Dir(output), "design")); err != nil {
		t.Fatal(err)
	}

	if err := GenerateTechnical(filepath.Join(filepath.Dir(output), "technical")); err != nil {
		t.Fatal(err)
	}

	markdown := readGenerated(t, output, "guide.md")

	html := readGenerated(t, output, "complete.html")
	if err := Generate(output); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(markdown, readGenerated(t, output, "guide.md")) || !bytes.Equal(html, readGenerated(t, output, "complete.html")) {
		t.Fatal("repeated generation changed the guide")
	}

	checkGuideLinks(t, output, html)
	checkGuideImages(t, output, html)
	checkGuideExamples(t, markdown, html)

	if !bytes.Contains(html, []byte(website.CSS())) {
		t.Fatal("guide does not use the shared site theme")
	}

	if !bytes.Contains(html, []byte(`href="guide.md" download`)) || bytes.Count(html, []byte("<h2 ")) != 12 {
		t.Fatal("complete guide is missing a shared topic or game path")
	}

	if bytes.Count(html, []byte("<figure>")) != 8 || !bytes.Contains(html, []byte(`<figure class="journey"><svg`)) {
		t.Fatal("guide is missing an image or its offline journey diagram")
	}
}

func checkGuideLinks(t *testing.T, output string, html []byte) {
	t.Helper()

	identities := map[string]bool{}
	for _, match := range regexp.MustCompile(`id="([^"]+)"`).FindAllSubmatch(html, -1) {
		if identities[string(match[1])] {
			t.Fatalf("duplicate HTML identity %s", match[1])
		}

		identities[string(match[1])] = true
	}

	for _, match := range regexp.MustCompile(`href="([^"]+)"`).FindAllSubmatch(html, -1) {
		target := string(match[1])
		if strings.HasPrefix(target, "#") && !identities[strings.TrimPrefix(target, "#")] {
			t.Fatalf("broken chapter link %s", target)
		}

		file, fragment, _ := strings.Cut(target, "#")
		checkLinkedFragment(t, output, file, fragment)

		switch {
		case strings.HasPrefix(target, "../design/") || strings.HasPrefix(target, "../guide/") || strings.HasPrefix(target, "../technical/"):
			if _, err := os.Stat(filepath.Join(output, file)); err != nil {
				t.Fatalf("broken documentation section link %s: %v", target, err)
			}
		case strings.HasPrefix(target, "../") && target != "../":
			name := strings.TrimSuffix(strings.TrimPrefix(target, "../"), "/")
			for _, required := range []string{"README.md", "preview.toml", "preview.jpg"} {
				if _, err := os.Stat(filepath.Join("..", "..", "..", "samples", name, required)); err != nil {
					t.Fatalf("guide links to a sample outside the playable gallery %s: %v", target, err)
				}
			}
		case !strings.HasPrefix(target, "#") && !strings.Contains(target, ":") && target != "../":
			if _, err := os.Stat(filepath.Join(output, file)); err != nil {
				t.Fatalf("broken guide page link %s: %v", target, err)
			}
		}
	}
}

func checkLinkedFragment(t *testing.T, output, file, fragment string) {
	t.Helper()

	if file == "" || fragment == "" || strings.Contains(file, ":") || !strings.HasSuffix(file, ".html") {
		return
	}

	if !bytes.Contains(readGenerated(t, output, file), []byte(`id="`+fragment+`"`)) {
		t.Fatalf("broken documentation section link %s#%s", file, fragment)
	}
}

func checkGuideImages(t *testing.T, output string, html []byte) {
	t.Helper()

	for _, match := range regexp.MustCompile(`src="(assets/[^"]+)"`).FindAllSubmatch(html, -1) {
		name := string(match[1])

		asset, err := website.Files.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(asset, readGenerated(t, output, name)) {
			t.Fatalf("generated image differs from its source: %s", name)
		}
	}

	for _, name := range []string{"ui-demo", "world-camera", "hangar", "media-lab"} {
		original, err := os.ReadFile(filepath.Join("..", "..", "..", "samples", name, "preview.jpg"))
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(original, readGenerated(t, output, "assets/"+name+".jpg")) {
			t.Fatalf("guide sample image needs refreshing: %s", name)
		}
	}
}

func checkGuideExamples(t *testing.T, markdown, html []byte) {
	t.Helper()

	for _, language := range []string{"sh", "toml", "yaml", "go", "kui"} {
		for _, example := range codeBlocks(string(markdown), language) {
			if !bytes.Contains(html, []byte(escapeCode(example))) {
				t.Fatalf("HTML changed a %s example", language)
			}
		}
	}
}

func TestGeneratedPathsKeepUnrelatedTopicsSeparate(t *testing.T) {
	t.Parallel()

	output := filepath.Join(t.TempDir(), "guide")
	if err := Generate(output); err != nil {
		t.Fatal(err)
	}

	if err := GenerateDesign(filepath.Join(filepath.Dir(output), "design")); err != nil {
		t.Fatal(err)
	}

	if err := GenerateTechnical(filepath.Join(filepath.Dir(output), "technical")); err != nil {
		t.Fatal(err)
	}

	pages, err := guidePages()
	if err != nil {
		t.Fatal(err)
	}

	for _, page := range pages {
		html := readGenerated(t, output, page.name+".html")
		checkGuideLinks(t, output, html)
	}

	landing := string(readGenerated(t, output, "index.html"))
	if strings.Count(landing, `class="path"`) != 3 || strings.Contains(landing, "CameraSettings") {
		t.Fatal("landing does not offer three focused game paths")
	}

	flat := string(readGenerated(t, output, "2d.html"))
	if strings.Contains(flat, "CameraSettings") || strings.Contains(flat, `href="lighting.html"`) ||
		strings.Contains(flat, `href="levels.html"`) {
		t.Fatal("2D path includes unrelated world setup")
	}

	if !strings.Contains(flat, `href="ui.html"`) || !strings.Contains(flat, `href="media.html"`) ||
		!strings.Contains(flat, `href="share.html"`) {
		t.Fatal("2D path is missing relevant next steps")
	}

	for _, required := range []string{"NewSprite2D", "NewVector2D", "NewText2D", "assets/2d-scene.jpg"} {
		if !strings.Contains(flat, required) {
			t.Fatalf("2D path is missing drawing guidance: %s", required)
		}
	}

	if strings.Contains(flat, "assets/ui-demo.jpg") || !strings.Contains(landing, "assets/2d-scene.jpg") {
		t.Fatal("2D path still uses a menu screenshot")
	}

	for _, name := range []string{"2d", "fps", "isometric"} {
		html := string(readGenerated(t, output, name+".html"))
		if !strings.Contains(html, `aria-label="Shared topics"`) || !strings.Contains(html, `href="ui.html"`) {
			t.Fatalf("game path is missing the shared UI guide: %s", name)
		}
	}

	for _, name := range []string{"why", "2d", "fps", "isometric", "levels", "textures", "prefabs", "lighting", "interactions", "ui", "media", "share"} {
		html := readGenerated(t, output, name+".html")
		if !bytes.Contains(html, []byte(`href="`+name+`.md" download`)) {
			t.Fatalf("path/topic is missing its Markdown download: %s", name)
		}

		if len(readGenerated(t, output, name+".md")) == 0 {
			t.Fatalf("empty Markdown path: %s", name)
		}
	}
}

func TestGeneratePreservesUnrelatedFiles(t *testing.T) {
	t.Parallel()

	output := t.TempDir()

	path := filepath.Join(output, "notes.txt")
	if err := os.WriteFile(path, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Generate(output); err != nil {
		t.Fatal(err)
	}

	if got := string(readGenerated(t, output, "notes.txt")); got != "keep me" {
		t.Fatalf("changed unrelated file: %s", got)
	}
}

func TestGenerateRejectsLinkedDirectoriesAndFiles(t *testing.T) {
	t.Parallel()

	for _, target := range []string{"parent", "guide.md", "index.html", "fps.html", "isometric.md", ".guide.md.tmp", "assets", "assets/world-camera.jpg"} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()

			outside := filepath.Join(root, "outside")
			if err := os.Mkdir(outside, 0o755); err != nil {
				t.Fatal(err)
			}

			victim := filepath.Join(outside, "victim")
			if err := os.WriteFile(victim, []byte("untouched"), 0o600); err != nil {
				t.Fatal(err)
			}

			output := filepath.Join(root, "output")
			if err := os.Mkdir(output, 0o755); err != nil {
				t.Fatal(err)
			}

			link := filepath.Join(output, target)
			linkTarget := victim

			if target == "parent" {
				link, linkTarget = filepath.Join(root, "linked"), outside
				output = filepath.Join(link, "guide")
			} else if target == "assets" {
				linkTarget = outside
			} else if err := os.MkdirAll(filepath.Dir(link), 0o750); err != nil {
				t.Fatal(err)
			}

			if err := os.Symlink(linkTarget, link); err != nil {
				t.Fatal(err)
			}

			if err := Generate(output); err == nil {
				t.Fatal("accepted a linked output")
			}

			if got := string(readGenerated(t, outside, "victim")); got != "untouched" {
				t.Fatal("modified a linked file")
			}

			entries, err := os.ReadDir(outside)
			if err != nil || len(entries) != 1 {
				t.Fatalf("created files outside output: %v, %v", entries, err)
			}
		})
	}
}

func TestGenerateRejectsNonregularTargetsBeforeWriting(t *testing.T) {
	t.Parallel()

	output := t.TempDir()
	if err := os.Mkdir(filepath.Join(output, "index.html"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Generate(output); !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("expected invalid target: %v", err)
	}

	if _, err := os.Stat(filepath.Join(output, "guide.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("published Markdown before checking HTML target: %v", err)
	}

	if err := Generate(""); !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("expected missing output error: %v", err)
	}
}

func TestJourneyExamplesCompileWorldAndLightmap(t *testing.T) {
	t.Parallel()

	var chapterSource strings.Builder

	for _, name := range []string{"02-level.md", "03-texture.md", "04-prefab.md", "05-light.md"} {
		chapter, err := website.Files.ReadFile("content/guides/chapters/" + name)
		if err != nil {
			t.Fatal(err)
		}

		chapterSource.Write(chapter)
		chapterSource.WriteByte('\n')
	}

	markdown := chapterSource.String()

	expanded, err := worldsource.Decode([]byte(strings.Join(codeBlocks(markdown, "yaml"), "\n")))
	if err != nil {
		t.Fatalf("guide world examples: %v", err)
	}

	var manifest struct {
		Level    struct{ Name, Kind string }
		World    struct{ Source string }
		Textures []struct {
			Name, Source, Profile string
			Materialize           struct{ Preset string }
		}
		Lightmap struct {
			Enabled, Offline bool
			Density          float64
			PageSize         int `toml:"page_size"`
		}
	}
	if err := toml.Unmarshal([]byte(strings.Join(codeBlocks(markdown, "toml"), "\n")), &manifest); err != nil {
		t.Fatalf("guide level examples: %v", err)
	}

	if manifest.Level.Name != "levels.room" || manifest.Level.Kind != "world" || manifest.World.Source != "world.yaml" {
		t.Fatalf("incomplete level manifest: %+v", manifest)
	}

	materials := map[string]uint32{}
	for index, texture := range manifest.Textures {
		materials[texture.Name] = uint32(index + 1)
		if texture.Source != "surface.png" || texture.Profile != "environment" {
			t.Fatalf("incomplete texture: %+v", texture)
		}

		if err := (assetpipeline.MaterializeOptions{Preset: texture.Materialize.Preset}).Validate(); err != nil {
			t.Fatalf("guide material preset: %v", err)
		}
	}

	document, err := geometry.Compile(expanded, materials)
	if err != nil {
		t.Fatalf("compile guide world: %v", err)
	}

	if _, err := world.Encode(document); err != nil {
		t.Fatal(err)
	}

	if len(document.Sectors) != 1 || len(document.StaticSolids.Items) != 1 || document.StaticSolids.Items[0].ID != "crate-a/body" {
		t.Fatalf("guide room/prefab did not compile as described: %+v", document)
	}

	if !manifest.Lightmap.Enabled || !manifest.Lightmap.Offline {
		t.Fatal("guide lightmap is not an offline recipe")
	}

	if _, err := worldlightmap.Compile(document, worldlightmap.Options{
		TexelsPerUnit: manifest.Lightmap.Density, PageSize: manifest.Lightmap.PageSize, AllStatic: true,
	}); err != nil {
		t.Fatalf("compile guide lightmap: %v", err)
	}
}

func TestJourneyGoExamplesTypeCheckAgainstSelectedSDK(t *testing.T) {
	t.Parallel()

	markdown, err := readMarkdown()
	if err != nil {
		t.Fatal(err)
	}

	examples := codeBlocks(markdown, "go")
	if len(examples) != 13 {
		t.Fatalf("expected viewer, interaction, media and UI examples: %d", len(examples))
	}

	viewer := examples[0]
	interaction := strings.Replace(viewer, "type Game struct {", "type Game struct {\n"+examples[1], 1)
	interaction = strings.Replace(interaction, "engine.Run(engine.Hooks{", "engine.Run(engine.Hooks{\nOnKeyDown: game.onKeyDown,", 1)
	interaction = strings.Replace(interaction, "game.camera.SetLayer(-1)", "game.camera.SetLayer(-1)\ngame.subscribeGoal()", 1)
	interaction += "\nfunc (game *Game) subscribeGoal() {\n" + examples[2] + "\n}\n" + examples[3]

	mediaMarkdown, err := website.Files.ReadFile("content/guides/chapters/07-media.md")
	if err != nil {
		t.Fatal(err)
	}

	mediaExamples := codeBlocks(string(mediaMarkdown), "go")
	media := strings.Replace(
		viewer,
		`import "example.com/my-game/.karty/engine"`,
		"import (\"math\"; \"example.com/my-game/.karty/engine\"; \"example.com/my-game/.karty/assets\")",
		1,
	) + "\n" + strings.Join(mediaExamples, "\n")
	menu := strings.Replace(viewer, `import "example.com/my-game/.karty/engine"`,
		"import (\"example.com/my-game/.karty/engine\"; \"example.com/my-game/.karty/ui\")", 1)
	menu = strings.Replace(menu, "type Game struct {", "type Game struct {\nscreen *engine.UI", 1) + "\n" + examples[4+len(mediaExamples)]

	manifest, err := sdk.Resolve(release.SDKVersion())
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
	packageImporter.packages[enginePath] = checkExample(t, &settings, fileSet, enginePath, bindings)
	assetPath := "example.com/my-game/.karty/assets"
	packageImporter.packages[assetPath] = checkExample(
		t,
		&settings,
		fileSet,
		assetPath,
		map[string][]byte{"media.go": []byte(`package assets
import "example.com/my-game/.karty/engine"
const SoundUiClick engine.SoundID = 1
const SoundDoorOpen engine.SoundID = 2
const SoundReactorHum engine.SoundID = 3
const SoundCrateScrape engine.SoundID = 4
const MusicTheme engine.MusicID = 1
const VideoIntro engine.VideoID = 1
const TexturePlayer engine.TextureID = "player"
`)},
	)
	uiPath := "example.com/my-game/.karty/ui"
	packageImporter.packages[uiPath] = checkExample(t, &settings, fileSet, uiPath, compileUIExamples(t, t.TempDir(), markdown))

	for name, text := range map[string]string{"viewer": viewer, "interaction": interaction, "media": media, "menu": menu} {
		checkExample(t, &settings, fileSet, "example.com/my-game/"+name, map[string][]byte{"main.go": []byte(text)})
	}

	controllerSources := map[string][]byte{}

	entries, err := os.ReadDir("../../../packages/controllers")
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		data, err := os.ReadFile(filepath.Join("../../../packages/controllers", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}

		controllerSources[entry.Name()] = data
	}

	packageImporter.packages["github.com/karty-game/controllers"] = checkExample(
		t,
		&settings,
		fileSet,
		"github.com/karty-game/controllers",
		controllerSources,
	)
	checkTrackExamples(t, &settings, fileSet)
}

func checkTrackExamples(t *testing.T, settings *types.Config, fileSet *token.FileSet) {
	t.Helper()

	flatMarkdown, err := website.Files.ReadFile("content/guides/tracks/2d.md")
	if err != nil {
		t.Fatal(err)
	}

	flatBlocks := codeBlocks(string(flatMarkdown), "go")
	if len(flatBlocks) != 1 {
		t.Fatalf("expected one complete 2D scene: %d", len(flatBlocks))
	}

	checkExample(t, settings, fileSet, "example.com/my-game/flat", map[string][]byte{"main.go": []byte(flatBlocks[0])})

	for _, name := range []string{"fps", "isometric"} {
		markdown, err := website.Files.ReadFile("content/guides/tracks/" + name + ".md")
		if err != nil {
			t.Fatal(err)
		}

		blocks := codeBlocks(string(markdown), "go")
		if len(blocks) != 1 {
			t.Fatalf("expected one complete preset adapter in %s: %d", name, len(blocks))
		}

		text := blocks[0]
		checkExample(t, settings, fileSet, "example.com/my-game/"+name, map[string][]byte{"main.go": []byte(text)})
	}
}

type guideImporter struct {
	packages map[string]*types.Package
	standard types.Importer
}

func (source guideImporter) Import(path string) (*types.Package, error) {
	if compiled, exists := source.packages[path]; exists {
		return compiled, nil
	}

	return source.standard.Import(path)
}

func checkExample(t *testing.T, settings *types.Config, fileSet *token.FileSet, path string, sourceFiles map[string][]byte) *types.Package {
	t.Helper()

	files := make([]*ast.File, 0, len(sourceFiles))
	for _, name := range slices.Sorted(maps.Keys(sourceFiles)) {
		parsed, err := parser.ParseFile(fileSet, name, sourceFiles[name], parser.AllErrors)
		if err != nil {
			t.Fatal(err)
		}

		files = append(files, parsed)
	}

	compiled, err := settings.Check(path, fileSet, files, nil)
	if err != nil {
		t.Fatalf("type-check guide example %s: %v", path, err)
	}

	return compiled
}

func compileUIExamples(t *testing.T, directory, markdown string) map[string][]byte {
	t.Helper()

	examples := codeBlocks(markdown, "kui")
	if len(examples) != 3 {
		t.Fatalf("expected layout, menu and pause components: %d", len(examples))
	}

	for index, name := range []string{"ui/layouts/screen-frame.kui", "ui/views/menu.kui", "ui/views/pause.kui"} {
		writeExample(t, directory, name, []byte(examples[index]))
	}

	views, err := uicompiler.LoadProjectWithTheme(directory,
		[]uicompiler.Source{{Name: "ui.menu", Source: "ui/views/menu.kui"}, {Name: "ui.pause", Source: "ui/views/pause.kui"}},
		[]uicompiler.LayoutSource{{Source: "ui/layouts/screen-frame.kui"}}, "")
	if err != nil {
		t.Fatalf("guide KartUI examples: %v", err)
	}

	generated, err := codegen.UIPackageFiles(views, "example.com/my-game")
	if err != nil {
		t.Fatal(err)
	}

	for _, view := range views {
		if warnings := view.StyleWarnings(); len(warnings) != 0 {
			t.Fatalf("guide KartUI style warnings: %v", warnings)
		}
	}

	return generated
}

func writeExample(t *testing.T, directory, name string, contents []byte) {
	t.Helper()

	path := filepath.Join(directory, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readGenerated(t *testing.T, output, name string) []byte {
	t.Helper()

	contents, err := os.ReadFile(filepath.Join(output, name))
	if err != nil {
		t.Fatal(err)
	}

	return contents
}

func codeBlocks(markdown, language string) []string {
	pattern := regexp.MustCompile("(?s)```" + language + "\\n(.*?)\\n```")
	matches := pattern.FindAllStringSubmatch(markdown, -1)
	examples := make([]string, 0, len(matches))

	for _, match := range matches {
		examples = append(examples, match[1])
	}

	return examples
}

func escapeCode(text string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;", "'", "&#39;").Replace(text)
}
