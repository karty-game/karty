package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/cartridge"
	definition "github.com/karty-game/karty-ui/schema"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/project"
	"golang.org/x/image/font/gofont/goregular"
)

func TestAnalyzeProjectFontsReportsUsedAndStrippedRoles(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	for _, name := range []string{"body.ttf", "mono.ttf"} {
		if err := os.WriteFile(filepath.Join(directory, name), goregular.TTF, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	fonts := []project.Font{{Role: "body", Source: "body.ttf"}, {Role: "mono", Source: "mono.ttf"}}
	report := assetpipeline.Report{}

	if err := analyzeProjectFonts(directory, fonts, map[string]bool{"body": true}, &report); err != nil {
		t.Fatal(err)
	}

	if report.Summary.DeclaredFontCount != 2 || report.Summary.UsedFontCount != 1 ||
		report.Summary.StrippedFontCount != 1 || len(report.Fonts) != 2 ||
		report.Fonts[0].Status != "used" || report.Fonts[1].Status != "stripped" {
		t.Fatalf("font report = %+v, summary = %+v", report.Fonts, report.Summary)
	}
}

func TestEmbedProjectAssetsRetainsOnlyUsedFontRoles(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	for _, name := range []string{"body.ttf", "mono.ttf"} {
		if err := os.WriteFile(filepath.Join(directory, name), goregular.TTF, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	artifact := filepath.Join(directory, "game.kart")
	if err := os.WriteFile(artifact, []byte("\x00asm\x01\x00\x00\x00"), 0o600); err != nil {
		t.Fatal(err)
	}

	fonts := []project.Font{{Role: "body", Source: "body.ttf"}, {Role: "mono", Source: "mono.ttf"}}
	if err := embedProjectAssets(directory, artifact, nil, fonts, map[string]bool{"body": true}); err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}

	assets, err := cartridge.ExtractAssets(contents)
	if err != nil {
		t.Fatal(err)
	}

	if len(assets) != 1 || assets[0].Name != definition.FontAssetPrefix+"body" {
		t.Fatalf("embedded assets = %+v", assets)
	}
}

func TestAnalyzeProjectFontsRejectsMalformedFont(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "body.ttf"), []byte("not a font"), 0o600); err != nil {
		t.Fatal(err)
	}

	report := assetpipeline.Report{}

	err := analyzeProjectFonts(
		directory,
		[]project.Font{{Role: "body", Source: "body.ttf"}},
		map[string]bool{"body": true},
		&report,
	)
	if err == nil || !strings.Contains(err.Error(), "decode body font") {
		t.Fatalf("analyzeProjectFonts() error = %v", err)
	}
}
