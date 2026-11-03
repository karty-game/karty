package levelbuild

import (
	"github.com/karty-game/karty-sdk/format/level"
)

func chunkMetadataMatches(metadata level.Metadata, key, expected string) bool {
	value, ok := metadata.String(key)

	return ok && value == expected
}
