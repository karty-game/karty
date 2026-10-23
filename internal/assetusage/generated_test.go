package assetusage_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/assetusage"
)

func writeGenerated(t *testing.T, directory, name, source string) {
	t.Helper()

	path := filepath.Join(directory, ".karty", "processed", "code", filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratedPackagesRetainReferencesAndPruneUnusedAssets(t *testing.T) {
	t.Parallel()

	directory := usageProject(t, `package main
import (
 "example.com/game/.karty/engine"
 "example.com/game/.karty/processed/code/tool.sdk/adapter"
 "example.com/game/.karty/processed/code/tool.sdk/helper"
)
func main() {
 adapter.Paint(new(engine.SpriteRenderer2D))
 helper.Paint(adapter.Target(), new(engine.SpriteRenderer2D))
}
`)
	writeGenerated(t, directory, "tool.sdk/adapter/adapter.go", `package adapter
import (
 "example.com/game/.karty/engine"
 "example.com/game/.karty/processed/code/tool.sdk/helper"
)
func Target() helper.Target { return helper.Target{} }
func Paint(r *engine.SpriteRenderer2D) {
 helper.Paint(Target(), r)
 new(engine.Game).NewSprite2D("sprites.generated", 0)
 new(engine.Game).ShowUI(engine.UIInventory)
}
`)
	writeGenerated(t, directory, "tool.sdk/helper/helper.go", `package helper
import (
 "example.com/game/.karty/assets"
 "example.com/game/.karty/engine"
)
type Target struct{}
func Paint(_ Target, r *engine.SpriteRenderer2D) { r.SetTexture(assets.TextureSpritesPlayer) }
`)

	result := assetusage.Analyze(directory, "example.com/game", []string{"sprites.player", "sprites.generated", "sprites.unused"})
	if result.KeepAll || result.Diagnostic != "" || result.Live["sprites.player"] != assetusage.ReasonTypedReference ||
		result.Live["sprites.generated"] != assetusage.ReasonStaticName || result.Live["sprites.unused"] != "" {
		t.Fatalf("incorrect generated package texture usage: %+v", result)
	}

	ui := assetusage.AnalyzeUI(directory, "example.com/game", []string{"ui.inventory", "ui.unused"})
	if ui.KeepAll || ui.Live["ui.inventory"] == "" || ui.Live["ui.unused"] != "" {
		t.Fatalf("incorrect generated package UI usage: %+v", ui)
	}
}

func TestGeneratedPackageDynamicLookupRetainsScope(t *testing.T) {
	t.Parallel()

	directory := usageProject(t, `package main
import "example.com/game/.karty/processed/code/tool/adapter"
func main() { adapter.Spawn("sprites.player") }
`)
	writeGenerated(t, directory, "tool/adapter/adapter.go", `package adapter
import "example.com/game/.karty/engine"
func Spawn(name string) { new(engine.Game).NewSprite2D(engine.DynamicTexture(name), 0) }
`)

	result := assetusage.Analyze(directory, "example.com/game", []string{"sprites.player", "sprites.unused"})
	if !result.KeepAll || !strings.Contains(result.Diagnostic, assetusage.ReasonDynamicLookup) ||
		strings.Contains(result.Diagnostic, assetusage.ReasonAnalysisFailed) {
		t.Fatalf("dynamic lookup was not analyzed: %+v", result)
	}

	if !strings.Contains(result.Diagnostic, ".karty/processed/code/tool/adapter/adapter.go") {
		t.Fatalf("diagnostic lost generated source location: %+v", result)
	}
}

func TestGeneratedPackageErrorsRetainScope(t *testing.T) {
	t.Parallel()

	for _, test := range []struct{ name, source string }{
		{"syntax", "package adapter\nfunc invalid( {"},
		{"type", "package adapter\nfunc Use() { unknown() }"},
		{"cycle", `package adapter
import "example.com/game/.karty/processed/code/tool/helper"
func Use() { helper.Use() }`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			directory := usageProject(t, `package main
import "example.com/game/.karty/processed/code/tool/adapter"
func main() { adapter.Use() }
`)
			writeGenerated(t, directory, "tool/adapter/adapter.go", test.source)
			writeGenerated(t, directory, "tool/helper/helper.go", `package helper
import "example.com/game/.karty/processed/code/tool/adapter"
func Use() { adapter.Use() }
`)

			result := assetusage.Analyze(directory, "example.com/game", []string{"sprites.player", "sprites.unused"})
			if !result.KeepAll || !strings.Contains(result.Diagnostic, assetusage.ReasonAnalysisFailed) {
				t.Fatalf("invalid generated code lost conservative fallback: %+v", result)
			}

			if test.name == "cycle" && !strings.Contains(result.Diagnostic, "import cycle") {
				t.Fatalf("cycle was not diagnosed: %+v", result)
			}
		})
	}
}

func TestGeneratedPackagesUseCartridgeBuildContext(t *testing.T) {
	t.Parallel()

	for _, compiler := range []string{"go", "tinygo"} {
		t.Run(compiler, func(t *testing.T) {
			t.Parallel()

			directory := usageProject(t, `package main
import "example.com/game/.karty/processed/code/tool/adapter"
func main() { adapter.Use() }
`)
			writeGenerated(t, directory, "tool/adapter/gc_wasip1_wasm.go", `//go:build !tinygo && !cgo

package adapter
import "example.com/game/.karty/engine"
func Use() { new(engine.Game).NewSprite2D("sprites.go", 0) }
`)
			writeGenerated(t, directory, "tool/adapter/tinygo_wasip1_wasm.go", `//go:build tinygo && scheduler.none && !cgo

package adapter
import "example.com/game/.karty/engine"
func Use() { new(engine.Game).NewSprite2D("sprites.tinygo", 0) }
`)
			writeGenerated(t, directory, "tool/adapter/host.go", `//go:build !wasip1 || !wasm || cgo

package adapter
this source must be excluded from cartridge analysis
`)
			writeGenerated(t, directory, "tool/adapter/invalid_test.go", "package adapter\ninvalid test source")

			result := assetusage.AnalyzeWithOptions(
				directory,
				"example.com/game",
				[]string{"sprites.go", "sprites.tinygo", "sprites.unused"},
				nil,
				assetusage.Options{Compiler: compiler},
			)

			want, excluded := "sprites.go", "sprites.tinygo"
			if compiler == "tinygo" {
				want, excluded = excluded, want
			}

			if result.KeepAll || result.Live[want] != assetusage.ReasonStaticName || result.Live[excluded] != "" ||
				result.Live["sprites.unused"] != "" {
				t.Fatalf("generated package used wrong compiler source selection: %+v", result)
			}
		})
	}
}
