package project

import (
	"log/slog"
	"strings"

	uicompiler "github.com/karty-game/karty-ui/compiler"
	ui "github.com/karty-game/karty-ui/schema"
)

// ReportUIWarnings supports current KartUI diagnostics while remaining buildable
// with the released public compiler module during the candidate release cycle.
func ReportUIWarnings(components []uicompiler.Component) {
	seen := map[string]bool{}

	for _, component := range components {
		diagnostics, ok := any(&component).(interface{ StyleWarnings() []string })
		if !ok {
			continue
		}

		for _, warning := range diagnostics.StyleWarnings() {
			if !seen[warning] {
				slog.Warn(warning)
				seen[warning] = true
			}
		}
	}
}

// DecodeUIWithTheme retains the compiled component long enough to report style
// warnings. Static-template validation is identical to KartUI's decoder.
func DecodeUIWithTheme(source string, data []byte, theme uicompiler.Theme) (ui.Template, error) {
	if !strings.HasSuffix(source, ".kui") {
		return uicompiler.DecodeSourceWithTheme(source, data, theme)
	}

	component, err := uicompiler.CompileWithTheme(source, data, theme)
	if err != nil {
		return ui.Template{}, err
	}

	ReportUIWarnings([]uicompiler.Component{component})

	if decoder, ok := any(&component).(interface{ StaticTemplate() (ui.Template, error) }); ok {
		return decoder.StaticTemplate()
	}

	return uicompiler.DecodeSourceWithTheme(source, data, theme)
}
