// Package sitetheme provides the shared, offline visual style for Karty samples
// and the game authoring generator.
package website

import _ "embed"

//go:embed styles/theme.css
var stylesheet string

// CSS returns the handwritten stylesheet embedded in the website tools.
func CSS() string {
	return stylesheet
}
