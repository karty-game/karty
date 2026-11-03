package levelbuild

import (
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldterrain"
	"github.com/karty-game/karty/internal/assetpipeline"
)

// ChunkArtifact is a content-addressed passive child cartridge. Its identity
// and exact byte length are bound by the parent level's chunk catalog.
type ChunkArtifact struct {
	// CompactLighting is derived from the completed, validated child artifact.
	CompactLighting bool
	ID              int
	Bytes           []byte
	ContentSHA256   string
	EnvelopeVersion uint16
}

type worldMaterialBuild struct {
	data           []level.SourceEntry
	metadata       []byte
	textures       []assetpipeline.Texture
	features       []string
	chunks         []ChunkArtifact
	sources        []level.SourceEntry
	chartSourceIDs []uint32
	chartGrid      int
	ready          *worldchunk.ReadyOptions
	terrain        *worldterrain.Document
}
