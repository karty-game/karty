package generator

import (
	"strings"
	"testing"
)

func TestRendererEscapesTextAndCode(t *testing.T) {
	t.Parallel()

	output, _, err := renderMarkdown("# A <heading>\n\n<script>alert(1)</script> and `x < y`\n\n```go\nx < y && z\n```\n")
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(output, "<script>") || !strings.Contains(output, "&lt;script&gt;") ||
		!strings.Contains(output, "<code>x &lt; y</code>") || !strings.Contains(output, "x &lt; y &amp;&amp; z") {
		t.Fatalf("unescaped HTML: %s", output)
	}
}

func TestRendererRejectsBrokenSyntaxAndUnsafeLinks(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"```go\nunclosed",
		"```go\" onclick=\"bad\n```",
		"# Heading\n\n# Heading",
		"# \n",
		"#### Heading",
		"text `unclosed",
		"[link](javascript:alert)",
		"[link](data:text/html,script)",
		"[link](//example.com)",
		"[link](/absolute)",
		"[link](https://user@example.com)",
		"[link](../two words/)",
		"[link](../back\\slash/)",
		"[link](unclosed",
		"![bad](https://example.com/picture.jpg)",
		"![bad](assets/../picture.jpg)",
		"![missing](assets/missing.jpg)",
	} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			if _, _, err := renderMarkdown(input); err == nil {
				t.Fatalf("accepted malformed guide: %q", input)
			}
		})
	}
}

func TestRendererPreservesGuideLinksAndBullets(t *testing.T) {
	t.Parallel()

	output, sections, err := renderMarkdown("## Build & share\n\n- [Sample](../hangar/)\n- [Release](https://example.com/a?x=1&y=2)\n")
	if err != nil {
		t.Fatal(err)
	}

	if len(sections) != 1 || sections[0].ID != "build-share" ||
		!strings.Contains(output, `<a href="../hangar/">Sample</a>`) ||
		!strings.Contains(output, `href="https://example.com/a?x=1&amp;y=2"`) || strings.Count(output, "<li>") != 2 {
		t.Fatalf("changed Markdown navigation: %s, %+v", output, sections)
	}
}
