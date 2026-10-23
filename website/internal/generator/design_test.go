package generator

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDesignPagesAndGuideMigration(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	guideOutput, designOutput := filepath.Join(root, "guide"), filepath.Join(root, "design")
	if err := Generate(guideOutput); err != nil {
		t.Fatal(err)
	}

	if err := GenerateDesign(designOutput); err != nil {
		t.Fatal(err)
	}

	if err := GenerateTechnical(filepath.Join(root, "technical")); err != nil {
		t.Fatal(err)
	}

	pages, err := designPages()
	if err != nil {
		t.Fatal(err)
	}

	before := map[string][]byte{}

	for _, page := range pages {
		for _, suffix := range []string{".html", ".md"} {
			name := page.name + suffix
			before[name] = readGenerated(t, designOutput, name)
		}

		html := before[page.name+".html"]
		checkGuideLinks(t, designOutput, html)
		checkGuideImages(t, designOutput, html)

		if !bytes.Contains(html, []byte(`href="`+page.name+`.md" download`)) ||
			!regexp.MustCompile(`href="index.html"\s+aria-current="page"`).Match(html) ||
			!bytes.Contains(html, []byte(`aria-label="Design pages"`)) ||
			bytes.Contains(html, []byte(`aria-label="Shared topics"`)) {
			t.Fatalf("design page has incorrect navigation or download: %s", page.name)
		}
	}

	if err := GenerateDesign(designOutput); err != nil {
		t.Fatal(err)
	}

	for name, want := range before {
		if !bytes.Equal(want, readGenerated(t, designOutput, name)) {
			t.Fatalf("design generation is not deterministic: %s", name)
		}
	}

	legacy := string(readGenerated(t, guideOutput, "why.html"))
	if !strings.Contains(legacy, `http-equiv="refresh" content="0; url=../design/why.html"`) ||
		!strings.Contains(legacy, `rel="canonical" href="../design/why.html"`) ||
		!strings.Contains(legacy, `href="../design/why.html"`) {
		t.Fatal("old Why Karty bookmark does not forward to Design")
	}

	if bytes.Contains(readGenerated(t, guideOutput, "guide.md"), []byte("# Why Karty?")) {
		t.Fatal("design explanation is still included in the complete tutorial")
	}

	oldWorkflow := string(before["sdk-and-toolchain.html"])
	if !strings.Contains(oldWorkflow, `http-equiv="refresh" content="0; url=workflow.html"`) ||
		!strings.Contains(oldWorkflow, `rel="canonical" href="workflow.html"`) {
		t.Fatal("old setup bookmark must forward to the current workflow")
	}

	if !bytes.Contains(before["why.html"], []byte("assets/design-room.png")) ||
		!bytes.Contains(before["architecture.html"], []byte("assets/design-architecture.svg")) ||
		!bytes.Contains(before["asset-pipeline.html"], []byte(`<figure class="journey"><svg`)) {
		t.Fatal("design pages are missing their illustration or offline diagrams")
	}
}

func TestPublicDocsUseDefaultSetup(t *testing.T) {
	t.Parallel()

	var pages []guidePage

	for _, load := range []func() ([]guidePage, error){guidePages, designPages, technicalPages} {
		loaded, err := load()
		if err != nil {
			t.Fatal(err)
		}

		pages = append(pages, loaded...)
	}

	for _, page := range pages {
		// Actual processor identifiers and repository URLs remain valid code/links.
		text := strings.ReplaceAll(page.markdown, "controllers-3d.sdk", "controllers-3d.adapter")
		text = strings.ReplaceAll(text, "github.com/karty-game/karty-sdk/", "github.com/karty-game/library/")

		if regexp.MustCompile(`(?i)\bsdk\b`).MatchString(text) || strings.Contains(text, "--sdk") {
			t.Errorf("public page %s exposes internal version selection", page.name)
		}
	}

	for _, topic := range designTopics() {
		if topic.URL == "sdk-and-toolchain.html" || strings.Contains(strings.ToLower(topic.Name), "sdk") {
			t.Fatal("public navigation still advertises internal version selection")
		}
	}
}

func TestDesignOutputConfinementAndPreservation(t *testing.T) {
	t.Parallel()

	output := t.TempDir()
	if err := os.WriteFile(filepath.Join(output, "notes.txt"), []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := GenerateDesign(output); err != nil {
		t.Fatal(err)
	}

	if got := string(readGenerated(t, output, "notes.txt")); got != "keep me" {
		t.Fatalf("design generation changed unrelated content: %q", got)
	}

	for _, target := range []string{"parent", "assets", "architecture.html", "packages.md", "assets/design-room.png"} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			root, outside := t.TempDir(), t.TempDir()

			output := filepath.Join(root, "design")
			if err := os.MkdirAll(filepath.Join(output, "assets"), 0o755); err != nil {
				t.Fatal(err)
			}

			path, linkTarget := filepath.Join(output, target), outside
			switch target {
			case "parent":
				path = filepath.Join(root, "linked")
				output = filepath.Join(path, "design")
			case "assets":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			default:
				linkTarget = filepath.Join(outside, "untouched")
				if err := os.WriteFile(linkTarget, []byte("keep me"), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			if err := os.Symlink(linkTarget, path); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}

			if err := GenerateDesign(output); !errors.Is(err, os.ErrInvalid) {
				t.Fatalf("linked design destination accepted: %v", err)
			}

			if _, err := os.Stat(filepath.Join(output, "index.html")); !os.IsNotExist(err) {
				t.Fatalf("wrote design pages before validating destinations: %v", err)
			}
		})
	}
}
