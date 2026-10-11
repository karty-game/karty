package levelbuild

import (
	"slices"

	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

func chunkDescriptorWithinOptions(child worldchunk.Chunk, options worldchunk.Options, groups []int) bool {
	if child.MeshBytes > options.MaxMeshBytes || child.Size > options.MaxEncodedBytes || len(child.Sectors) > options.MaxSectors ||
		options.NoLightmap != (child.LightmapBytes == 0) || child.LightmapBytes > uint64(options.Lightmap.MaxTexels)*12 {
		return false
	}

	if len(groups) != 0 {
		group := groups[child.Sectors[0]]
		for _, sector := range child.Sectors {
			if groups[sector] != group {
				return false
			}
		}
	}

	return true
}

func validateWorldChunkCacheChild(
	module []byte,
	child worldchunk.Chunk,
	catalog *worldchunk.Catalog,
	document *world.Document,
	noLightmap bool,
	settings ...*lightmapBuildSettings,
) error {
	envelope, err := level.DecodePassiveModule(module)
	if err != nil {
		return err
	}

	metadata, err := level.DecodeMetadata(envelope.Metadata)
	if err != nil {
		return err
	}

	chunkID, chunkIDValid := metadata.Uint("chunkID")
	if !chunkMetadataMatches(metadata, worldchunk.ChunkMetadataKey, worldchunk.Schema) ||
		!chunkMetadataMatches(metadata, "geometrySha256", catalog.GeometrySHA256) ||
		!chunkIDValid || chunkID != uint64(child.ID) {
		return worldchunk.ErrChunk
	}

	entries, err := readWorldChunkCacheEntries(envelope)
	if err != nil {
		return err
	}

	mesh, err := worldchunk.DecodeMesh(entries[worldchunk.MeshEntryName], document)
	if err != nil || !slices.Equal(mesh.Sectors, child.Sectors) || !slices.Equal(mesh.Solids, child.Solids) ||
		mesh.Bounds != child.Bounds || mesh.GeometrySHA256 != catalog.GeometrySHA256 || worldchunk.DecodedMeshBytes(mesh) != child.MeshBytes {
		return worldchunk.ErrChunk
	}

	if noLightmap {
		if child.LightmapBytes != 0 || len(entries) != 1 || chunkMetadataPresent(metadata, worldlightmap.MetadataKey) ||
			chunkMetadataPresent(metadata, worldlightmap.PrebakeMetadataKey) {
			return worldchunk.ErrChunk
		}

		return worldchunk.ValidateMeshBinding(&mesh, nil)
	}

	return validateWorldChunkCacheLightmap(entries, metadata, &mesh, child, catalog, document, settings...)
}

func validateWorldChunkCacheLightmap(entries map[string][]byte, metadata level.Metadata, mesh *worldchunk.Mesh,
	child worldchunk.Chunk, catalog *worldchunk.Catalog, document *world.Document, settings ...*lightmapBuildSettings) error {
	if child.LightmapBytes == 0 || len(entries) != 4 ||
		!chunkMetadataMatches(metadata, worldlightmap.MetadataKey, worldlightmap.Schema) ||
		!chunkMetadataMatches(metadata, worldlightmap.PrebakeMetadataKey, worldlightmap.PrebakeSchema) {
		return worldchunk.ErrChunk
	}

	layout, err := worldlightmap.Decode(entries[worldlightmap.EntryName], document)
	if err != nil {
		return err
	}

	if err := worldchunk.ValidateBakedSelection(catalog, child.ID, document, &layout); err != nil {
		return err
	}

	if err := worldchunk.ValidateMeshBinding(mesh, &layout); err != nil {
		return err
	}

	pair, err := worldlightmap.DecodePrebake(
		entries[worldlightmap.PrebakeEntryName],
		entries[worldlightmap.PrebakeImageEntryName],
		layout,
		document,
	)
	if err != nil {
		return err
	}

	if uint64(pair.Manifest.Width)*uint64(pair.Manifest.Height)*4 != child.LightmapBytes {
		return worldchunk.ErrChunk
	}

	if len(settings) != 0 {
		options := chunkBakeOptions(settings[0])

		producer, err := worldlightmap.OfflineProducer(options.Denoise, options.MaxRayDistance)
		if err != nil || pair.Manifest.Algorithm != worldlightmap.OfflinePrebakeAlgorithm || pair.Manifest.Producer != producer ||
			pair.Manifest.MaxRayDistance != options.MaxRayDistance || pair.Manifest.Samples != options.Samples ||
			pair.Manifest.Bounces != options.Bounces || pair.Manifest.Seed != options.Seed {
			return worldchunk.ErrChunk
		}
	}

	return nil
}

func chunkMetadataMatches(metadata level.Metadata, key, expected string) bool {
	value, ok := metadata.String(key)

	return ok && value == expected
}
func chunkMetadataPresent(metadata level.Metadata, key string) bool {
	_, ok := metadata[key]

	return ok
}

func readWorldChunkCacheEntries(envelope level.Envelope) (map[string][]byte, error) {
	if len(envelope.Entries) != 1 && len(envelope.Entries) != 4 {
		return nil, worldchunk.ErrChunk
	}

	entries := make(map[string][]byte, len(envelope.Entries))
	for _, entry := range envelope.Entries {
		if entry.Kind != level.EntryData {
			return nil, worldchunk.ErrChunk
		}

		switch entry.Name {
		case worldchunk.MeshEntryName, worldlightmap.EntryName, worldlightmap.PrebakeEntryName, worldlightmap.PrebakeImageEntryName:
		default:
			return nil, worldchunk.ErrChunk
		}

		data, _, found := envelope.Read(entry.Name, 0, entry.Length)
		if !found {
			return nil, worldchunk.ErrChunk
		}

		entries[entry.Name] = data
	}

	return entries, nil
}
