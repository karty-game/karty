// Package skills embeds the agent guidance included in new game projects.
package skills

import "embed"

// Files contains the maintained skills shipped with the CLI.
//
//go:embed karty-game
var Files embed.FS
