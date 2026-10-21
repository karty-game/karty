package website

import (
	"bytes"
	"errors"
	"html/template"
	"os"
	"strings"
	"testing"
)

func TestSocialMetadataKeepsPublishedPrefixes(t *testing.T) {
	t.Parallel()

	for _, example := range []struct{ base, page, want string }{
		{defaultSiteURL, "guide/index.html", defaultSiteURL + "guide/index.html"},
		{"https://docs.example/karty/pr/17", "technical/input.html", "https://docs.example/karty/pr/17/technical/input.html"},
		{"https://docs.example/karty/pr/17/", "", "https://docs.example/karty/pr/17/"},
	} {
		card, err := socialForBase(example.base, "Karty", "Retro looks, modern tools.", example.page)
		if err != nil {
			t.Fatal(err)
		}

		if card.URL != example.want || card.ImageURL != strings.TrimRight(example.base, "/")+"/guide/assets/banner.png" {
			t.Fatalf("social URLs escaped the published preview: %+v", card)
		}
	}
}

func TestSocialMetadataRejectsAmbiguousURLs(t *testing.T) {
	t.Parallel()

	for _, base := range []string{"/relative", "//example.com", "data:text/html,hello", "https://user:secret@example.com/", "https://example.com/?theme=dark", "https://example.com/#docs"} {
		if _, err := socialForBase(base, "Karty", "", "guide/index.html"); !errors.Is(err, os.ErrInvalid) {
			t.Errorf("invalid base accepted: %q: %v", base, err)
		}
	}

	for _, page := range []string{"/guide/index.html", "//other.example/a", "https://other.example/a", "../banner.png", "%2e%2e/banner.png", "guide/../../banner.png", "guide\\index.html", "guide/index.html?query", "guide/index.html#fragment"} {
		if _, err := socialForBase(defaultSiteURL, "Karty", "", page); !errors.Is(err, os.ErrInvalid) {
			t.Errorf("invalid page accepted: %q: %v", page, err)
		}
	}
}

func TestSocialTemplateEscapesPageText(t *testing.T) {
	t.Parallel()

	card, err := socialForBase(defaultSiteURL, `Karty " onload="alert(1)`, `<script>unsafe</script>`, "guide/index.html")
	if err != nil {
		t.Fatal(err)
	}

	page, err := template.ParseFS(Files, "templates/social.html")
	if err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err = page.ExecuteTemplate(&output, "social", card); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(output.String(), `onload="alert`) || strings.Contains(output.String(), "<script>") {
		t.Fatal("social metadata injected unescaped page text")
	}

	if !strings.Contains(output.String(), `content="summary_large_image"`) ||
		!strings.Contains(output.String(), `content="`+card.ImageURL+`"`) {
		t.Fatal("social metadata lost the banner card")
	}
}
