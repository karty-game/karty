package generator

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty/website"
)

func TestSiteDocsShareAssetsAndKeepRelativeLinks(t *testing.T) {
	t.Parallel()

	output := t.TempDir()
	if err := GenerateSiteDocs(output); err != nil {
		t.Fatal(err)
	}

	for _, section := range []string{"guide", "design", "technical"} {
		if _, err := os.Stat(filepath.Join(output, section, "assets")); !os.IsNotExist(err) {
			t.Fatalf("%s duplicates assets: %v", section, err)
		}
	}

	for name, link := range map[string]string{
		"guide/fps.html":          `src="../assets/hangar.jpg"`,
		"design/why.md":           `(../assets/design-room.png)`,
		"technical/graphics.html": `href="../assets/particle-spark.png"`,
	} {
		data, err := os.ReadFile(filepath.Join(output, name))
		if err != nil {
			t.Fatal(err)
		}

		if !strings.Contains(string(data), link) {
			t.Fatalf("%s missing %s", name, link)
		}
	}

	assets, err := website.Files.ReadDir("assets")
	if err != nil {
		t.Fatal(err)
	}

	for _, asset := range assets {
		want, err := website.Files.ReadFile("assets/" + asset.Name())
		if err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(filepath.Join(output, "assets", asset.Name()))
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(got, want) {
			t.Fatalf("changed shared asset %s", asset.Name())
		}
	}
}
