package levelbuild

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"

	"github.com/karty-game/karty-sdk/codec/ktx2"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty-sdk/format/worldmaterialgpu"
	"github.com/karty-game/karty-sdk/format/worldmaterialssbump"
	"github.com/karty-game/karty/internal/assetpipeline"
)

func worldMaterialGPUEnabled(assets *assetBuild) bool {
	return assets != nil && slices.Contains(assets.manifest.Assets.Capabilities.Runtime, worldmaterialgpu.Feature)
}

func appendWorldMaterialGPU(
	ctx context.Context,
	assets *assetBuild,
	document world.Document,
	base worldmaterial.Pair,
	result *worldMaterialBuild,
) error {
	prepared := assets.ssbump
	if prepared == nil {
		var err error

		prepared, err = generateWorldSSBump(ctx, assets, base)
		if err != nil {
			return err
		}
	}

	pair, cached, err := assetpipeline.ProcessWorldMaterialGPU(
		ctx,
		assets.projectRoot,
		assets.manifest,
		base,
		prepared.tiles,
		worldWaterNormalAmplitudes(document, worldGPUChunksEnabled(assets)),
	)
	if err != nil {
		return err
	}

	return packageWorldMaterialGPU(result, base.Layout, pair, cached)
}

// New GPU materials retain the canonical material layout for geometry and
// descriptor binding. QOI data remains a build/cache intermediate and does not
// enter the finished runtime level or its decoded texture accounting.
func packageWorldMaterialGPU(
	result *worldMaterialBuild,
	layout worldmaterial.Layout,
	pair worldmaterialgpu.Pair,
	cached []assetpipeline.Artifact,
) error {
	if err := pair.Validate(layout); err != nil {
		return err
	}

	if len(cached) != len([...][]byte{pair.Albedo, pair.SSBump, pair.Detail}) {
		return ErrSource
	}

	for _, entry := range result.data {
		if ownedMaterialGPUEntry(entry.Name) {
			return ErrSource
		}
	}

	identity, err := level.DecodeMetadata(result.metadata)
	if err != nil {
		return err
	}

	if _, exists := identity[worldmaterialgpu.MetadataKey]; exists {
		return ErrSource
	}

	identity[worldmaterialgpu.MetadataKey] = level.StringValue(worldmaterialgpu.Feature)
	delete(identity, worldmaterialssbump.MetadataKey)

	metadata, err := level.EncodeMetadata(identity)
	if err != nil {
		return err
	}

	descriptor, err := worldmaterialgpu.EncodeDescriptor(pair.Descriptor)
	if err != nil {
		return err
	}

	entries := []level.SourceEntry{{Name: worldmaterialgpu.DescriptorEntry, Kind: level.EntryData, Data: descriptor}}

	reports := make([]assetpipeline.Texture, 0, len(cached))
	for index, entry := range []struct {
		name string
		data []byte
	}{{worldmaterialgpu.AlbedoEntry, pair.Albedo}, {worldmaterialgpu.SSBumpEntry, pair.SSBump}, {worldmaterialgpu.DetailEntry, pair.Detail}} {
		if len(entry.data) > level.MaxEntrySize {
			return fmt.Errorf("%s: %w", entry.name, level.ErrEnvelopeSize)
		}

		info, err := ktx2.Inspect(entry.data)
		if err != nil {
			return err
		}

		entries = append(entries, level.SourceEntry{Name: entry.name, Kind: level.EntryData, Data: entry.data})
		digest := sha256.Sum256(entry.data)
		checksum := hex.EncodeToString(digest[:])
		reports = append(
			reports,
			assetpipeline.Texture{Kind: "world-material", Name: entry.name, Profile: "world-material", Source: world.EntryName,
				Output: checksum + ".ktx2", ContentSHA256: checksum, OutputSHA256: checksum,
				SourceSHA256: cached[index].SourceDigest, CacheKey: cached[index].CacheKey, CacheHit: cached[index].CacheHit,
				Processor: worldmaterialgpu.Feature, Encoding: "ktx2-uastc-zstd", Width: int(info.Width), Height: int(info.Height),
				OutputBytes: int64(len(entry.data)), EstimatedDecodedBytes: int64(info.RGBABytes)},
		)
	}

	result.data = slices.DeleteFunc(result.data, func(entry level.SourceEntry) bool {
		return legacyMaterialGPUImage(entry.Name) || ownedSSBumpEntry(entry.Name)
	})
	result.data = append(result.data, entries...)
	result.metadata = metadata
	result.textures = slices.DeleteFunc(result.textures, func(texture assetpipeline.Texture) bool {
		return legacyMaterialGPUImage(texture.Name) || ownedSSBumpEntry(texture.Name)
	})
	result.textures = append(result.textures, reports...)
	result.features = slices.DeleteFunc(result.features, func(feature string) bool {
		return feature == worldmaterial.FeatureReflectance || feature == worldmaterialssbump.Feature
	})
	result.features = append(result.features, worldmaterialgpu.Feature)

	return nil
}

func legacyMaterialGPUImage(name string) bool {
	return name == worldmaterial.AlbedoEntry || name == worldmaterial.DataEntry || name == worldmaterial.MipTailEntry ||
		name == worldmaterial.ReflectanceEntry || name == worldmaterial.ReflectanceMipTailEntry
}

func ownedMaterialGPUEntry(name string) bool {
	return name == worldmaterialgpu.DescriptorEntry || name == worldmaterialgpu.AlbedoEntry ||
		name == worldmaterialgpu.SSBumpEntry || name == worldmaterialgpu.DetailEntry
}

func worldWaterNormalAmplitudes(document world.Document, enabled bool) map[uint32]float64 {
	if !enabled || document.Animations == nil {
		return nil
	}

	result := make(map[uint32]float64)

	for _, binding := range document.Animations.Materials {
		for _, preset := range document.Animations.Presets {
			if preset.Name == binding.Preset && preset.Kind == "liquid" {
				result[binding.Material] = preset.Amplitude

				break
			}
		}
	}

	return result
}
