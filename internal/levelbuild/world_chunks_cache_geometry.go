package levelbuild

import (
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

// Ready-only cache acceptance uses the full offline source oracle, even though
// runtime admission needs only the completed artifact and its catalog hash.
type cachedWorldChunkGeometry struct {
	mesh    worldchunk.Mesh
	encoded []byte
	layout  *worldlightmap.Layout
}

func rebuildCachedWorldChunkGeometry(envelope level.Envelope, descriptor worldchunk.Chunk,
	document *world.Document,
) (cachedWorldChunkGeometry, error) {
	entries, err := readWorldChunkCacheEntries(envelope)
	if err != nil || entries[worldchunk.MeshEntryName] != nil || len(entries[worldchunk.ReadyEntryName]) == 0 {
		return cachedWorldChunkGeometry{}, worldchunk.ErrChunk
	}

	var (
		layout   worldlightmap.Layout
		lightmap *worldlightmap.Layout
	)

	if encoded := entries[worldlightmap.EntryName]; encoded != nil {
		layout, err = worldlightmap.Decode(encoded, document)
		if err != nil {
			return cachedWorldChunkGeometry{}, err
		}

		lightmap = &layout
	}

	mesh, err := worldchunk.CompileMesh(*document, descriptor.Sectors, descriptor.Solids, layout)
	if err != nil {
		return cachedWorldChunkGeometry{}, err
	}

	encoded, err := worldchunk.EncodeMesh(mesh, document)
	if err != nil {
		return cachedWorldChunkGeometry{}, err
	}

	return cachedWorldChunkGeometry{mesh: mesh, encoded: encoded, layout: lightmap}, nil
}
