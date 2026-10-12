// Package sitetheme provides the shared, offline visual style for Karty samples
// and the game authoring guide.
package sitetheme

import _ "embed"

//go:embed theme.css
var stylesheet string

// CSS returns the handwritten stylesheet embedded in the CLI.
func CSS() string {
	return stylesheet
}
