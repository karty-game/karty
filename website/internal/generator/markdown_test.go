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
		"**[link](javascript:alert)**",
		"[**link**](javascript:alert)",
		"**text `unclosed**",
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

func TestInlineBoldFormatting(t *testing.T) {
	t.Parallel()

	for _, test := range []struct{ input, want string }{
		{"**See changes immediately.** Preview the room.", "<strong>See changes immediately.</strong> Preview the room."},
		{"**first** and **second**", "<strong>first</strong> and <strong>second</strong>"},
		{"**Use `karty dev`**", "<strong>Use <code>karty dev</code></strong>"},
		{"**Use `**literal**` here**", "<strong>Use <code>**literal**</code> here</strong>"},
		{"`**literal**` and **bold**", "<code>**literal**</code> and <strong>bold</strong>"},
		{"**[Read more](guide.html)**", `<strong><a href="guide.html">Read more</a></strong>`},
		{"[**Read** `more`](guide.html)", `<a href="guide.html"><strong>Read</strong> <code>more</code></a>`},
		{"**[Read](https://example.com/**path)**", `<strong><a href="https://example.com/**path">Read</a></strong>`},
		{"**<script> & unsafe**", "<strong>&lt;script&gt; &amp; unsafe</strong>"},
		{"**unclosed and `code`", "**unclosed and <code>code</code>"},
		{"**closed** then **unclosed", "<strong>closed</strong> then **unclosed"},
		{"**** and ** spaced** and **spaced **", "**** and ** spaced** and **spaced **"},
		{"a * b and **", "a * b and **"},
	} {
		t.Run(test.input, func(t *testing.T) {
			t.Parallel()

			got, err := renderInline(test.input)
			if err != nil || got != test.want {
				t.Fatalf("renderInline(%q) = %q, %v; want %q", test.input, got, err, test.want)
			}
		})
	}
}

func TestRendererBoldAcrossBodyElements(t *testing.T) {
	t.Parallel()

	output, _, err := renderMarkdown(
		"**Paragraph**\n\n- **Bullet** `command`\n\n| Label |\n| --- |\n| **Cell** |\n\n```md\n**literal**\n```\n",
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"<p><strong>Paragraph</strong></p>",
		"<li><strong>Bullet</strong> <code>command</code></li>",
		"<td><strong>Cell</strong></td>",
		"<pre><code class=\"language-md\">**literal**\n</code></pre>",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing %q in rendered body: %s", want, output)
		}
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
