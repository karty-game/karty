package levelbuild

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

const chunkTriangleMeshBytes = 484

func supportsReadyChunks(assets *assetBuild) bool {
	return assets != nil && slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.Capability(worldchunk.ReadyFeature))
}

func repartitionChunkSettings(definition manifest, limit int) manifest {
	settings := *definition.Chunks
	settings.MaxSectors = &limit
	definition.Chunks = &settings

	return definition
}

func chunkArtifactLimit(descriptor worldchunk.Chunk, chunk worldchunk.PlannedChunk, options worldchunk.Options,
	ready bool, buildErr error,
) (bool, error) {
	if buildErr != nil {
		return ready && (errors.Is(buildErr, level.ErrEnvelopeSize) || errors.Is(buildErr, worldchunk.ErrReadyCapacity)) &&
			len(chunk.Sectors) > 1, buildErr
	}

	if descriptor.Size <= options.MaxEncodedBytes {
		if descriptor.MeshBytes <= options.MaxMeshBytes {
			return false, nil
		}

		return len(chunk.Sectors) > 1, fmt.Errorf("chunk %d sectors %v needs %d mesh bytes, budget %d: %w",
			chunk.ID, chunk.Sectors, descriptor.MeshBytes, options.MaxMeshBytes, ErrManifest)
	}

	return len(chunk.Sectors) > 1, fmt.Errorf("chunk %d sectors %v needs %d encoded bytes, budget %d: %w",
		chunk.ID, chunk.Sectors, descriptor.Size, options.MaxEncodedBytes, ErrManifest)
}

func prepareAndFinishWorldChunks(ctx context.Context, directory string, definition manifest, document world.Document,
	result worldMaterialBuild, assets *assetBuild,
) (worldMaterialBuild, error) {
	definition, cache, found, err := prepareWorldChunkCache(ctx, directory, definition, document, assets, result.ready)
	if err != nil {
		return worldMaterialBuild{}, err
	}

	return finishWorldChunks(ctx, directory, definition, document, result, assets, cache, found)
}

func prepareWorldChunkStreams(
	result worldMaterialBuild,
	chunks []worldchunk.PlannedChunk,
	document *world.Document,
) (worldMaterialBuild, error) {
	if result.ready == nil {
		return result, nil
	}

	meshes := make([]worldchunk.Mesh, len(chunks))
	for index := range chunks {
		meshes[index] = chunks[index].Mesh
	}

	surfaces, err := worldchunk.ReadySurfaces(meshes, document)
	if err != nil {
		return worldMaterialBuild{}, err
	}

	options := *result.ready
	options.Surfaces = surfaces
	result.ready = &options

	return result, nil
}

func appendWorldChunkStreams(entries []level.SourceEntry, metadata level.Metadata,
	mesh *worldchunk.Mesh, encodedMesh []byte, document *world.Document, layout *worldlightmap.Layout,
	options ...*worldchunk.ReadyOptions,
) ([]level.SourceEntry, uint64, error) {
	if len(options) == 0 || options[0] == nil {
		return entries, worldchunk.DecodedMeshBytes(*mesh), nil
	}

	ready, err := worldchunk.CompileReady(mesh, encodedMesh, document, layout, *options[0])
	if err != nil {
		return nil, 0, err
	}

	encoded, err := worldchunk.EncodeReady(ready)
	if err != nil {
		return nil, 0, err
	}

	if len(encoded) > level.MaxEntrySize {
		return nil, 0, fmt.Errorf(
			"prepared chunk stream needs %d bytes, entry bound %d: %w",
			len(encoded),
			level.MaxEntrySize,
			level.ErrEnvelopeSize,
		)
	}

	metadata[worldchunk.ReadyMetadataKey] = level.StringValue(worldchunk.ReadySchema)
	// The semantic mesh is an offline proof input. Shipping it would duplicate
	// geometry that the runtime already consumes directly from the ready entry.
	entries = slices.DeleteFunc(entries, func(entry level.SourceEntry) bool { return entry.Name == worldchunk.MeshEntryName })

	return append(entries, level.SourceEntry{Name: worldchunk.ReadyEntryName, Kind: level.EntryData, Data: encoded}),
		readyChunkMeshBytes(&ready), nil
}

// Keep the established catalog reservation/count unit, including any triangles
// introduced by the offline cleanup rather than the canonical proof input.
func readyChunkMeshBytes(ready *worldchunk.Ready) uint64 {
	var triangles uint64
	for _, batch := range ready.Batches {
		triangles += uint64(len(batch.Sources))
	}

	return triangles * chunkTriangleMeshBytes
}

func cachedWorldChunkStreams(chunks []ChunkArtifact, descriptors []worldchunk.Chunk, document *world.Document,
	options ...*worldchunk.ReadyOptions,
) (*worldchunk.ReadyOptions, []cachedWorldChunkGeometry, error) {
	if len(options) == 0 || options[0] == nil {
		return nil, nil, nil
	}

	if len(chunks) != len(descriptors) {
		return nil, nil, worldchunk.ErrChunk
	}

	meshes := make([]worldchunk.Mesh, len(chunks))

	prepared := make([]cachedWorldChunkGeometry, len(chunks))
	for index, chunk := range chunks {
		envelope, err := level.DecodePassiveModule(chunk.Bytes)
		if err != nil {
			return nil, nil, err
		}

		geometry, err := rebuildCachedWorldChunkGeometry(envelope, descriptors[index], document)
		if err != nil {
			return nil, nil, err
		}

		prepared[index] = geometry
		meshes[index] = geometry.mesh
	}

	surfaces, err := worldchunk.ReadySurfaces(meshes, document)
	if err != nil {
		return nil, nil, err
	}

	ready := *options[0]
	ready.Surfaces = surfaces

	return &ready, prepared, nil
}

func validateCachedWorldChunkStreams(entries map[string][]byte, mesh *worldchunk.Mesh,
	encodedMesh []byte, document *world.Document, layout *worldlightmap.Layout, options *worldchunk.ReadyOptions, meshBytes uint64,
) error {
	if options == nil {
		return nil
	}

	ready, err := worldchunk.DecodeReady(
		entries[worldchunk.ReadyEntryName],
		mesh,
		encodedMesh,
		document,
		layout,
		*options,
	)
	if err != nil {
		return err
	}

	if readyChunkMeshBytes(&ready) != meshBytes {
		return worldchunk.ErrChunk
	}

	return nil
}
