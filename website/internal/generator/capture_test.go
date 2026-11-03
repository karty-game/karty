package generator

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestSoftwareCaptureGuideIsPublished(t *testing.T) {
	t.Parallel()

	site := t.TempDir()

	output := filepath.Join(site, "guide")
	if err := Generate(output); err != nil {
		t.Fatal(err)
	}

	if err := GenerateDesign(filepath.Join(site, "design")); err != nil {
		t.Fatal(err)
	}

	if err := GenerateTechnical(filepath.Join(site, "technical")); err != nil {
		t.Fatal(err)
	}

	page := readGenerated(t, output, "software-capture.html")

	for _, text := range []string{
		"karty render screenshot", "karty render reflections", "--backend software",
		"validated WebGPU native host bundle", "--backend hardware",
		"does not switch backends automatically",
	} {
		if !bytes.Contains(page, []byte(text)) {
			t.Fatal("published software capture guide is missing", text)
		}
	}

	if !bytes.Contains(readGenerated(t, output, "lighting.html"), []byte(`href="software-capture.html"`)) {
		t.Fatal("lighting guide does not link to the published capture page")
	}

	checkGuideLinks(t, output, page)
}
