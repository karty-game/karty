package levelbuild

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty/internal/assetpipeline"
)

var errWebGPUChartEnvelope = errors.New("WebGPU chart atlas exceeds level entry bound")

func prepareWebGPUCharts(ctx context.Context, assets *assetBuild, document *world.Document, sources []level.SourceEntry,
	metadata []byte, strengths map[uint32]*worldmaterial.Strengths, settings map[uint32]assetpipeline.MaterializeOptions,
) (*assetBuild, []level.SourceEntry, []byte, error) {
	if err := validateWebGPUSingleLayer(document); err != nil {
		return nil, nil, nil, err
	}

	charts := needsWebGPUCharts(document)
	if !charts && !worldSSBumpEnabled(assets) {
		return assets, sources, metadata, nil
	}

	textures := make(map[uint32][]byte, len(sources))
	nextID := uint32(1)

	for _, source := range sources {
		textureID, valid := level.TextureAssetID(source.Name)
		if !valid || textureID == math.MaxUint32 {
			return nil, nil, nil, ErrSource
		}

		textures[textureID] = source.Data
		nextID = max(nextID, textureID+1)
	}

	process := assets.materials
	if process == nil {
		process = assetpipeline.ProcessWorldMaterials
	}

	pair, cached, err := process(ctx, assets.projectRoot, assets.manifest, *document, textures, settings)
	if err != nil {
		return nil, nil, nil, err
	}

	pair.Layout = applyMaterialStrengths(pair.Layout, strengths)

	var originalSS *worldSSBumpBuild
	if worldSSBumpEnabled(assets) {
		originalSS, err = generateWorldSSBump(ctx, assets, pair)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	if !charts {
		originalSS.pair, err = assetpipeline.PackWorldMaterialSSBump(ctx, pair.Layout, originalSS.tiles)
		if err != nil {
			return nil, nil, nil, err
		}

		if err := originalSS.pair.ValidateTogether(pair); err != nil {
			return nil, nil, nil, err
		}

		if err := checkSSBumpEntryBounds(originalSS.pair); err != nil {
			return nil, nil, nil, err
		}

		owned := preparedWebGPUMaterials(assets, pair, cached, originalSS)

		return owned, sources, metadata, nil
	}

	baker, converted, err := fitWebGPUChartsAtGrid(ctx, document, pair, nextID, assets.chartGrid, originalSS)
	if err != nil {
		return nil, nil, nil, err
	}

	identity, err := level.DecodeMetadata(metadata)
	if err != nil {
		return nil, nil, nil, err
	}

	values, _ := identity["kartyTextures"].AsArray()

	for _, tile := range baker.tiles {
		encoded, _, err := qoi.Encode(tile.albedo, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceSRGB})
		if err != nil {
			return nil, nil, nil, err
		}

		sources = append(sources, level.SourceEntry{Name: level.TextureEntryName(tile.id), Kind: level.EntryTexture, Data: encoded})
		values = append(values, level.ObjectValue(level.Metadata{"id": level.UintValue(uint64(tile.id)),
			"name": level.StringValue(fmt.Sprintf("karty_webgpu_chart_%08x", tile.id))}))
	}

	identity["kartyTextures"] = level.ArrayValue(values)

	profileRevision := webGPUChartRevision
	if originalSS != nil {
		profileRevision += "+" + worldSSBumpRevision
	}

	identity["kartyWebGPUCharts"] = level.StringValue(profileRevision)
	identity["kartyWebGPUChartGrid"] = level.UintValue(uint64(baker.grid))

	metadata, err = level.EncodeMetadata(identity)
	if err != nil {
		return nil, nil, nil, err
	}
	// No legacy material cache entry is rewritten. The prepared result is local
	// to this complete build, and its report binds the profile and baked pixels.
	hash := sha256.New()
	hash.Write([]byte(profileRevision))
	hash.Write([]byte(cached.CacheKey))
	hash.Write(converted.Albedo)
	hash.Write(converted.Data)
	hash.Write(converted.Reflectance)
	cached.CacheKey, cached.CacheHit = hex.EncodeToString(hash.Sum(nil)), false
	owned := preparedWebGPUMaterials(assets, converted, cached, baker.ssbump)
	owned.chartGrid = baker.grid

	owned.chartSourceIDs = make([]uint32, len(pair.Layout.Materials))
	for i, r := range pair.Layout.Materials {
		owned.chartSourceIDs[i] = r.MaterialID
	}

	return owned, sources, metadata, nil
}

func fitWebGPUChartsAtGrid(
	ctx context.Context,
	document *world.Document,
	pair worldmaterial.Pair,
	nextID uint32,
	minimum int,
	companions ...*worldSSBumpBuild,
) (*webGPUChartBaker, worldmaterial.Pair, error) {
	if len(companions) > 1 {
		return nil, worldmaterial.Pair{}, ErrSource
	}

	original, err := world.Encode(*document)
	if err != nil {
		return nil, worldmaterial.Pair{}, err
	}

	for _, grid := range []int{2, 4, 8} {
		if grid < minimum {
			continue
		}

		if grid != webGPUChartDefaultGrid {
			*document, err = world.Decode(original)
			if err != nil {
				return nil, worldmaterial.Pair{}, err
			}
		}

		baker, err := newWebGPUChartBaker(document, pair, nextID)
		if err != nil {
			return nil, worldmaterial.Pair{}, err
		}

		baker.grid = grid
		if len(companions) == 1 {
			baker.ssbump = companions[0]
		}

		if err := baker.convert(ctx); err != nil {
			return nil, worldmaterial.Pair{}, err
		}

		converted, err := baker.pack(ctx, len(pair.Layout.Mips) != 0)
		if err == nil {
			var packed *worldSSBumpBuild

			if baker.ssbump != nil {
				packed, err = baker.packSSBump(ctx, converted)
			}

			if err == nil {
				baker.ssbump = packed

				return baker, converted, nil
			}
		}

		if !errors.Is(err, errWebGPUChartEnvelope) || grid == 8 {
			return nil, worldmaterial.Pair{}, err
		}
	}

	return nil, worldmaterial.Pair{}, errWebGPUChartEnvelope
}

func (b *webGPUChartBaker) pack(ctx context.Context, mips bool) (worldmaterial.Pair, error) {
	layout, err := worldmaterial.NewLayoutV2(world.MaterialIDs(b.document))
	if err != nil {
		return worldmaterial.Pair{}, fmt.Errorf("WebGPU chart material table: %w", err)
	}

	if mips {
		layout, err = worldmaterial.NewMipLayout(layout)
		if err != nil {
			return worldmaterial.Pair{}, err
		}
	}

	width, height := layout.MipDimensions()

	images := uint64(2)
	if b.source.Reflectance() != nil {
		images++
	}

	if uint64(layout.Width)*uint64(layout.Height)*4*images+uint64(width)*uint64(height)*4*(images-1) > asset.MaxDecodedTextures {
		return worldmaterial.Pair{}, fmt.Errorf("WebGPU chart atlas exceeds decoded texture budget: %w", ErrManifest)
	}

	pixels, err := b.atlasPixels(&layout, images)
	if err != nil {
		return worldmaterial.Pair{}, err
	}

	pair := worldmaterial.Pair{Layout: layout}
	for index, target := range []*[]byte{&pair.Albedo, &pair.Data, &pair.Reflectance} {
		if pixels[index] == nil {
			continue
		}

		space := qoi.ColorspaceLinear
		if index == 0 {
			space = qoi.ColorspaceSRGB
		}

		*target, _, err = qoi.Encode(pixels[index], qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: space})
		if err != nil {
			return worldmaterial.Pair{}, err
		}
	}

	if mips {
		pair.MipTail, pair.ReflectanceMipTail, err = webGPUChartMips(ctx, layout, pixels)
		if err != nil {
			return worldmaterial.Pair{}, err
		}
	}

	for _, entry := range []struct {
		name string
		data []byte
	}{
		{worldmaterial.AlbedoEntry, pair.Albedo}, {worldmaterial.DataEntry, pair.Data},
		{worldmaterial.MipTailEntry, pair.MipTail}, {worldmaterial.ReflectanceEntry, pair.Reflectance},
		{worldmaterial.ReflectanceMipTailEntry, pair.ReflectanceMipTail},
	} {
		if len(entry.data) > level.MaxEntrySize {
			return worldmaterial.Pair{}, fmt.Errorf(
				"%w: %s is %d bytes (limit %d; %dpx chart interiors)",
				errWebGPUChartEnvelope,
				entry.name,
				len(entry.data),
				level.MaxEntrySize,
				webGPUChartTileSize/b.grid-2*webGPUChartGutter,
			)
		}
	}

	return pair, pair.Validate()
}

func copyWebGPUTile(destination, source *image.NRGBA, rect worldmaterial.Rect, sourceX, sourceY int) {
	for row := -rect.Gutter; row < rect.Height+rect.Gutter; row++ {
		for column := -rect.Gutter; column < rect.Width+rect.Gutter; column++ {
			pixel := color.NRGBA{A: webGPUChartByteMax}
			if source != nil {
				pixel = source.NRGBAAt(sourceX+min(rect.Width-1, max(0, column)), sourceY+min(rect.Height-1, max(0, row)))
			}

			destination.SetNRGBA(rect.X+column, rect.Y+row, pixel)
		}
	}
}

func (b *webGPUChartBaker) atlasPixels(layout *worldmaterial.Layout, images uint64) ([3]*image.NRGBA, error) {
	pixels := [3]*image.NRGBA{}
	for index := range images {
		pixels[index] = image.NewNRGBA(image.Rect(0, 0, layout.Width, layout.Height))
		for at := 3; at < len(pixels[index].Pix); at += 4 {
			pixels[index].Pix[at] = webGPUChartByteMax
		}
	}

	generated := make(map[uint32]webGPUChartTile, len(b.tiles))
	for _, tile := range b.tiles {
		generated[tile.id] = tile
	}

	for index := range layout.Materials {
		rect := &layout.Materials[index]
		tile, baked := generated[rect.MaterialID]

		var (
			source           [3]*image.NRGBA
			sourceX, sourceY int
		)

		if baked {
			source = [3]*image.NRGBA{tile.albedo, tile.data, tile.reflectance}
			if tile.masked {
				rect.Coverage = worldmaterial.CoverageMasked
			}
		} else {
			original, exists := b.materials[rect.MaterialID]
			if !exists {
				return [3]*image.NRGBA{}, fmt.Errorf("chart material %d is missing: %w", rect.MaterialID, ErrSource)
			}

			rect.Strengths, rect.Coverage = original.Strengths, original.Coverage
			if rect.Coverage == "" {
				rect.Coverage = worldmaterial.CoverageOpaque
			}

			sourceX, sourceY = original.X, original.Y
			source = [3]*image.NRGBA{b.source.Albedo(), b.source.Data(), b.source.Reflectance()}
		}

		for channel := range images {
			copyWebGPUTile(pixels[channel], source[channel], *rect, sourceX, sourceY)
		}
	}

	return pixels, nil
}
