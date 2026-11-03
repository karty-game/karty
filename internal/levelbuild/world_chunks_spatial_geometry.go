package levelbuild

import (
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
	"github.com/karty-game/karty-sdk/format/worldterrain"
)

// Spatial chunks reserve decoded render, physical and terrain data separately.
// Their combined budget may exceed a single encoded entry's unchanged bound.
const maxSpatialGeometryBytes = 16 << 20

// Preflight complete geometry before tracing rays. Encoded lighting size still
// needs the final post-bake module check, but an already-known geometry overflow
// must not trigger a costly bake before selecting a smaller primary group.

func preflightSpatialGeometryDescriptor(chunk *worldchunk.PlannedChunk, primary bool, document *world.Document,
	terrain *worldterrain.Document, ready *worldchunk.ReadyOptions,
) (worldchunk.Chunk, error) {
	descriptor := worldchunk.Chunk{MeshBytes: worldchunk.DecodedMeshBytes(chunk.Mesh)}
	if chunk.Layout.Schema != "" {
		encoded, err := worldlightmap.Encode(chunk.Layout, document)
		if err != nil {
			return descriptor, err
		}

		descriptor.Size += uint64(len(encoded))
	}

	if ready != nil {
		reservation, encodedBytes, err := preflightSpatialReady(chunk, document, ready)
		if err != nil {
			return descriptor, err
		}

		descriptor.MeshBytes = reservation
		descriptor.Size += encodedBytes
	} else {
		encoded, err := worldchunk.EncodeMesh(chunk.Mesh, document)
		if err != nil {
			return descriptor, err
		}

		descriptor.Size += uint64(len(encoded))
	}

	if primary {
		encoded, reservation, err := compilePrimaryChunkCollision(document, chunk.Sectors, chunk.Solids)
		if err != nil {
			return descriptor, err
		}

		descriptor.CollisionBytes = reservation
		descriptor.Size += uint64(len(encoded))
	}

	if terrain != nil {
		descriptor.TerrainBytes = worldterrain.ResidentGeometryBytes(terrain)

		encoded, err := worldterrain.Encode(*terrain)
		if err != nil {
			return descriptor, err
		}

		if len(encoded) > level.MaxEntrySize {
			return descriptor, level.ErrEnvelopeSize
		}

		descriptor.Size += uint64(len(encoded))
	}

	return descriptor, nil
}

func preflightSpatialReady(
	chunk *worldchunk.PlannedChunk,
	document *world.Document,
	options *worldchunk.ReadyOptions,
) (uint64, uint64, error) {
	encoded, err := worldchunk.EncodeMesh(chunk.Mesh, document)
	if err != nil {
		return 0, 0, err
	}

	var layout *worldlightmap.Layout
	if chunk.Layout.Schema != "" {
		layout = &chunk.Layout
	}

	ready, err := worldchunk.CompileReady(&chunk.Mesh, encoded, document, layout, *options)
	if err != nil {
		return 0, 0, err
	}

	encoded, err = worldchunk.EncodeReady(ready)
	if err != nil {
		return 0, 0, err
	}

	if len(encoded) > level.MaxEntrySize {
		return 0, 0, level.ErrEnvelopeSize
	}

	return readyChunkMeshBytes(&ready), uint64(len(encoded)), nil
}
