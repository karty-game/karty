package levelbuild

import (
	"context"
	"fmt"

	worldlightmapbake "github.com/karty-game/karty-sdk/bake/worldlightmap"
	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

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

func bakeWorldChunkLightmap(
	ctx context.Context, document world.Document, layout worldlightmap.Layout,
	materials []worldlightmapbake.Material, options worldlightmapbake.Options,
	cache *chunkPixelCache,
) ([]level.SourceEntry, error) {
	encodedLayout, err := worldlightmap.Encode(layout, &document)
	if err != nil {
		return nil, err
	}

	producer, err := worldlightmap.OfflineProducer(options.Denoise, options.MaxRayDistance)
	if err != nil {
		return nil, err
	}

	rgbmRange, err := worldlightmap.OfflineRGBMRangeBounded(layout, &document, options.Bounces, options.MaxRayDistance)
	if err != nil {
		return nil, err
	}

	var (
		key         string
		image       []byte
		reflectance string
		hit         bool
	)

	if cache.enabled() {
		key, err = worldlightmapbake.BakeInputDigest(document, layout, materials, options)
		if err != nil {
			return nil, err
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
		return nil, err
	}

	pair, err := worldlightmap.NewOfflinePrebake(layout, &document, image, worldlightmap.OfflineBakeInputs{
		ReflectanceSHA256: reflectance, Samples: options.Samples, Bounces: options.Bounces,
		Seed: options.Seed, RGBMRange: rgbmRange, Denoise: options.Denoise, MaxRayDistance: options.MaxRayDistance,
	})
	if err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if !hit {
		if err := cache.write(key, producer, rgbmRange, image); err != nil {
			return nil, fmt.Errorf("write chunk pixel cache: %w", err)
		}
	}

	prebake, err := worldlightmap.EncodePrebake(pair, layout, &document)
	if err != nil {
		return nil, err
	}

	entries := []level.SourceEntry{
		{Name: worldlightmap.EntryName, Kind: level.EntryData, Data: encodedLayout},
		{Name: worldlightmap.PrebakeEntryName, Kind: level.EntryData, Data: prebake},
		{Name: worldlightmap.PrebakeImageEntryName, Kind: level.EntryData, Data: image},
	}

	return entries, nil
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

	var (
		baked   worldlightmapbake.Result
		bakeErr error
	)
	if cache != nil && cache.baker != nil {
		baked, bakeErr = cache.baker.Bake(ctx, layout, options)
	} else {
		baked, bakeErr = worldlightmapbake.Bake(ctx, document, layout, materials, options)
	}

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
