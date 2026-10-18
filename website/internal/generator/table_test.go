package generator

import (
	"strings"
	"testing"
)

func TestPlatformTableEscapesCellsAndKeepsCodeAndLinks(t *testing.T) {
	t.Parallel()

	output, _, err := renderMarkdown(
		"| Platform | Build |\n| --- | --- |\n| <script> | `windows-arm64` |\n| Browser | [Share](share.html) |\n\n## Next\n",
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{`<th scope="col">Platform</th>`, "&lt;script&gt;", "<code>windows-arm64</code>", `href="share.html"`, "</table></div>", `<h2 id="next">`} {
		if !strings.Contains(output, want) {
			t.Fatalf("table missing %q: %s", want, output)
		}
	}

	if strings.Contains(output, "<script>") {
		t.Fatal("table accepted raw HTML")
	}
}

func TestPlatformTableRejectsMalformedRowsAndUnsafeLinks(t *testing.T) {
	t.Parallel()

	for _, markdown := range []string{
		"|\n",
		"| Header |\n",
		"| Header |\n| wrong |\n",
		"| A | B |\n| --- | --- |\n| only one |\n",
		"| A |\n| --- |\n| [bad](javascript:alert) |\n",
		"| A |\n| --- |\n| incomplete\n",
	} {
		if _, _, err := renderMarkdown(markdown); err == nil {
			t.Fatalf("accepted malformed table: %q", markdown)
		}
	}
}
