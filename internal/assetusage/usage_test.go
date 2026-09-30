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
func spawn(name string) { new(engine.Game).NewSprite2DName(name, 0) }
func main() { spawn("sprites.player") }
`)

	result := assetusage.Analyze(directory, "example.com/game", []string{"sprites.player", "sprites.unused"})
	if !result.KeepAll || !strings.Contains(result.Diagnostic, assetusage.ReasonDynamicLookup) {
		t.Fatalf("Analyze() = %+v", result)
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
func (*Game) NewSprite2DName(string, int) {}
type Sprite2d struct{}
func (*Sprite2d) UpdateAsset(TextureID) {}
func (*Sprite2d) UpdateAssetName(string) {}
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
