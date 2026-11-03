package levelbuild

import (
	"github.com/karty-game/karty-sdk/format/worldmaterialssbump"
)

// Package only a completely validated pair bound to the final atlas layout.
// A chart producer must supply already reprojected coefficients, never ask this
// packaging boundary to infer them from unrelated packed height-chart cells.

func ownedSSBumpEntry(name string) bool {
	return name == worldmaterialssbump.DescriptorEntry || name == worldmaterialssbump.AtlasEntry || name == worldmaterialssbump.MipTailEntry
}
