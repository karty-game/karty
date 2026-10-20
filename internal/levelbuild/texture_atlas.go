package levelbuild

import (
	"context"
	"slices"
	"strings"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/textureatlas"
	"github.com/karty-game/karty-sdk/format/worldparticles"
	"github.com/karty-game/karty-sdk/format/worldvegetation"
	"github.com/karty-game/karty/internal/assetpipeline"
)

func packageTextureAtlas(
	ctx context.Context,
	entries []level.SourceEntry,
	reports []assetpipeline.Texture,
	assets *assetBuild,
) ([]level.SourceEntry, []assetpipeline.Texture, bool, error) {
	if assets == nil || !slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.CapabilityTextureAtlasV1) {
		return entries, reports, false, nil
	}

	filters, err := atlasImageFilters(entries)
	if err != nil {
		return nil, nil, false, err
	}

	var (
		inputs []assetpipeline.AtlasImage
		groups [][]string
	)

	result := make([]level.SourceEntry, 0, len(entries))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name, "@atlas/") {
			return nil, nil, false, textureatlas.ErrAtlas
		}

		if entry.Kind == level.EntryTexture {
			id, _ := level.TextureAssetID(entry.Name)
			inputs = append(inputs, assetpipeline.AtlasImage{Name: entry.Name, Bytes: entry.Data, Filter: filters[id]})

			continue
		}

		if entry.Name == worldparticles.EntryName {
			particles, err := worldparticles.Decode(entry.Data)
			if err != nil {
				return nil, nil, false, err
			}

			for _, preset := range particles.Presets {
				group := make([]string, len(preset.Sprite.Frames))
				for i, id := range preset.Sprite.Frames {
					group[i] = level.TextureEntryName(id)
				}

				groups = append(groups, group)
			}
		}

		result = append(result, entry)
	}

	if len(inputs) == 0 {
		return entries, reports, false, nil
	}

	bundle, pages, err := assetpipeline.ProcessTextureAtlas(ctx, assets.projectRoot, inputs, groups)
	if err != nil {
		return nil, nil, false, err
	}

	atlasEntries, err := bundle.Entries()
	if err != nil {
		return nil, nil, false, err
	}

	for _, entry := range atlasEntries {
		result = append(result, level.SourceEntry{Name: entry.Name, Kind: level.EntryData, Data: entry.Bytes})
	}

	for _, report := range reports {
		if report.Kind == "texture" {
			pages[0].SourceBytes += report.SourceBytes
		}
	}

	reports = slices.DeleteFunc(slices.Clone(reports), func(report assetpipeline.Texture) bool { return report.Kind == "texture" })

	return result, append(reports, pages...), true, nil
}

func atlasImageFilters(entries []level.SourceEntry) (map[uint32]int, error) {
	filters := make(map[uint32]int)

	for _, entry := range entries {
		if entry.Name != worldvegetation.EntryName {
			continue
		}

		vegetation, err := worldvegetation.Decode(entry.Data)
		if err != nil {
			return nil, err
		}

		for _, preset := range vegetation.Presets {
			if filters[preset.Texture] == textureatlas.FilterNormal {
				return nil, textureatlas.ErrAtlas
			}

			filters[preset.Texture] = textureatlas.FilterColor
			if preset.NormalTexture != 0 {
				if filters[preset.NormalTexture] == textureatlas.FilterColor {
					return nil, textureatlas.ErrAtlas
				}

				filters[preset.NormalTexture] = textureatlas.FilterNormal
			}
		}
	}

	return filters, nil
}

func finalizeLevelTextures(
	ctx context.Context,
	data []level.SourceEntry,
	processedTextures []assetpipeline.Texture,
	textureMetadata []metadataTexture,
	features []string,
	metadata []byte,
	assets *assetBuild,
) ([]level.SourceEntry, []assetpipeline.Texture, []string, []byte, error) {
	data, err := omitAtlasSourceTextures(data)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	processedTextures = packagedTextureReports(processedTextures, textureMetadata, data)

	var packed bool

	data, processedTextures, packed, err = packageTextureAtlas(ctx, data, processedTextures, assets)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	if packed {
		values, decodeErr := level.DecodeMetadata(metadata)
		if decodeErr != nil {
			return nil, nil, nil, nil, decodeErr
		}

		values["kartyTextureAtlas"] = level.StringValue("texture/atlas@1")

		metadata, err = level.EncodeMetadata(values)
		if err != nil {
			return nil, nil, nil, nil, err
		}

		features = append(features, "texture/atlas@1")
		slices.Sort(features)
		features = slices.Compact(features)
	}

	return data, processedTextures, features, metadata, nil
}
