// Package website owns the public site's content, templates and styles.
// Website tools embed these files; the CLI does not import this package.
package website

import "embed"

// Files contains handwritten site sources, independent of the process working directory.
//
//go:embed content/guides/*.md content/guides/chapters/*.md content/guides/tracks/*.md content/design/*.md content/api/*.md templates/*.html assets/*.jpg assets/*.png assets/*.svg assets/*.capture.json
var Files embed.FS
