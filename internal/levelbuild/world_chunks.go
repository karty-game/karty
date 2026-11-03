package levelbuild

import (
	"fmt"
	"slices"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

type chunkBuildSettings struct {
	Enabled           bool                  `toml:"enabled"`
	MaxMeshBytes      *uint64               `toml:"max_mesh_bytes"`
	MaxLightmapTexels *int                  `toml:"max_lightmap_texels"`
	MaxEncodedBytes   *uint64               `toml:"max_encoded_bytes"`
	MaxSpan           *float64              `toml:"max_span"`
	MaxSectors        *int                  `toml:"max_sectors"`
	Spatial           *spatialChunkSettings `toml:"spatial"`
	GPU               *gpuChunkSettings     `toml:"gpu"`
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

	maximumSpan, err := chunkSpanLimit(settings)
	if err != nil {
		return worldchunk.Options{}, err
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
		EnclosureAware:  true,
		MaxSpan:         maximumSpan,
		MaxMeshBytes:    defaultChunkMeshBytes,
		MaxEncodedBytes: defaultChunkEncodedBytes,
		MaxSectors:      defaultChunkSectors,
		Lightmap:        lightmap,
		NoLightmap:      noLightmap,
	}

	if settings.MaxMeshBytes != nil {
		maximum := uint64(level.MaxEntrySize)
		if settings.Spatial != nil {
			maximum = maxSpatialGeometryBytes
		}

		if *settings.MaxMeshBytes == 0 || *settings.MaxMeshBytes > maximum {
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

	if err := validateSpatialChunkSettings(settings.Spatial, lightmap.TexelsPerUnit); err != nil {
		return worldchunk.Options{}, err
	}

	if settings.Spatial != nil && (noLightmap || definition.Lightmap == nil || !definition.Lightmap.Fixed) {
		return worldchunk.Options{}, fmt.Errorf("spatial chunks require fixed offline lighting: %w", ErrManifest)
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

// Fresh builds and validated caches use the same root ownership and metadata path.
