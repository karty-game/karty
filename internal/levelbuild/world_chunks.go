package levelbuild

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"

	worldlightmapbake "github.com/karty-game/karty-sdk/bake/worldlightmap"
	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

type chunkBuildSettings struct {
	Enabled           bool                `toml:"enabled"`
	MaxMeshBytes      *uint64             `toml:"max_mesh_bytes"`
	MaxLightmapTexels *int                `toml:"max_lightmap_texels"`
	MaxEncodedBytes   *uint64             `toml:"max_encoded_bytes"`
	MaxSectors        *int                `toml:"max_sectors"`
	Areas             []chunkAreaSettings `toml:"areas"`
}

const (
	defaultChunkMeshBytes    = 2 << 20
	defaultChunkEncodedBytes = 8 << 20
	defaultChunkSectors      = world.MaxSectors
	defaultChunkBakeSamples  = 16
)

func chunkOptions(definition manifest) (worldchunk.Options, error) {
	settings := definition.Chunks
	if settings == nil || !settings.Enabled {
		return worldchunk.Options{}, ErrManifest
	}

	var lightmap worldlightmap.Options

	noLightmap := definition.Lightmap == nil || !definition.Lightmap.Enabled
	if !noLightmap {
		var err error

		lightmap, err = definition.Lightmap.options()
		if err != nil {
			return worldchunk.Options{}, err
		}
	}

	if lightmap.PageSize == 0 {
		lightmap.PageSize = 512
	}

	lightmap.MaxPages = 1

	lightmap.MaxTexels = lightmap.PageSize * lightmap.PageSize
	if settings.MaxLightmapTexels != nil {
		if *settings.MaxLightmapTexels < lightmap.PageSize*lightmap.PageSize || *settings.MaxLightmapTexels > worldlightmap.MaxTexels {
			return worldchunk.Options{}, fmt.Errorf("max_lightmap_texels must fit a complete lightmap page: %w", ErrManifest)
		}

		lightmap.MaxTexels = *settings.MaxLightmapTexels
	}

	options := worldchunk.Options{
		MaxMeshBytes:    defaultChunkMeshBytes,
		MaxEncodedBytes: defaultChunkEncodedBytes,
		MaxSectors:      defaultChunkSectors,
		Lightmap:        lightmap,
		NoLightmap:      noLightmap,
	}

	if settings.MaxMeshBytes != nil {
		if *settings.MaxMeshBytes == 0 || *settings.MaxMeshBytes > level.MaxEntrySize {
			return worldchunk.Options{}, fmt.Errorf("max_mesh_bytes: %w", ErrManifest)
		}

		options.MaxMeshBytes = *settings.MaxMeshBytes
	}

	if settings.MaxEncodedBytes != nil {
		if *settings.MaxEncodedBytes == 0 || *settings.MaxEncodedBytes > 32<<20 {
			return worldchunk.Options{}, fmt.Errorf("max_encoded_bytes: %w", ErrManifest)
		}

		options.MaxEncodedBytes = *settings.MaxEncodedBytes
	}

	if settings.MaxSectors != nil {
		if *settings.MaxSectors < 1 || *settings.MaxSectors > world.MaxSectors {
			return worldchunk.Options{}, fmt.Errorf("max_sectors: %w", ErrManifest)
		}

		options.MaxSectors = *settings.MaxSectors
	}

	return options, nil
}

func validateChunkSettings(definition manifest, assets *assetBuild) error {
	if definition.Chunks == nil || !definition.Chunks.Enabled {
		return nil
	}

	if definition.World.Source == "" || assets == nil ||
		!slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.Capability(worldchunk.Feature)) {
		return fmt.Errorf("chunks require a world and SDK advertising %s: %w", worldchunk.Feature, ErrManifest)
	}

	if definition.Lightmap != nil && definition.Lightmap.Enabled && (!definition.Lightmap.Offline ||
		definition.Lightmap.Prebake != nil || definition.Lightmap.PrebakeImage != nil) {
		return fmt.Errorf("chunk lightmaps require offline = true; imported whole-world prebakes cannot be split: %w", ErrManifest)
	}

	if _, err := chunkOptions(definition); err != nil {
		return err
	}

	return nil
}

func buildWorldChunks(ctx context.Context, directory string, definition manifest, document world.Document,
	result worldMaterialBuild, pixelCaches ...*chunkPixelCache) (worldMaterialBuild, error) {
	// Keep the shared root intact if complete module sizes require a smaller
	// partition after baking. Planner estimates cannot predict QOI byte lengths.
	shared := result

	options, err := chunkOptions(definition)
	if err != nil {
		return worldMaterialBuild{}, err
	}

	options.SectorGroups, err = resolveChunkAreas(definition.Chunks.Areas, &document)
	if err != nil {
		return worldMaterialBuild{}, err
	}

	plan, err := worldchunk.Plan(document, options)
	if err != nil {
		return worldMaterialBuild{}, err
	}

	encodedProxy, err := encodeWorldChunkShadow(document)
	if err != nil {
		return worldMaterialBuild{}, err
	}

	var materials []worldlightmapbake.Material
	if !options.NoLightmap {
		_, materials, err = loadBakeMaterials(directory, definition)
		if err != nil {
			return worldMaterialBuild{}, err
		}
	}

	digest, err := worldchunk.GeometryDigest(&document)
	if err != nil {
		return worldMaterialBuild{}, err
	}

	catalog := worldchunk.Catalog{Schema: worldchunk.Schema, GeometrySHA256: digest,
		SectorChunks: plan.SectorChunks, Candidates: plan.Candidates}
	bakeOptions := chunkBakeOptions(definition.Lightmap)
	baked := make(map[string]bool)

	for _, chunk := range plan.Chunks {
		if err := ctx.Err(); err != nil {
			return worldMaterialBuild{}, err
		}

		artifact, descriptor, err := encodeWorldChunk(
			ctx,
			document,
			definition.Level.Name,
			chunk,
			materials,
			bakeOptions,
			options.NoLightmap,
			pixelCaches...,
		)
		if err != nil {
			return worldMaterialBuild{}, err
		}

		if descriptor.Size > options.MaxEncodedBytes {
			if len(chunk.Sectors) > 1 {
				settings := *definition.Chunks
				limit := len(chunk.Sectors) - 1
				settings.MaxSectors = &limit
				definition.Chunks = &settings

				return buildWorldChunks(ctx, directory, definition, document, shared, pixelCaches...)
			}

			return worldMaterialBuild{}, fmt.Errorf("chunk %d sectors %v needs %d encoded bytes, budget %d: %w",
				chunk.ID, chunk.Sectors, descriptor.Size, options.MaxEncodedBytes, ErrManifest)
		}

		result.chunks = append(result.chunks, artifact)

		catalog.Chunks = append(catalog.Chunks, descriptor)
		if descriptor.LightmapBytes != 0 && chunk.Layout.RuntimeBake != nil {
			for _, id := range chunk.Layout.RuntimeBake.LightIDs {
				baked[id] = true
			}
		}
	}

	appendCatalogBakedLights(&catalog, &document, baked)

	encodedCatalog, err := worldchunk.EncodeCatalog(catalog, &document)
	if err != nil {
		return worldMaterialBuild{}, err
	}

	entries := []level.SourceEntry{{Name: worldchunk.CatalogEntryName, Kind: level.EntryData, Data: encodedCatalog}}
	if encodedProxy != nil {
		entries = append(entries, level.SourceEntry{Name: worldchunk.ShadowEntryName, Kind: level.EntryData, Data: encodedProxy})
	}

	return applyWorldChunks(result, entries, result.chunks, definition)
}

// Fresh builds and validated caches use the same root ownership and metadata path.
func applyWorldChunks(
	result worldMaterialBuild,
	entries []level.SourceEntry,
	chunks []ChunkArtifact,
	definition manifest,
) (worldMaterialBuild, error) {
	for _, entry := range result.data {
		if strings.HasPrefix(entry.Name, "@world/chunks/") || strings.HasPrefix(entry.Name, "@world/lightmaps/") {
			return worldMaterialBuild{}, ErrSource
		}
	}

	identity, err := level.DecodeMetadata(result.metadata)
	if err != nil {
		return worldMaterialBuild{}, ErrManifest
	}

	if _, declared := identity[worldlightmap.MetadataKey]; declared {
		return worldMaterialBuild{}, fmt.Errorf("chunk roots cannot declare a whole-world lightmap: %w", ErrManifest)
	}

	if _, declared := identity[worldlightmap.PrebakeMetadataKey]; declared {
		return worldMaterialBuild{}, fmt.Errorf("chunk roots cannot declare a whole-world prebake: %w", ErrManifest)
	}

	identity[worldchunk.MetadataKey] = level.StringValue(worldchunk.Schema)

	result.metadata, err = level.EncodeMetadata(identity)
	if err != nil {
		return worldMaterialBuild{}, err
	}

	result.data = append(result.data, entries...)
	result.chunks = chunks

	result.features = append(result.features, worldchunk.Feature)
	if definition.Lightmap != nil && definition.Lightmap.Enabled {
		result.features = append(result.features, worldlightmap.Feature, worldlightmap.PrebakeFeature)
	}

	return result, nil
}

func encodeWorldChunkShadow(document world.Document) ([]byte, error) {
	proxy, err := worldchunk.CompileShadowProxy(document)
	if errors.Is(err, worldchunk.ErrShadowBudget) {
		// Complete render chunks retain their larger independent geometry bound.
		// A host must reject shadow commands when this optional asset is absent.
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return worldchunk.EncodeShadowProxy(proxy, &document)
}

func chunkBakeOptions(settings *lightmapBuildSettings) worldlightmapbake.Options {
	options := worldlightmapbake.Options{Samples: defaultChunkBakeSamples, Bounces: 1, Seed: 1, Denoise: "medium"}

	if settings != nil {
		if settings.BakeSamples != nil {
			options.Samples = *settings.BakeSamples
		}

		if settings.BakeBounces != nil {
			options.Bounces = *settings.BakeBounces
		}

		options.Denoise = settings.denoise()
		options.MaxRayDistance = settings.maxRayDistance()
	}

	return options
}

func encodeWorldChunk(
	ctx context.Context, document world.Document, name string, chunk worldchunk.PlannedChunk,
	materials []worldlightmapbake.Material, options worldlightmapbake.Options, noLightmap bool,
	pixelCaches ...*chunkPixelCache,
) (ChunkArtifact, worldchunk.Chunk, error) {
	mesh, err := worldchunk.EncodeMesh(chunk.Mesh, &document)
	if err != nil {
		return ChunkArtifact{}, worldchunk.Chunk{}, err
	}

	identity := level.Metadata{
		worldchunk.ChunkMetadataKey: level.StringValue(worldchunk.Schema),
		"chunkID":                   level.UintValue(uint64(chunk.ID)),
		"geometrySha256":            level.StringValue(chunk.Mesh.GeometrySHA256),
	}
	entries := []level.SourceEntry{{Name: worldchunk.MeshEntryName, Kind: level.EntryData, Data: mesh}}

	var lightmapBytes uint64

	if !noLightmap {
		lightmaps, decodedBytes, err := bakeWorldChunkLightmap(ctx, document, chunk.Layout, materials, options, pixelCaches...)
		if err != nil {
			return ChunkArtifact{}, worldchunk.Chunk{}, fmt.Errorf("chunk %d bake: %w", chunk.ID, err)
		}

		lightmapBytes = decodedBytes
		identity[worldlightmap.MetadataKey] = level.StringValue(worldlightmap.Schema)
		identity[worldlightmap.PrebakeMetadataKey] = level.StringValue(worldlightmap.PrebakeSchema)

		entries = append(entries, lightmaps...)
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
	contentDigest := hex.EncodeToString(digest[:])
	artifact := ChunkArtifact{ID: chunk.ID, Bytes: module, ContentSHA256: contentDigest, EnvelopeVersion: level.EnvelopeVersion}
	descriptor := worldchunk.Chunk{ID: chunk.ID, Sectors: chunk.Sectors, Solids: chunk.Solids, ContentSHA256: contentDigest,
		Size: uint64(len(module)), MeshBytes: chunk.MeshBytes, LightmapBytes: lightmapBytes, Bounds: chunk.Bounds}

	return artifact, descriptor, nil
}

func bakeWorldChunkLightmap(
	ctx context.Context, document world.Document, layout worldlightmap.Layout,
	materials []worldlightmapbake.Material, options worldlightmapbake.Options,
	pixelCaches ...*chunkPixelCache,
) ([]level.SourceEntry, uint64, error) {
	encodedLayout, err := worldlightmap.Encode(layout, &document)
	if err != nil {
		return nil, 0, err
	}

	var cache *chunkPixelCache
	if len(pixelCaches) != 0 {
		cache = pixelCaches[0]
	}

	producer, err := worldlightmap.OfflineProducer(options.Denoise, options.MaxRayDistance)
	if err != nil {
		return nil, 0, err
	}

	rgbmRange, err := worldlightmap.OfflineRGBMRangeBounded(layout, &document, options.Bounces, options.MaxRayDistance)
	if err != nil {
		return nil, 0, err
	}

	var (
		key         string
		image       []byte
		reflectance string
		hit         bool
	)

	if cache != nil {
		key, err = worldlightmapbake.BakeInputDigest(document, layout, materials, options)
		if err != nil {
			return nil, 0, err
		}

		image, hit = cache.read(key, producer, rgbmRange)
		if hit {
			// Dimension checks follow complete QOI validation, before trusting a
			// cache hit. Wrong-size but valid pixels must trigger a cold bake.
			metadata, _ := qoi.Inspect(image)
			hit = len(layout.Pages) == 1 && int(metadata.Width) == 3*layout.Pages[0].Width &&
				int(metadata.Height) == layout.Pages[0].Height
		}
	}

	image, reflectance, rgbmRange, err = resolveChunkLightmapPixels(ctx, document, layout, materials, options, cache, hit, image, rgbmRange)
	if err != nil {
		return nil, 0, err
	}

	pair, err := worldlightmap.NewOfflinePrebake(layout, &document, image, worldlightmap.OfflineBakeInputs{
		ReflectanceSHA256: reflectance, Samples: options.Samples, Bounces: options.Bounces,
		Seed: options.Seed, RGBMRange: rgbmRange, Denoise: options.Denoise, MaxRayDistance: options.MaxRayDistance,
	})
	if err != nil {
		return nil, 0, err
	}

	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}

	if !hit {
		if err := cache.write(key, producer, rgbmRange, image); err != nil {
			return nil, 0, fmt.Errorf("write chunk pixel cache: %w", err)
		}
	}

	prebake, err := worldlightmap.EncodePrebake(pair, layout, &document)
	if err != nil {
		return nil, 0, err
	}

	decodedBytes := uint64(pair.Manifest.Width) * uint64(pair.Manifest.Height) * bakeTextureChannels
	entries := []level.SourceEntry{
		{Name: worldlightmap.EntryName, Kind: level.EntryData, Data: encodedLayout},
		{Name: worldlightmap.PrebakeEntryName, Kind: level.EntryData, Data: prebake},
		{Name: worldlightmap.PrebakeImageEntryName, Kind: level.EntryData, Data: image},
	}

	return entries, decodedBytes, nil
}

func appendCatalogBakedLights(catalog *worldchunk.Catalog, document *world.Document, baked map[string]bool) {
	if document.Lighting != nil {
		for index, light := range document.Lighting.Lights {
			if baked[light.ID] {
				catalog.BakedLights = append(catalog.BakedLights, index)
			}
		}
	}
}

func resolveChunkLightmapPixels(
	ctx context.Context,
	document world.Document,
	layout worldlightmap.Layout,
	materials []worldlightmapbake.Material,
	options worldlightmapbake.Options,
	cache *chunkPixelCache,
	hit bool,
	image []byte,
	rgbmRange float64,
) ([]byte, string, float64, error) {
	var (
		reflectance string
		err         error
	)
	if hit {
		reflectance, err = worldlightmapbake.ReflectanceDigestForLayout(document, layout, materials, options)
		if err != nil {
			return nil, "", 0, err
		}

		cache.hits++

		return image, reflectance, rgbmRange, nil
	}

	baked, bakeErr := worldlightmapbake.Bake(ctx, document, layout, materials, options)
	if bakeErr != nil {
		return nil, "", 0, bakeErr
	}

	image, _, err = qoi.Encode(baked.Image, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceLinear})
	if err != nil {
		return nil, "", 0, err
	}

	reflectance, rgbmRange = baked.ReflectanceSHA256, baked.RGBMRange

	if cache != nil {
		cache.misses++
	}

	return image, reflectance, rgbmRange, nil
}
