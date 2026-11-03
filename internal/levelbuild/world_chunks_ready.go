package levelbuild

import (
	"github.com/karty-game/karty-sdk/format/worldchunk"
)

const chunkTriangleMeshBytes = 484

// Keep the established catalog reservation/count unit, including any triangles
// introduced by the offline cleanup rather than the canonical proof input.
func readyChunkMeshBytes(ready *worldchunk.Ready) uint64 {
	var triangles uint64
	for _, batch := range ready.Batches {
		triangles += uint64(len(batch.Sources))
	}

	return triangles * chunkTriangleMeshBytes
}
