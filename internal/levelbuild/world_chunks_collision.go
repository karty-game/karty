package levelbuild

import (
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

// Collision is compiled from complete primary owners before their render
// surfaces are delegated. It deliberately has no lightmap or visibility cleanup:
// render residency and baked chart selection must never remove colliders.
func compilePrimaryChunkCollision(document *world.Document, sectors, solids []int) ([]byte, uint64, error) {
	mesh, err := worldchunk.CompileMesh(*document, sectors, solids, worldlightmap.Layout{})
	if err != nil {
		return nil, 0, err
	}

	encoded, err := worldchunk.EncodeMesh(mesh, document)
	if err != nil {
		return nil, 0, err
	}

	ready, err := worldchunk.CompileReady(&mesh, encoded, document, nil, worldchunk.ReadyOptions{})
	if err != nil {
		return nil, 0, err
	}

	encoded, err = worldchunk.EncodeReady(ready)
	if err != nil {
		return nil, 0, err
	}

	if len(encoded) > level.MaxEntrySize {
		return nil, 0, level.ErrEnvelopeSize
	}

	return encoded, readyChunkMeshBytes(&ready), nil
}
