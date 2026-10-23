package devserver

import _ "embed"

//go:embed spector.js
var spectorScript string

// Install the spy synchronously, before the host creates its WebGL context.
// Keep the helper inline so generation bases and workspace proxies cannot
// redirect its URL to a nonexistent generation asset.
func spectorScripts() string {
	return `<script src="https://cdn.jsdelivr.net/npm/spectorjs@0.9.33/dist/spector.bundle.js"></script>` +
		`<script>` + spectorScript + `</script>`
}
