package levelbuild

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"

	worldlightmapbake "github.com/karty-game/karty-sdk/bake/worldlightmap"
	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
	"github.com/pelletier/go-toml/v2"
)

type environmentLightmapSettings struct {
	Times           []float64 `toml:"times"`
	DefaultTime     *float64  `toml:"default_time"`
	WeatherVariants bool      `toml:"weather_variants"`
}

func validateEnvironmentLightmapKeys(contents []byte) error {
	var raw struct {
		Lightmap struct {
			Environment map[string]any `toml:"environment"`
		} `toml:"lightmap"`
	}
	if err := toml.Unmarshal(contents, &raw); err != nil {
		return ErrManifest
	}

	for key := range raw.Lightmap.Environment {
		if !slices.Contains([]string{"times", "default_time", "weather_variants"}, key) {
			return fmt.Errorf("unknown lightmap environment option %q: %w", key, ErrManifest)
		}
	}

	return nil
}

func (settings environmentLightmapSettings) validate() error {
	endpoints := 1
	if settings.WeatherVariants {
		endpoints = 2
	}

	if len(settings.Times) == 0 || len(settings.Times)*endpoints > worldlightmap.MaxEnvironmentVariants {
		return ErrManifest
	}

	defaultTime := world.DefaultEnvironmentTimeOfDay
	if settings.DefaultTime != nil {
		defaultTime = *settings.DefaultTime
	}

	found := false

	for i, time := range settings.Times {
		if world.ValidateEnvironmentSettings(world.EnvironmentSettings{TimeOfDay: time}) != nil || i > 0 && time <= settings.Times[i-1] {
			return fmt.Errorf("environment times must be unique ascending hours in [0,24): %w", ErrManifest)
		}

		found = found || time == defaultTime
	}

	if !found {
		return fmt.Errorf("environment default_time must belong to times: %w", ErrManifest)
	}

	return nil
}

func encodeEnvironmentWorldChunk(ctx context.Context, document world.Document, name string, chunk worldchunk.PlannedChunk,
	materials []worldlightmapbake.Material, options worldlightmapbake.Options, settings environmentLightmapSettings,
	cache *chunkPixelCache) (ChunkArtifact, worldchunk.Chunk, []ChunkArtifact, error) {
	if err := settings.validate(); err != nil {
		return ChunkArtifact{}, worldchunk.Chunk{}, nil, err
	}

	// Fine foliage shadows are receiver-clipped runtime stamps. Opaque canopy
	// proxies would leave visible polygons in these lower-resolution lightmaps.

	mesh, err := worldchunk.EncodeMesh(chunk.Mesh, &document)
	if err != nil {
		return ChunkArtifact{}, worldchunk.Chunk{}, nil, err
	}

	layout, err := worldlightmap.Encode(chunk.Layout, &document)
	if err != nil {
		return ChunkArtifact{}, worldchunk.Chunk{}, nil, err
	}

	layoutHash, err := worldlightmap.LayoutDigest(&chunk.Layout)
	if err != nil {
		return ChunkArtifact{}, worldchunk.Chunk{}, nil, err
	}

	surfaceHash, err := worldlightmap.EnvironmentSurfaceDigest(&document)
	if err != nil {
		return ChunkArtifact{}, worldchunk.Chunk{}, nil, err
	}

	reflectanceHash, err := worldlightmapbake.EnvironmentReflectanceDigest(document, chunk.Layout, materials)
	if err != nil {
		return ChunkArtifact{}, worldchunk.Chunk{}, nil, err
	}

	catalog := worldlightmap.EnvironmentVariants{
		Schema:            worldlightmap.EnvironmentVariantsSchema,
		GeometrySHA256:    chunk.Layout.GeometrySHA256,
		LayoutSHA256:      layoutHash,
		SurfaceSHA256:     surfaceHash,
		ReflectanceSHA256: reflectanceHash,
		Producer:          worldlightmap.EnvironmentPrebakeProducer,
		Samples:           options.Samples,
		Bounces:           options.Bounces,
		Seed:              options.Seed,
		Denoise:           options.Denoise,
	}

	defaultTime := world.DefaultEnvironmentTimeOfDay
	if settings.DefaultTime != nil {
		defaultTime = *settings.DefaultTime
	}

	var (
		children     []ChunkArtifact
		defaultImage []byte
	)

	decoded := uint64(chunk.Layout.Pages[0].Width * chunk.Layout.Pages[0].Height * 12)

	clouds := []float64{0}
	if settings.WeatherVariants {
		clouds = append(clouds, 1)
	}

	for _, time := range settings.Times {
		for _, cloud := range clouds {
			if err := ctx.Err(); err != nil {
				return ChunkArtifact{}, worldchunk.Chunk{}, nil, err
			}

			condition := world.EnvironmentSettings{TimeOfDay: time, CloudAmount: cloud}

			image, rgbmRange, err := bakeEnvironmentVariantPixels(ctx, document, chunk.Layout, materials, condition, options, cache)
			if err != nil {
				return ChunkArtifact{}, worldchunk.Chunk{}, nil, fmt.Errorf("chunk %d time %g cloud %g: %w", chunk.ID, time, cloud, err)
			}

			module, imageDigest, err := encodeEnvironmentVariantModule(name, layoutHash, image)
			if err != nil {
				return ChunkArtifact{}, worldchunk.Chunk{}, nil, err
			}

			digest := sha256.Sum256(module)
			contentHash := hex.EncodeToString(digest[:])
			index := len(catalog.Variants)
			catalog.Variants = append(catalog.Variants, worldlightmap.EnvironmentVariant{
				TimeOfDay:     time,
				CloudAmount:   cloud,
				RGBMRange:     rgbmRange,
				ContentSHA256: contentHash,
				ImageSHA256:   imageDigest,
				Size:          uint64(len(module)),
				ImageSize:     uint64(len(image)),
				DecodedBytes:  decoded,
			})

			children = append(
				children,
				ChunkArtifact{ID: index, Bytes: module, ContentSHA256: contentHash, EnvelopeVersion: level.EnvelopeVersion},
			)
			if time == defaultTime && cloud == 0 {
				catalog.Default, defaultImage = index, image
			}
		}
	}

	encodedCatalog, err := worldlightmap.EncodeEnvironmentVariants(catalog, chunk.Layout, &document)
	if err != nil {
		return ChunkArtifact{}, worldchunk.Chunk{}, nil, err
	}

	identity, err := level.EncodeMetadata(level.Metadata{
		worldchunk.ChunkMetadataKey: level.StringValue(worldchunk.Schema), "chunkID": level.UintValue(uint64(chunk.ID)),
		"geometrySha256": level.StringValue(chunk.Mesh.GeometrySHA256), worldlightmap.MetadataKey: level.StringValue(worldlightmap.Schema),
		worldlightmap.EnvironmentVariantsMetadataKey: level.StringValue(worldlightmap.EnvironmentVariantsSchema),
	})
	if err != nil {
		return ChunkArtifact{}, worldchunk.Chunk{}, nil, err
	}

	entries := []level.SourceEntry{{Name: worldchunk.MeshEntryName, Kind: level.EntryData, Data: mesh},
		{Name: worldlightmap.EntryName, Kind: level.EntryData, Data: layout},
		{Name: worldlightmap.EnvironmentVariantsEntryName, Kind: level.EntryData, Data: encodedCatalog},
		{Name: worldlightmap.EnvironmentImageEntryName, Kind: level.EntryData, Data: defaultImage}}

	module, err := encodeLevelModule(name, identity, entries)
	if err != nil {
		return ChunkArtifact{}, worldchunk.Chunk{}, nil, err
	}

	digest := sha256.Sum256(module)
	contentHash := hex.EncodeToString(digest[:])
	artifact := ChunkArtifact{ID: chunk.ID, Bytes: module, ContentSHA256: contentHash, EnvelopeVersion: level.EnvelopeVersion}
	descriptor := worldchunk.Chunk{ID: chunk.ID, Sectors: chunk.Sectors, Solids: chunk.Solids, ContentSHA256: contentHash,
		Size: uint64(len(module)), MeshBytes: chunk.MeshBytes, LightmapBytes: decoded, Bounds: chunk.Bounds}

	return artifact, descriptor, children, nil
}

func bakeEnvironmentVariantPixels(ctx context.Context, document world.Document, layout worldlightmap.Layout,
	materials []worldlightmapbake.Material, settings world.EnvironmentSettings, options worldlightmapbake.Options,
	cache *chunkPixelCache) ([]byte, float64, error) {
	rgbmRange, err := worldlightmap.EnvironmentRGBMRange(document.Environment, settings, options.Bounces)
	if err != nil {
		return nil, 0, err
	}

	key, err := worldlightmapbake.EnvironmentInputDigest(document, layout, materials, settings, options)
	if err != nil {
		return nil, 0, err
	}

	image, found := cache.read(key, worldlightmap.EnvironmentPrebakeProducer, rgbmRange)
	if found {
		metadata, _ := qoi.Inspect(image)
		found = int(metadata.Width) == layout.Pages[0].Width*3 && int(metadata.Height) == layout.Pages[0].Height
	}

	if found {
		cache.hits++

		return image, rgbmRange, nil
	}

	var result worldlightmapbake.Result
	if cache != nil && cache.baker != nil {
		result, err = cache.baker.BakeEnvironment(ctx, layout, settings, options)
	} else {
		result, err = worldlightmapbake.BakeEnvironment(ctx, document, layout, materials, settings, options)
	}

	if err != nil {
		return nil, 0, err
	}

	image, _, err = qoi.Encode(result.Image, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceLinear})
	if err != nil {
		return nil, 0, err
	}

	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}

	if err := cache.write(key, worldlightmap.EnvironmentPrebakeProducer, result.RGBMRange, image); err != nil {
		return nil, 0, err
	}

	if cache != nil {
		cache.misses++
	}

	return image, result.RGBMRange, nil
}

func encodeEnvironmentVariantModule(name, layoutHash string, image []byte) ([]byte, string, error) {
	imageHash := sha256.Sum256(image)
	imageDigest := hex.EncodeToString(imageHash[:])

	identity, err := level.EncodeMetadata(level.Metadata{
		worldlightmap.EnvironmentVariantMetadataKey: level.StringValue(worldlightmap.EnvironmentVariantsSchema),
		"layoutSha256": level.StringValue(layoutHash), "imageSha256": level.StringValue(imageDigest),
	})
	if err != nil {
		return nil, "", err
	}

	module, err := encodeLevelModule(
		name,
		identity,
		[]level.SourceEntry{{Name: worldlightmap.EnvironmentImageEntryName, Kind: level.EntryData, Data: image}},
	)
	if err != nil {
		return nil, "", err
	}

	return module, imageDigest, nil
}
