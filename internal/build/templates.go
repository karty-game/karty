package build

import _ "embed"

//go:embed templates/index.html.tmpl
var webIndexTemplate string

//go:embed templates/launcher.js.tmpl
var webLauncherTemplate string
