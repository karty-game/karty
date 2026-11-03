package levelbuild

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	worldlightmapbake "github.com/karty-game/karty-sdk/bake/worldlightmap"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
	"github.com/karty-game/karty-sdk/format/worldterrain"
)

// Physical modules retain collision, probes and terrain as one authored owner.
// Their empty render selection has no vertex, index or RNM runtime payload.
func encodeGPUPhysicalChunk(
	ctx context.Context,
	document world.Document,
	name string,
	chunk worldchunk.PlannedChunk,
	materials []worldlightmapbake.Material,
	lighting worldlightmap.Options,
	options worldlightmapbake.Options,
	cache *chunkPixelCache,
	development bool,
) (ChunkArtifact, worldchunk.Chunk, error) {
	const emptyPhysicalPage = 32

	lighting.PageSize = emptyPhysicalPage
	lighting.MaxTexels = emptyPhysicalPage * emptyPhysicalPage

	layout, err := worldlightmap.CompileSelectedReceivers(document, lighting, chunk.Sectors, chunk.Solids, worldlightmap.SurfaceSelection{})
	if err != nil {
		return ChunkArtifact{}, worldchunk.Chunk{}, err
	}

	encodedLayout, err := worldlightmap.Encode(layout, &document)
	if err != nil {
		return ChunkArtifact{}, worldchunk.Chunk{}, err
	}

	if development {
		key, err := worldlightmapbake.FixedInputDigest(document, layout, materials, options)
		if err != nil {
			return ChunkArtifact{}, worldchunk.Chunk{}, err
		}

		if _, _, found := readFixedChunkPixels(cache, key, layout, &document, false); !found {
			return ChunkArtifact{}, worldchunk.Chunk{}, fmt.Errorf(
				"physical probes require a completed offline bake; run karty build: %w",
				ErrManifest,
			)
		}
	}

	fixed, _, err := fixedChunkLighting(ctx, document, layout, materials, options, cache, false)
	if err != nil {
		return ChunkArtifact{}, worldchunk.Chunk{}, err
	}

	if err := worldlightmap.ValidateFixedLightingMetadata(fixed, layout); err != nil {
		return ChunkArtifact{}, worldchunk.Chunk{}, err
	}

	encodedFixed, err := worldlightmap.EncodeFixedLighting(fixed)
	if err != nil {
		return ChunkArtifact{}, worldchunk.Chunk{}, err
	}

	collision, reservation, err := compilePrimaryChunkCollision(&document, chunk.Sectors, chunk.Solids)
	if err != nil {
		return ChunkArtifact{}, worldchunk.Chunk{}, err
	}

	identity := level.Metadata{
		worldchunk.ChunkMetadataKey:    level.StringValue(worldchunk.PhysicalSchema),
		worldlightmap.MetadataKey:      level.StringValue(worldlightmap.SelectedSchema),
		worldlightmap.FixedMetadataKey: level.StringValue(fixed.Schema),
		"chunkID":                      level.UintValue(uint64(chunk.ID)), "geometrySha256": level.StringValue(chunk.Mesh.GeometrySHA256),
	}
	entries := []level.SourceEntry{
		{Name: worldchunk.CollisionEntryName, Kind: level.EntryData, Data: collision},
		{Name: worldlightmap.EntryName, Kind: level.EntryData, Data: encodedLayout},
		{Name: worldlightmap.FixedEntryName, Kind: level.EntryData, Data: encodedFixed},
	}

	var terrainBytes uint64

	if options.TerrainReceivers != nil {
		encoded, err := worldterrain.Encode(*options.TerrainReceivers)
		if err != nil {
			return ChunkArtifact{}, worldchunk.Chunk{}, err
		}

		if err := worldlightmap.ValidateFixedTerrainBinding(fixed, options.TerrainReceivers); err != nil {
			return ChunkArtifact{}, worldchunk.Chunk{}, err
		}

		entries = append(entries, level.SourceEntry{Name: worldterrain.EntryName, Kind: level.EntryData, Data: encoded})
		identity[worldterrain.MetadataKey] = level.StringValue(worldterrain.Schema)
		terrainBytes = worldterrain.ResidentGeometryBytes(options.TerrainReceivers)
	}

	metadata, err := level.EncodeMetadata(identity)
	if err != nil {
		return ChunkArtifact{}, worldchunk.Chunk{}, err
	}

	module, err := encodeLevelModule(name, metadata, entries)
	if err != nil {
		return ChunkArtifact{}, worldchunk.Chunk{}, err
	}

	digest := sha256.Sum256(module)
	checksum := hex.EncodeToString(digest[:])

	return ChunkArtifact{ID: chunk.ID, Bytes: module, ContentSHA256: checksum, EnvelopeVersion: level.EnvelopeVersion}, worldchunk.Chunk{
		ID:             chunk.ID,
		Sectors:        chunk.Sectors,
		Solids:         chunk.Solids,
		ContentSHA256:  checksum,
		Size:           uint64(len(module)),
		CollisionBytes: reservation,
		LightingBytes:  worldlightmap.FixedLightingBytes(&fixed),
		TerrainBytes:   terrainBytes,
		Bounds:         chunk.Bounds,
	}, nil
}

func (build *gpuWorldBuild) buildPhysics(ctx context.Context, result *worldMaterialBuild) (bool, error) {
	if build.definition.Lightmap == nil || !build.definition.Lightmap.Fixed {
		return false, nil
	}

	geometry, err := worldchunk.GeometryDigest(&build.document)
	if err != nil {
		return false, err
	}

	catalog := worldchunk.Catalog{
		Schema:         worldchunk.PhysicalSchema,
		GeometrySHA256: geometry,
		SectorChunks:   build.physical.SectorChunks,
		Candidates:     build.physical.Candidates,
	}

	terrain, err := partitionWorldChunkTerrain(build.terrain, &build.document, build.physical.Chunks)
	if err != nil {
		return false, err
	}

	baked := make(map[string]bool)

	for _, chunk := range build.physical.Chunks {
		options := build.bake
		options.TerrainReceivers = terrain[chunk.ID]

		artifact, descriptor, err := encodeGPUPhysicalChunk(
			ctx,
			build.document,
			build.definition.Level.Name,
			chunk,
			build.bakeMaterials,
			build.lighting,
			options,
			build.cache,
			build.assets.development,
		)
		if err != nil {
			return false, err
		}

		result.chunks = append(result.chunks, artifact)
		catalog.Chunks = append(catalog.Chunks, descriptor)

		envelope, err := level.DecodePassiveModule(artifact.Bytes)
		if err != nil {
			return false, err
		}

		layout, _, _ := envelope.Read(worldlightmap.EntryName, 0, worldlightmap.MaxEncodedSize)
		if err := addBakedLightIDs(baked, &build.document, layout); err != nil {
			return false, err
		}
	}

	appendCatalogBakedLights(&catalog, &build.document, baked)

	encoded, err := worldchunk.EncodeCatalog(catalog, &build.document)
	if err != nil {
		return false, err
	}

	result.data = append(result.data, level.SourceEntry{Name: worldchunk.CatalogEntryName, Kind: level.EntryData, Data: encoded})

	identity, err := level.DecodeMetadata(result.metadata)
	if err != nil {
		return false, err
	}

	identity[worldchunk.MetadataKey] = level.StringValue(worldchunk.PhysicalSchema)
	identity[worldlightmap.FixedMetadataKey] = level.StringValue(worldlightmap.FixedSchema)

	result.metadata, err = level.EncodeMetadata(identity)
	if err != nil {
		return false, err
	}

	result.features = append(result.features, worldchunk.Feature, worldlightmap.Feature, worldlightmap.FixedFeature)

	return true, nil
}
