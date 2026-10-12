package guide

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

type journeyNode struct {
	identity, label string
}

// renderJourney draws the guide's small Mermaid TD chain as offline SVG. It
// rejects other graph syntax instead of guessing at a general Mermaid layout.
//
//nolint:mnd // These constants are the fixed SVG drawing coordinates and limits.
func renderJourney(code string) (string, error) {
	lines := strings.Split(strings.TrimSpace(code), "\n")
	if len(lines) < 2 || lines[0] != "flowchart TD" {
		return "", fmt.Errorf("guide diagrams require a flowchart TD chain: %w", ErrMarkdown)
	}

	var nodes []journeyNode

	pattern := regexp.MustCompile(`^([A-Za-z][A-Za-z0-9]*)(?:\[([^\]]+)\])? --> ([A-Za-z][A-Za-z0-9]*)\[([^\]]+)\]$`)

	for _, line := range lines[1:] {
		match := pattern.FindStringSubmatch(strings.TrimSpace(line))
		if match == nil {
			return "", fmt.Errorf("unsupported guide diagram edge %q: %w", line, ErrMarkdown)
		}

		if len(nodes) == 0 {
			if match[2] == "" {
				return "", fmt.Errorf("guide diagram starts without a label: %w", ErrMarkdown)
			}

			nodes = append(nodes, journeyNode{identity: match[1], label: match[2]})
		} else if previous := nodes[len(nodes)-1]; previous.identity != match[1] ||
			match[2] != "" && match[2] != previous.label {
			return "", fmt.Errorf("guide diagram must be one continuous chain: %w", ErrMarkdown)
		}

		for _, node := range nodes {
			if node.identity == match[3] {
				return "", fmt.Errorf("guide diagram repeats a node: %w", ErrMarkdown)
			}
		}

		nodes = append(nodes, journeyNode{identity: match[3], label: match[4]})
		if len(nodes) > 8 {
			return "", fmt.Errorf("guide diagram exceeds eight steps: %w", ErrMarkdown)
		}
	}

	var output strings.Builder

	fmt.Fprintf(
		&output,
		`<figure class="journey"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 %d" role="img" aria-label="Game-making steps">`,
		len(nodes)*64,
	)

	for index, node := range nodes {
		vertical := 12 + index*64
		fmt.Fprintf(
			&output,
			`<rect x="20" y="%d" width="360" height="40" rx="8" fill="#f1f1e9" stroke="#deded1"/><text x="200" y="%d" text-anchor="middle" fill="#243331" font-family="system-ui,sans-serif" font-size="15">%s</text>`,
			vertical,
			vertical+25,
			html.EscapeString(node.label),
		)

		if index < len(nodes)-1 {
			fmt.Fprintf(&output, `<path d="M200 %dV%dM195 %dL200 %dL205 %d" fill="none" stroke="#9d3d19" stroke-width="2"/>`,
				vertical+44, vertical+60, vertical+55, vertical+60, vertical+55)
		}
	}

	output.WriteString("</svg></figure>\n")

	return output.String(), nil
}
