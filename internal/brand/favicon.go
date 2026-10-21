// Package brand contains shared Karty artwork for the CLI and website.
package brand

import (
	_ "embed"
	"encoding/base64"
	"html/template"
)

//go:embed favicon.svg
var favicon []byte

// FaviconURL embeds the icon in the page so nested previews and offline games
// share the same artwork without another file to stage or cache.
func FaviconURL() template.URL {
	return template.URL(
		"data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(favicon),
	) // #nosec G203 -- only trusted embedded SVG is encoded.
}
