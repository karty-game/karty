package generator

import (
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/karty-game/karty/website"
)

// ErrMarkdown identifies unsupported or incomplete guide markup.
var ErrMarkdown = errors.New("invalid guide Markdown")

const chapterHeading = 2

type section struct {
	ID, Title string
}

type markdownRenderer struct {
	output          strings.Builder
	sections        []section
	paragraph, code []string
	language        string
	inCode, inList  bool
	identities      map[string]bool
	table           [][]string
}

// renderMarkdown supports the guide's headings, paragraphs, one-level bullets,
// fenced code, inline code, bold text, links and simple tables. It displays raw HTML as escaped text.
func renderMarkdown(markdown string) (string, []section, error) {
	renderer := markdownRenderer{identities: map[string]bool{}}
	for line := range strings.SplitSeq(markdown, "\n") {
		if err := renderer.consume(line); err != nil {
			return "", nil, err
		}
	}

	if renderer.inCode {
		return "", nil, fmt.Errorf("unclosed code block: %w", ErrMarkdown)
	}

	if err := renderer.flushParagraph(); err != nil {
		return "", nil, err
	}

	renderer.closeList()

	if err := renderer.flushTable(); err != nil {
		return "", nil, err
	}

	return renderer.output.String(), renderer.sections, nil
}

func (renderer *markdownRenderer) consume(line string) error {
	if renderer.inCode {
		return renderer.consumeCode(line)
	}

	if strings.HasPrefix(strings.TrimSpace(line), "|") {
		return renderer.tableRow(line)
	}

	if err := renderer.flushTable(); err != nil {
		return err
	}

	if strings.HasPrefix(line, "```") {
		if err := renderer.flushParagraph(); err != nil {
			return err
		}

		renderer.closeList()
		renderer.language = strings.TrimPrefix(line, "```")

		if !regexp.MustCompile(`^[a-z]*$`).MatchString(renderer.language) {
			return fmt.Errorf("unsupported code language %q: %w", renderer.language, ErrMarkdown)
		}

		renderer.inCode = true

		return nil
	}

	if strings.TrimSpace(line) == "" {
		if err := renderer.flushParagraph(); err != nil {
			return err
		}

		renderer.closeList()

		return nil
	}

	if strings.HasPrefix(line, "#") {
		return renderer.heading(line)
	}

	if strings.HasPrefix(line, "![") {
		return renderer.figure(line)
	}

	if item, found := strings.CutPrefix(line, "- "); found {
		return renderer.bullet(item)
	}

	renderer.closeList()
	renderer.paragraph = append(renderer.paragraph, strings.TrimSpace(line))

	return nil
}

func (renderer *markdownRenderer) consumeCode(line string) error {
	if line != "```" {
		renderer.code = append(renderer.code, line)

		return nil
	}

	code := strings.Join(renderer.code, "\n")
	if renderer.language == "mermaid" {
		diagram, err := renderJourney(code)
		if err != nil {
			return err
		}

		renderer.output.WriteString(diagram)
	} else {
		renderer.output.WriteString(
			"<pre><code class=\"language-" + renderer.language + "\">" + html.EscapeString(code) + "\n</code></pre>\n",
		)
	}

	renderer.inCode, renderer.code = false, nil

	return nil
}

func (renderer *markdownRenderer) figure(line string) error {
	if err := renderer.flushParagraph(); err != nil {
		return err
	}

	renderer.closeList()

	match := regexp.MustCompile(`^!\[([^\]]+)\]\((assets/[a-z0-9-]+\.(?:jpg|png|svg))\)$`).FindStringSubmatch(line)
	if match == nil {
		return fmt.Errorf("unsupported guide image %q: %w", line, ErrMarkdown)
	}

	if _, err := website.Files.ReadFile(match[2]); err != nil {
		return fmt.Errorf("missing guide image: %w", err)
	}

	fmt.Fprintf(&renderer.output,
		"<figure><img src=\"%s\" alt=\"%s\" loading=\"lazy\"><figcaption>%s</figcaption></figure>\n",
		html.EscapeString(match[2]), html.EscapeString(match[1]), html.EscapeString(match[1]))

	return nil
}

func (renderer *markdownRenderer) flushParagraph() error {
	if len(renderer.paragraph) == 0 {
		return nil
	}

	text, err := renderInline(strings.Join(renderer.paragraph, " "))
	if err != nil {
		return err
	}

	renderer.output.WriteString("<p>" + text + "</p>\n")
	renderer.paragraph = nil

	return nil
}

func (renderer *markdownRenderer) closeList() {
	if renderer.inList {
		renderer.output.WriteString("</ul>\n")
		renderer.inList = false
	}
}

func (renderer *markdownRenderer) heading(line string) error {
	if err := renderer.flushParagraph(); err != nil {
		return err
	}

	renderer.closeList()

	level := len(line) - len(strings.TrimLeft(line, "#"))
	if level < 1 || level > 3 || !strings.HasPrefix(line[level:], " ") {
		return fmt.Errorf("unsupported heading %q: %w", line, ErrMarkdown)
	}

	title := strings.TrimSpace(line[level:])
	identity := headingID(title)

	if identity == "" || renderer.identities[identity] {
		return fmt.Errorf("empty or duplicate heading %q: %w", title, ErrMarkdown)
	}

	renderer.identities[identity] = true
	fmt.Fprintf(&renderer.output, "<h%d id=\"%s\">%s</h%d>\n", level, identity, html.EscapeString(title), level)

	if level == chapterHeading {
		renderer.sections = append(renderer.sections, section{ID: identity, Title: title})
	}

	return nil
}

func (renderer *markdownRenderer) bullet(line string) error {
	if err := renderer.flushParagraph(); err != nil {
		return err
	}

	if !renderer.inList {
		renderer.output.WriteString("<ul>\n")
		renderer.inList = true
	}

	text, err := renderInline(line)
	if err != nil {
		return err
	}

	renderer.output.WriteString("<li>" + text + "</li>\n")

	return nil
}

func headingID(title string) string {
	var identity strings.Builder
	for _, char := range strings.ToLower(title) {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' {
			identity.WriteRune(char)
		} else if identity.Len() > 0 && !strings.HasSuffix(identity.String(), "-") {
			identity.WriteByte('-')
		}
	}

	return strings.TrimRight(identity.String(), "-")
}

func renderInline(text string) (string, error) {
	var output strings.Builder

	strong := -1

	for len(text) > 0 {
		special := strings.IndexAny(text, "`[*")
		if special < 0 {
			output.WriteString(html.EscapeString(text))

			break
		}

		output.WriteString(html.EscapeString(text[:special]))
		text = text[special:]

		if text[0] == '*' {
			text = writeStrongMarker(&output, text, &strong)

			continue
		}

		markup, remaining, err := renderInlineSpan(text)
		if err != nil {
			return "", err
		}

		output.WriteString(markup)

		text = remaining
	}

	markup := output.String()
	if strong >= 0 {
		// An unmatched delimiter remains literal, just like ordinary Markdown text.
		markup = markup[:strong] + "**" + markup[strong+len("<strong>"):]
	}

	return markup, nil
}
