package assetusage_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/assetusage"
)

func TestAnalyzeFindsTypedReferenceThroughImportAlias(t *testing.T) {
	t.Parallel()

	directory := usageProject(t, `package main
import k "example.com/game/.karty/assets"
func main() { var _ = k.TextureSpritesPlayer }
`)

	result := assetusage.Analyze(directory, "example.com/game", []string{"sprites.player", "sprites.unused"})
	if result.KeepAll || result.Live["sprites.player"] != assetusage.ReasonTypedReference {
		t.Fatalf("Analyze() = %+v", result)
	}

	if _, exists := result.Live["sprites.unused"]; exists {
		t.Fatalf("unused texture marked live: %+v", result)
	}
}

func TestAnalyzeFindsStaticLogicalName(t *testing.T) {
	t.Parallel()

	directory := usageProject(t, `package main
import "example.com/game/.karty/engine"
func main() { new(engine.Game).NewSprite2D("sprites.player", 0) }
`)

	result := assetusage.Analyze(directory, "example.com/game", []string{"sprites.player", "sprites.unused"})
	if result.KeepAll || result.Live["sprites.player"] != assetusage.ReasonStaticName {
		t.Fatalf("Analyze() = %+v", result)
	}
}

func TestAnalyzeRetainsScopeForDynamicLookup(t *testing.T) {
	t.Parallel()

	directory := usageProject(t, `package main
import "example.com/game/.karty/engine"
func spawn(name string) { new(engine.Game).NewSprite2D(engine.DynamicTexture(name), 0) }
func main() { spawn("sprites.player") }
`)

	result := assetusage.Analyze(directory, "example.com/game", []string{"sprites.player", "sprites.unused"})
	if !result.KeepAll || !strings.Contains(result.Diagnostic, assetusage.ReasonDynamicLookup) {
		t.Fatalf("Analyze() = %+v", result)
	}
}

func TestAnalyzeTextureCallsAndEscapes(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name, body string
		keepAll    bool
	}{
		{"current setter", `new(engine.SpriteRenderer2D).SetTexture("sprites.player")`, false},
		{"legacy setter", `new(engine.Sprite2D).UpdateAsset("sprites.player")`, false},
		{"parenthesized call", `(new(engine.Game).NewSprite2D)("sprites.player", 0)`, false},
		{"method expression", `(*engine.Game).NewSprite2D(new(engine.Game), "sprites.player", 0)`, false},
		{"constructor value", `spawn := new(engine.Game).NewSprite2D; spawn("sprites.player", 0)`, true},
		{"setter value", `set := new(engine.SpriteRenderer2D).SetTexture; set("sprites.player")`, true},
		{"method expression value", `spawn := (*engine.Game).NewSprite2D; spawn(new(engine.Game), "sprites.player", 0)`, true},
		{"callback argument", `func(f func(engine.TextureID)) { f("sprites.player") }(new(engine.SpriteRenderer2D).SetTexture)`, true},
		{"dynamic setter", `name := "sprites.player"; new(engine.SpriteRenderer2D).SetTexture(engine.TextureID(name))`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			directory := usageProject(t, `package main
import "example.com/game/.karty/engine"
func main() { `+test.body+` }
`)

			result := assetusage.Analyze(directory, "example.com/game", []string{"sprites.player", "sprites.unused"})
			if result.KeepAll != test.keepAll || strings.Contains(result.Diagnostic, assetusage.ReasonAnalysisFailed) {
				t.Fatalf("Analyze() = %+v", result)
			}

			if !test.keepAll && (result.Live["sprites.player"] != assetusage.ReasonStaticName || result.Live["sprites.unused"] != "") {
				t.Fatalf("incorrect live set: %+v", result)
			}
		})
	}
}

func TestAnalyzeSelectsCartridgeSource(t *testing.T) {
	t.Parallel()

	for _, compiler := range []string{"go", "tinygo"} {
		t.Run(compiler, func(t *testing.T) {
			t.Parallel()

			directory := usageProject(t, "package main\nfunc main() {}\n")
			files := map[string]string{
				"sprite_wasip1_wasm.go": `//go:build wasip1 && wasm && !cgo

package main
import "example.com/game/.karty/engine"
func init() { new(engine.Game).NewSprite2D("sprites.player", 0); new(engine.Game).ShowUI("ui.inventory") }
`,
				"host.go": `//go:build !wasip1 || !wasm || cgo

package main
this is deliberately invalid source excluded by the cartridge target
`,
				"host_features.go": `//go:build amd64.v1 || arm64.v8.0

package main
this is deliberately invalid source excluded by the cartridge architecture
`,
				"tinygo.go": `//go:build tinygo && tinygo.wasm && scheduler.none

package main
import "example.com/game/.karty/engine"
func init() { new(engine.Game).NewSprite2D("sprites.tinygo", 0); new(engine.Game).ShowUI("ui.tinygo") }
`,
				"gc.go": `//go:build gc && !tinygo

package main
import "example.com/game/.karty/engine"
func init() { new(engine.Game).NewSprite2D("sprites.go", 0); new(engine.Game).ShowUI("ui.go") }
`,
				"gc_common.go": `//go:build gc

package main
import "example.com/game/.karty/engine"
func init() { new(engine.Game).NewSprite2D("sprites.gc", 0); new(engine.Game).ShowUI("ui.gc") }
`,
			}

			for name, source := range files {
				if err := os.WriteFile(filepath.Join(directory, "src", name), []byte(source), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			options := assetusage.Options{Compiler: compiler}

			textures := assetusage.AnalyzeWithOptions(
				directory,
				"example.com/game",
				[]string{"sprites.player", "sprites.go", "sprites.tinygo", "sprites.gc"},
				nil,
				options,
			)

			uiUsage := assetusage.AnalyzeUIWithOptions(
				directory,
				"example.com/game",
				[]string{"ui.inventory", "ui.go", "ui.tinygo", "ui.gc"},
				nil,
				options,
			)
			if textures.KeepAll || textures.Live["sprites.player"] == "" || uiUsage.KeepAll || uiUsage.Live["ui.inventory"] == "" {
				t.Fatalf("target source selection: textures=%+v ui=%+v", textures, uiUsage)
			}

			if textures.Live["sprites.gc"] == "" || uiUsage.Live["ui.gc"] == "" {
				t.Fatalf("gc compatibility tag: textures=%+v ui=%+v", textures, uiUsage)
			}

			other := "tinygo"
			if compiler == "tinygo" {
				other = "go"
			}

			if textures.Live["sprites."+compiler] == "" || textures.Live["sprites."+other] != "" || uiUsage.Live["ui."+compiler] == "" ||
				uiUsage.Live["ui."+other] != "" {
				t.Fatalf("compiler source selection: textures=%+v ui=%+v", textures, uiUsage)
			}
		})
	}
}

func TestAnalyzeHonorsGoBuildTags(t *testing.T) {
	t.Setenv("GOFLAGS", "-trimpath -tags=custom,cartridge")

	directory := usageProject(t, "package main\nfunc main() {}\n")

	source := `//go:build custom && cartridge

package main
import "example.com/game/.karty/engine"
func init() { new(engine.Game).NewSprite2D("sprites.player", 0) }
`
	if err := os.WriteFile(filepath.Join(directory, "src", "tagged.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}

	result := assetusage.Analyze(directory, "example.com/game", []string{"sprites.player"})
	if result.KeepAll || result.Live["sprites.player"] == "" {
		t.Fatalf("Analyze() = %+v", result)
	}

	result = assetusage.AnalyzeWithOptions(
		directory,
		"example.com/game",
		[]string{"sprites.player"},
		nil,
		assetusage.Options{Compiler: "tinygo"},
	)
	if result.KeepAll || result.Live["sprites.player"] != "" {
		t.Fatalf("TinyGo must override GOFLAGS tags: %+v", result)
	}
}

func TestAnalyzeRetainsScopeForUncertainCompilerConstraints(t *testing.T) {
	t.Parallel()

	for _, tag := range []string{"go1.999", "goexperiment.futureexperiment"} {
		t.Run(tag, func(t *testing.T) {
			t.Parallel()

			directory := usageProject(t, "package main\nfunc main() {}\n")

			source := "//go:build " + tag + "\n\npackage main\nfunc init() {}\n"
			if err := os.WriteFile(filepath.Join(directory, "src", "future.go"), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}

			for _, compiler := range []string{"go", "tinygo"} {
				options := assetusage.Options{Compiler: compiler}

				textures := assetusage.AnalyzeWithOptions(directory, "example.com/game", nil, nil, options)

				uiUsage := assetusage.AnalyzeUIWithOptions(directory, "example.com/game", nil, nil, options)
				if !textures.KeepAll || !uiUsage.KeepAll || !strings.Contains(textures.Diagnostic, "compiler release or experiment") {
					t.Fatalf("uncertain constraints: textures=%+v ui=%+v", textures, uiUsage)
				}
			}
		})
	}
}

func usageProject(t *testing.T, source string) string {
	t.Helper()

	directory := t.TempDir()
	engineDirectory := filepath.Join(directory, ".karty", "engine")
	assetsDirectory := filepath.Join(directory, ".karty", "assets")

	sourceDirectory := filepath.Join(directory, "src")
	for _, path := range []string{engineDirectory, assetsDirectory, sourceDirectory} {
		if err := os.MkdirAll(path, 0o750); err != nil {
			t.Fatal(err)
		}
	}

	engine := `package engine
type TextureID string
type UIAsset string
const UIInventory UIAsset = "ui.inventory"
func (*Game) ShowUI(UIAsset) {}
func (*Game) ShowInventory() {}
type Game struct{}
func (*Game) NewSprite2D(TextureID, int) {}
type Sprite2D struct{}
func (*Sprite2D) UpdateAsset(TextureID) {}
type SpriteRenderer2D struct{}
func (*SpriteRenderer2D) SetTexture(TextureID) {}
func DynamicTexture(string) TextureID { return "" }
`
	if err := os.WriteFile(filepath.Join(engineDirectory, "engine.go"), []byte(engine), 0o600); err != nil {
		t.Fatal(err)
	}

	assets := `package assets
import "example.com/game/.karty/engine"
const (
	TextureSpritesPlayer engine.TextureID = "sprites.player"
	TextureSpritesUnused engine.TextureID = "sprites.unused"
)
`
	if err := os.WriteFile(filepath.Join(assetsDirectory, "assets.go"), []byte(assets), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(sourceDirectory, "main.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}

	return directory
}
