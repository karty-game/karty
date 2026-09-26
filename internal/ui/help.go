package ui

import (
	"bytes"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/urfave/cli/v3"
)

// ConfigureHelp installs terminal-aware styling around urfave's default help renderer.
func ConfigureHelp() {
	cli.HelpPrinter = printHelp
}

func printHelp(output io.Writer, template string, data any) {
	var plain bytes.Buffer
	cli.DefaultPrintHelp(&plain, template, data)

	renderer := newHelpRenderer(output)
	styled := styleHelp(plain.String(), renderer)
	_, _ = io.WriteString(output, styled)
}

func newHelpRenderer(output io.Writer) *lipgloss.Renderer {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return lipgloss.NewRenderer(output)
	}

	return lipgloss.NewRenderer(output, termenv.WithTTY(true), termenv.WithProfile(termenv.ANSI256))
}

func styleHelp(help string, renderer *lipgloss.Renderer) string {
	heading := renderer.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{
		Light: "#9A6700",
		Dark:  "#E5B567",
	})
	name := renderer.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{
		Light: "#145A7A",
		Dark:  "#88B6D3",
	})

	var styled strings.Builder

	section := ""

	for _, line := range strings.SplitAfter(help, "\n") {
		text := strings.TrimSuffix(line, "\n")
		newline := line[len(text):]
		trimmed := strings.TrimSpace(text)

		if strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(text, " ") {
			section = strings.TrimSuffix(trimmed, ":")
			styled.WriteString(heading.Render("◆ " + trimmed))
			styled.WriteString(newline)

			continue
		}

		if isHelpNameSection(section) {
			styled.WriteString(styleHelpName(text, name))
		} else {
			styled.WriteString(text)
		}

		styled.WriteString(newline)
	}

	return styled.String()
}

func isHelpNameSection(section string) bool {
	switch section {
	case "NAME", "USAGE", "COMMANDS", "OPTIONS", "GLOBAL OPTIONS":
		return true
	default:
		return false
	}
}

func styleHelpName(line string, style lipgloss.Style) string {
	leadingLength := len(line) - len(strings.TrimLeft(line, " \t"))
	leading := line[:leadingLength]

	rest := line[leadingLength:]
	if rest == "" {
		return line
	}

	nameLength := len(rest) - len(strings.TrimLeft(rest, " \t"))
	nameStart := nameLength

	nameEnd := nameStart

	for nameEnd < len(rest) && rest[nameEnd] != ' ' && rest[nameEnd] != '\t' {
		nameEnd++
	}

	if nameStart == nameEnd {
		return line
	}

	return leading + rest[:nameStart] + style.Render(rest[nameStart:nameEnd]) + rest[nameEnd:]
}
