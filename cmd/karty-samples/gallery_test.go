package main

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/sitetheme"
)

func previewFixture(t *testing.T, directory string) {
	t.Helper()
	writeFixture(
		t,
		filepath.Join(directory, "preview.toml"),
		"title = 'A sample'\ndescription = 'Build a room and try its lights.'\nalt = 'A lit room.'\n",
	)

	var output bytes.Buffer
	if err := jpeg.Encode(&output, image.NewGray(image.Rect(0, 0, 960, 540)), nil); err != nil {
		t.Fatal(err)
	}

	writeFixture(t, filepath.Join(directory, "preview.jpg"), output.String())
}

func TestGalleryExplainsEachPublishedSample(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..")

	names, err := sampleNames(root)
	if err != nil {
		t.Fatal(err)
	}

	cards, err := sampleCards(root, names)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(names, []string{"hangar", "crate-room", "environment", "world-camera", "ui-demo", "media-lab", "particle-lab"}) {
		t.Fatalf("published samples: %v", names)
	}

	output := t.TempDir()
	if err = writeGallery(output, cards); err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile(filepath.Join(output, "index.html"))
	if err != nil {
		t.Fatal(err)
	}

	page := string(contents)
	if !strings.Contains(page, sitetheme.CSS()) {
		t.Fatal("gallery does not use the shared guide theme")
	}

	for index, name := range names {
		for _, want := range []string{
			cards[index].Preview.Title, cards[index].Preview.Description,
			`src="` + name + `/preview.jpg"`, `href="` + name + `/"`,
			`https://github.com/karty-game/karty/tree/main/samples/` + name,
		} {
			if !strings.Contains(page, want) {
				t.Fatalf("%s gallery missing %q", name, want)
			}
		}

		if err = copySamplePreview(filepath.Join(root, "samples", name), output); err != nil {
			t.Fatal(err)
		}

		actual, err := os.ReadFile(filepath.Join(output, "preview.jpg"))
		if err != nil {
			t.Fatal(err)
		}

		original, err := os.ReadFile(filepath.Join(root, "samples", name, "preview.jpg"))
		if err != nil || !bytes.Equal(actual, original) {
			t.Fatalf("%s screenshot changed while copying: %v", name, err)
		}
	}

	if !strings.Contains(page, `href="guide/"`) || !strings.Contains(page, `href="environment/"`) {
		t.Fatal("guide or environment missing from published gallery")
	}
}

func TestGalleryIncludesEnvironmentInPriorityOrder(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..")

	names, err := sampleNames(root)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"hangar", "crate-room", "environment", "world-camera", "ui-demo", "media-lab", "particle-lab"}
	if !slices.Equal(names, want) {
		t.Fatalf("gallery order: %v", names)
	}

	cards, err := sampleCards(root, names)
	if err != nil {
		t.Fatal(err)
	}

	output := t.TempDir()
	if err := writeGallery(output, cards); err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile(filepath.Join(output, "index.html"))
	if err != nil {
		t.Fatal(err)
	}

	page := string(contents)
	previous := -1

	for _, name := range want {
		position := strings.Index(page, `src="`+name+`/preview.jpg"`)
		if position <= previous {
			t.Fatalf("%s missing or out of order in rendered gallery", name)
		}

		previous = position
	}

	if !strings.Contains(page, "non-Euclidean level geometry") {
		t.Fatal("world-camera card does not advertise non-Euclidean geometry")
	}
}

func TestGalleryEscapesTextAndRejectsUnsafeURLs(t *testing.T) {
	t.Parallel()

	output := t.TempDir()

	cards := []sampleCard{
		{
			Preview: samplePreview{
				Title:       `<script>alert("title")</script>`,
				Description: `<img src=x onerror="alert(1)">`,
				Alt:         `" onerror="alert(2)`,
			},
			DemoURL:   "javascript:alert(1)",
			ImageURL:  "javascript:alert(2)",
			SourceURL: "javascript:alert(3)",
		},
	}
	if err := writeGallery(output, cards); err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile(filepath.Join(output, "index.html"))
	if err != nil {
		t.Fatal(err)
	}

	page := string(contents)
	for _, invalid := range []string{"<script>", "<img src=x", "javascript:", `alt="" onerror=`} {
		if strings.Contains(page, invalid) {
			t.Fatalf("unsafe gallery content %q", invalid)
		}
	}

	if !strings.Contains(page, "&lt;script&gt;") {
		t.Fatal("escaped title missing")
	}
}

func TestSamplePreviewRejectsIncompleteOrLinkedFiles(t *testing.T) {
	t.Parallel()

	for _, invalid := range []string{"metadata", "missing title", "unknown field", "image", "oversize", "symlink"} {
		t.Run(invalid, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			previewFixture(t, directory)

			switch invalid {
			case "metadata":
				writeFixture(t, filepath.Join(directory, "preview.toml"), "title = [")
			case "missing title":
				writeFixture(t, filepath.Join(directory, "preview.toml"), "description = 'A room.'\nalt = 'A room.'\n")
			case "unknown field":
				writeFixture(
					t,
					filepath.Join(directory, "preview.toml"),
					"title = 'A room'\ndescription = 'A room.'\nalt = 'A room.'\nsource = 'javascript:alert(1)'\n",
				)
			case "image":
				writeFixture(t, filepath.Join(directory, "preview.jpg"), "not an image")
			case "oversize":
				if err := os.Truncate(filepath.Join(directory, "preview.jpg"), maxPreviewBytes+1); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Rename(filepath.Join(directory, "preview.jpg"), filepath.Join(directory, "linked.jpg")); err != nil {
					t.Fatal(err)
				}

				if err := os.Symlink(filepath.Join(directory, "linked.jpg"), filepath.Join(directory, "preview.jpg")); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			}

			if _, err := readSamplePreview(directory); err == nil {
				t.Fatal("invalid preview accepted")
			}
		})
	}
}

func TestSampleCardsRejectPathAndURLInjection(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"../outside", "bad/name", "bad name", `bad" onclick="alert(1)`, "javascript:alert(1)", "guide"} {
		if _, err := sampleCards(t.TempDir(), []string{name}); err == nil {
			t.Fatalf("invalid sample name %q accepted", name)
		}
	}
}

func TestAssembleIncludesGuideAndScreenshots(t *testing.T) {
	t.Parallel()

	state, source := sampleFixture(t)
	writeFixture(t, filepath.Join(source, "guide", "index.html"), "guide page")
	writeFixture(t, filepath.Join(source, "guide", "guide.md"), "# Guide\n")
	previewFixture(t, filepath.Join(source, "ui-demo"))

	if err := assemble(state, source, "main", false, nil); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"guide/index.html", "guide/guide.md", "ui-demo/preview.jpg"} {
		if err := requireFile(filepath.Join(state, "main", filepath.FromSlash(path))); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.Remove(filepath.Join(source, "guide", "guide.md")); err != nil {
		t.Fatal(err)
	}

	if err := assemble(state, source, "main", false, nil); err == nil {
		t.Fatal("incomplete guide accepted")
	}

	if err := requireFile(filepath.Join(state, "main", "guide", "guide.md")); err != nil {
		t.Fatal("working guide replaced before validation", err)
	}
}

func TestGuideAloneIsNotASampleSite(t *testing.T) {
	t.Parallel()

	source := t.TempDir()
	for _, path := range []string{"index.html", "guide/index.html", "guide/guide.md"} {
		writeFixture(t, filepath.Join(source, filepath.FromSlash(path)), "guide")
	}

	if err := validateSamples(os.DirFS(source)); err == nil {
		t.Fatal("site without playable samples accepted")
	}
}
