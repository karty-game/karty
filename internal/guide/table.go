package guide

import (
	"fmt"
	"regexp"
	"strings"
)

const tableHeaderRows = 2

func (renderer *markdownRenderer) tableRow(line string) error {
	if err := renderer.flushParagraph(); err != nil {
		return err
	}

	renderer.closeList()

	line = strings.TrimSpace(line)
	if len(line) < 2 || !strings.HasSuffix(line, "|") {
		return fmt.Errorf("guide table rows require outer pipes: %w", ErrMarkdown)
	}

	cells := strings.Split(line[1:len(line)-1], "|")
	for index, cell := range cells {
		cells[index] = strings.TrimSpace(cell)
	}

	renderer.table = append(renderer.table, cells)

	return nil
}

func (renderer *markdownRenderer) flushTable() error {
	if len(renderer.table) == 0 {
		return nil
	}

	if len(renderer.table) < tableHeaderRows {
		return fmt.Errorf("guide table requires a separator row: %w", ErrMarkdown)
	}

	columns := len(renderer.table[0])
	separator := regexp.MustCompile(`^:?-{3,}:?$`)

	for index, row := range renderer.table {
		if len(row) != columns {
			return fmt.Errorf("guide table has inconsistent columns: %w", ErrMarkdown)
		}

		if index == 1 {
			for _, cell := range row {
				if !separator.MatchString(cell) {
					return fmt.Errorf("invalid guide table separator: %w", ErrMarkdown)
				}
			}
		}
	}

	renderer.output.WriteString(
		`<p class="table-hint">Scroll to see all columns.</p><div class="table-wrap" tabindex="0" role="region" aria-label="Scrollable table"><table><thead>`,
	)

	if err := renderer.writeTableRow(renderer.table[0], "th scope=\"col\"", "th"); err != nil {
		return err
	}

	renderer.output.WriteString("</thead><tbody>")

	for _, row := range renderer.table[tableHeaderRows:] {
		if err := renderer.writeTableRow(row, "td", "td"); err != nil {
			return err
		}
	}

	renderer.output.WriteString("</tbody></table></div>\n")
	renderer.table = nil

	return nil
}

func (renderer *markdownRenderer) writeTableRow(cells []string, opening, closing string) error {
	renderer.output.WriteString("<tr>")

	for _, cell := range cells {
		text, err := renderInline(cell)
		if err != nil {
			return err
		}

		renderer.output.WriteString("<" + opening + ">" + text + "</" + closing + ">")
	}

	renderer.output.WriteString("</tr>")

	return nil
}
