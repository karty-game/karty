package generator

import (
	"fmt"
	"html"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Code and links are consumed as complete spans so their contents and link
// destinations cannot accidentally open or close surrounding bold text.
func renderInlineSpan(text string) (string, string, error) {
	if text[0] == '`' {
		end := strings.IndexByte(text[1:], '`')
		if end < 0 {
			return "", "", fmt.Errorf("unclosed inline code: %w", ErrMarkdown)
		}

		code := text[1 : end+1]

		opening := "<code>"
		if strings.HasPrefix(code, "--") {
			opening = `<code class="command-flag">`
		}

		return opening + html.EscapeString(code) + "</code>", text[end+2:], nil
	}

	labelEnd := strings.Index(text, "](")
	if labelEnd < 0 {
		return "[", text[1:], nil
	}

	urlEnd := strings.IndexByte(text[labelEnd+2:], ')')
	if urlEnd < 0 {
		return "", "", fmt.Errorf("unclosed link: %w", ErrMarkdown)
	}

	target := text[labelEnd+2 : labelEnd+2+urlEnd]
	if err := validateLink(target); err != nil {
		return "", "", err
	}

	label, err := renderInline(text[1:labelEnd])
	if err != nil {
		return "", "", err
	}

	return `<a href="` + html.EscapeString(target) + `">` + label + "</a>", text[labelEnd+urlEnd+3:], nil
}

func writeStrongMarker(output *strings.Builder, text string, opening *int) string {
	if !strings.HasPrefix(text, "**") {
		output.WriteByte('*')

		return text[1:]
	}

	if *opening >= 0 {
		previous, _ := utf8.DecodeLastRuneInString(output.String())
		if !unicode.IsSpace(previous) {
			output.WriteString("</strong>")

			*opening = -1

			return text[2:]
		}
	} else if next, _ := utf8.DecodeRuneInString(text[2:]); len(text) > 2 && next != '*' && !unicode.IsSpace(next) {
		*opening = output.Len()
		output.WriteString("<strong>")

		return text[2:]
	}

	output.WriteString("**")

	return text[2:]
}

func validateLink(target string) error {
	parsed, err := url.Parse(target)
	if err != nil || target == "" || strings.ContainsAny(target, "\\\r\n\t ") {
		return fmt.Errorf("invalid guide link %q: %w", target, ErrMarkdown)
	}

	if parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil {
		return nil
	}

	if parsed.Scheme == "" && parsed.Host == "" && !strings.HasPrefix(target, "/") {
		return nil
	}

	return fmt.Errorf("unsupported guide link %q: %w", target, ErrMarkdown)
}
