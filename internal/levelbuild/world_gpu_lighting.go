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

type gpuCellLighting struct {
	pixels        []byte
	width, height uint32
	rangeValue    float64
	entries       []level.SourceEntry
}

func gpuLightingOptions(
	definition manifest,
	document *world.Document,
	entries []level.SourceEntry,
) (worldlightmap.Options, worldlightmapbake.Options, error) {
	bake := chunkBakeOptions(definition.Lightmap)
	if definition.Lightmap == nil || !definition.Lightmap.Enabled {
		return worldlightmap.Options{}, bake, nil
	}

	settings := definition.Lightmap
	if settings.Environment != nil {
		return worldlightmap.Options{}, bake, fmt.Errorf("GPU cells do not support environment-variant lighting: %w", ErrManifest)
	}

	if !settings.Offline && settings.Prebake == nil {
		return worldlightmap.Options{}, bake, fmt.Errorf("GPU cell lighting requires a completed offline bake: %w", ErrManifest)
	}

	options, err := settings.options()
	if err != nil {
		return options, bake, err
	}

	if options.PageSize == 0 {
		options.PageSize = 1024
	}

	if options.TexelsPerUnit == 0 {
		options.TexelsPerUnit = 16
	}

	// Fixed lighting can bake the environment without any authored point lights.
	// Match the fixed semantic chunk path rather than requesting a static-light
	// enumeration from an absent lighting document.
	if settings.Fixed && document.Lighting == nil {
		options.AllStatic = false
	}

	options.MaxPages = 1
	options.MaxTexels = options.PageSize * options.PageSize
	options.CapsWithinSourceRoom = true
	options.CompactPacking = true

	if settings.Prebake != nil {
		manifest, err := worldlightmap.DecodePrebakeManifest(gpuWorldEntry(entries, worldlightmap.PrebakeEntryName))
		if err != nil {
			return options, bake, err
		}

		mode, err := worldlightmap.OfflineDenoiseMode(manifest.Producer)
		if err != nil {
			return options, bake, err
		}

		bake.Denoise = mode
		bake.Samples, bake.Bounces, bake.Seed, bake.MaxRayDistance = manifest.Samples, manifest.Bounces, manifest.Seed, manifest.MaxRayDistance
	}

	return options, bake, nil
}

func (build *gpuWorldBuild) cellLighting(ctx context.Context, cell gpuCellPlan) (gpuCellLighting, error) {
	if build.lighting.PageSize == 0 {
		return gpuCellLighting{
			width:      1,
			height:     1,
			rangeValue: 1,
			pixels:     []byte{153, 153, 153, 255, 153, 153, 153, 255, 153, 153, 153, 255},
		}, nil
	}

	layout := cell.chunk.Layout
	page := layout.Pages[0]
	result := gpuCellLighting{width: uint32(page.Width), height: uint32(page.Height)}

	if build.definition.Lightmap.Fixed {
		return build.fixedCellLighting(ctx, layout, result)
	}

	if build.assets.development {
		key, err := worldlightmapbake.BakeInputDigest(build.document, layout, build.bakeMaterials, build.bake)
		if err != nil {
			return result, err
		}

		producer, err := worldlightmap.OfflineProducer(build.bake.Denoise, build.bake.MaxRayDistance)
		if err != nil {
			return result, err
		}

		span, err := worldlightmap.OfflineRGBMRangeBounded(layout, &build.document, build.bake.Bounces, build.bake.MaxRayDistance)
		if err != nil {
			return result, err
		}

		image, found := build.cache.read(key, producer, span)

		metadata, err := qoi.Inspect(image)
		if !found || err != nil || int(metadata.Width) != page.Width*3 || int(metadata.Height) != page.Height {
			return result, fmt.Errorf("GPU cell requires a completed offline prebake; run karty build: %w", ErrManifest)
		}
	}

	entries, err := bakeWorldChunkLightmap(ctx, build.document, layout, build.bakeMaterials, build.bake, build.cache)
	if err != nil {
		return result, err
	}

	manifest, err := worldlightmap.DecodePrebakeManifest(gpuWorldEntry(entries, worldlightmap.PrebakeEntryName))
	if err != nil {
		return result, err
	}

	result.rangeValue = manifest.RGBMRange

	result.pixels, err = cellRNMPixels(layout, worldlightmap.FixedLighting{}, gpuWorldEntry(entries, worldlightmap.PrebakeImageEntryName))
	if err != nil {
		return result, err
	}

	// The completed bake remains an offline input/cache artifact. The cell's
	// GPU payload and lossless RNM image carry its complete render data.

	return result, nil
}

func (build *gpuWorldBuild) fixedCellLighting(
	ctx context.Context,
	layout worldlightmap.Layout,
	result gpuCellLighting,
) (gpuCellLighting, error) {
	if build.assets.development {
		if err := build.requireFixedCellCache(layout); err != nil {
			return result, err
		}
	}

	fixed, image, err := fixedChunkLighting(ctx, build.document, layout, build.bakeMaterials, build.bake, build.cache, false)
	if err != nil {
		return result, err
	}

	result.rangeValue = fixed.RGBMRange

	result.pixels, err = cellRNMPixels(layout, fixed, image)
	if err != nil {
		return result, err
	}

	// The cell payload and GPU-ready RNM are the complete render artifact.
	// Keep CPU layouts/fixed tables in offline caches and physical chunks;
	// optional render-cell provenance adds no runtime lighting data.

	return result, nil
}

func (build *gpuWorldBuild) requireFixedCellCache(layout worldlightmap.Layout) error {
	key, err := worldlightmapbake.FixedInputDigest(build.document, layout, build.bakeMaterials, build.bake)
	if err != nil {
		return err
	}

	if _, _, found := readFixedChunkPixels(build.cache, key, layout, &build.document, false); !found {
		return fmt.Errorf("GPU cell requires a completed offline bake; run karty build: %w", ErrManifest)
	}

	return nil
}

// Imported prebakes remain validated compiler inputs. Generated GPU cells are
// baked directly; they never require an intermediate whole-world atlas.
func needsWholeWorldLightingInput(definition manifest, assets *assetBuild) bool {
	return !worldGPUChunksEnabled(assets) || (definition.Lightmap != nil && definition.Lightmap.Prebake != nil)
}

func gpuCellBakedLights(document *world.Document, cells []gpuCellPlan) []uint32 {
	if document.Lighting == nil {
		return nil
	}

	baked := make(map[string]bool)

	for _, cell := range cells {
		if cell.chunk.Layout.RuntimeBake == nil || cell.chunk.Layout.RuntimeBake.Encoding != worldlightmap.DirectRNMEncoding {
			continue
		}

		for _, name := range cell.chunk.Layout.RuntimeBake.LightIDs {
			baked[name] = true
		}
	}

	var indices []uint32

	for index, light := range document.Lighting.Lights {
		if baked[light.ID] {
			indices = append(indices, uint32(index))
		}
	}

	return indices
}

// Spatial loading-unit regrouping compiles receiver layouts independently of
// GPU cell packing. Keep environment-only fixed lighting valid in both paths.
func gpuLoadingChunkOptions(definition manifest, document *world.Document) (worldchunk.Options, error) {
	options, err := chunkOptions(definition)
	if err != nil {
		return options, err
	}

	if definition.Lightmap != nil && definition.Lightmap.Fixed && document.Lighting == nil {
		options.Lightmap.AllStatic = false
	}

	return options, nil
}
