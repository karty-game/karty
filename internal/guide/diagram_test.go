package guide

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestJourneyDiagramMatchesMermaidAndEscapesLabels(t *testing.T) {
	t.Parallel()

	output, err := renderJourney("flowchart TD\nA[Create <a>] --> B[Build & share]")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output, "Create &lt;a&gt;") || !strings.Contains(output, "Build &amp; share") {
		t.Fatalf("diagram changed or failed to escape labels: %s", output)
	}

	var figure struct {
		SVG struct {
			Texts []string `xml:"text"`
		} `xml:"svg"`
	}
	if err := xml.Unmarshal([]byte(output), &figure); err != nil {
		t.Fatalf("invalid SVG: %v", err)
	}

	if len(figure.SVG.Texts) != 2 || figure.SVG.Texts[0] != "Create <a>" || figure.SVG.Texts[1] != "Build & share" {
		t.Fatalf("diagram lost its steps: %+v", figure)
	}
}

func TestJourneyDiagramRejectsUnsupportedGraphs(t *testing.T) {
	t.Parallel()

	for _, code := range []string{
		"flowchart LR\nA[One] --> B[Two]",
		"flowchart TD\nA --> B[Two]",
		"flowchart TD\nA[One] --> B[Two]\nC[Three] --> D[Four]",
		"flowchart TD\nA[One] --> B[Two]\nB --> A[One]",
		"flowchart TD\nA[One] --> B[Two]\nB[Changed] --> C[Three]",
	} {
		t.Run(code, func(t *testing.T) {
			t.Parallel()

			if _, err := renderJourney(code); err == nil {
				t.Fatalf("accepted unsupported guide diagram: %s", code)
			}
		})
	}
}
