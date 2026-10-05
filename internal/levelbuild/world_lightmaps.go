package levelbuild

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

type lightmapBuildSettings struct {
	Enabled      bool     `toml:"enabled"`
	Density      *float64 `toml:"density"`
	PageSize     *int     `toml:"page_size"`
	Padding      *int     `toml:"padding"`
	MaxPages     *int     `toml:"max_pages"`
	MaxTexels    *int     `toml:"max_texels"`
	Light        *string  `toml:"light"`
	Lights       []string `toml:"lights"`
	ShadowSize   *int     `toml:"shadow_size"`
	Prebake      *string  `toml:"prebake"`
	PrebakeImage *string  `toml:"prebake_image"`
	Offline      bool     `toml:"offline"`
	BakeSamples  *int     `toml:"bake_samples"`
	BakeBounces  *int     `toml:"bake_bounces"`
}

func (s lightmapBuildSettings) options() (worldlightmap.Options, error) {
	options := worldlightmap.Options{Lights: slices.Clone(s.Lights)}
	if err := s.validatePrebake(); err != nil {
		return options, err
	}

	if s.Light != nil {
		if *s.Light == "" || s.Lights != nil {
			return options, ErrManifest
		}

		options.Light = *s.Light
	}

	if s.Lights != nil && (len(s.Lights) == 0 || len(s.Lights) > worldlightmap.MaxBakeLights) {
		return options, ErrManifest
	}

	if s.Density != nil {
		if math.IsNaN(*s.Density) || math.IsInf(*s.Density, 0) || *s.Density <= 0 || *s.Density > 64 {
			return options, ErrManifest
		}

		options.TexelsPerUnit = *s.Density
	}

	for _, pair := range []struct {
		source *int
		target *int
	}{{s.PageSize, &options.PageSize}, {s.Padding, &options.Padding}, {s.MaxPages, &options.MaxPages}, {s.MaxTexels, &options.MaxTexels}, {s.ShadowSize, &options.ShadowSize}} {
		if pair.source != nil {
			if *pair.source <= 0 {
				return options, ErrManifest
			}

			*pair.target = *pair.source
		}
	}

	page := options.PageSize
	if page == 0 {
		page = 1024
	}

	if (page != 512 && page != 1024) || options.Padding > 16 || options.MaxPages > 1 ||
		(options.MaxTexels != 0 && (options.MaxTexels < page*page || options.MaxTexels > worldlightmap.MaxTexels)) ||
		(options.ShadowSize != 0 && (options.ShadowSize < 32 || options.ShadowSize > 512)) {
		return options, ErrManifest
	}

	return options, nil
}

func (s lightmapBuildSettings) validatePrebake() error {
	if (s.BakeSamples != nil && (*s.BakeSamples < 1 || *s.BakeSamples > worldlightmap.MaxOfflineSamples)) ||
		(s.BakeBounces != nil && (*s.BakeBounces < 0 || *s.BakeBounces > worldlightmap.MaxOfflineBounces)) {
		return ErrManifest
	}

	if s.Offline && (s.Prebake != nil || s.PrebakeImage != nil || len(s.Lights) == 0) {
		return ErrManifest
	}

	if (s.Prebake == nil) != (s.PrebakeImage == nil) ||
		(s.Prebake != nil && (*s.Prebake == "" || *s.PrebakeImage == "" || len(s.Lights) == 0)) {
		return ErrManifest
	}

	return nil
}

func validateLightmapSettings(definition manifest, assets *assetBuild) error {
	settings := definition.Lightmap
	if settings == nil || !settings.Enabled {
		return nil
	}

	if definition.World.Source == "" || assets == nil ||
		!slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldLightmapsV1) {
		return fmt.Errorf("lightmap requires a world and SDK advertising %s: %w", worldlightmap.Feature, ErrManifest)
	}

	if (settings.Prebake != nil || settings.Offline) &&
		!slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldLightmapsPrebakedV1) {
		return fmt.Errorf("prebake requires SDK advertising %s: %w", worldlightmap.PrebakeFeature, ErrManifest)
	}

	if _, err := settings.options(); err != nil {
		return fmt.Errorf("lightmap settings: %w", err)
	}

	return nil
}

func buildWorldLightmaps(
	directory string,
	definition manifest,
	document world.Document,
	data []level.SourceEntry,
	metadata []byte,
) ([]level.SourceEntry, []byte, []string, error) {
	if definition.Lightmap == nil || !definition.Lightmap.Enabled {
		return data, metadata, nil, nil
	}

	options, err := definition.Lightmap.options()
	if err != nil {
		return nil, nil, nil, err
	}

	layout, err := worldlightmap.Compile(document, options)
	if err != nil {
		return nil, nil, nil, err
	}

	encoded, err := worldlightmap.Encode(layout, &document)
	if err != nil {
		return nil, nil, nil, err
	}

	for _, entry := range data {
		if entry.Name == worldlightmap.EntryName || entry.Name == worldlightmap.PrebakeEntryName ||
			entry.Name == worldlightmap.PrebakeImageEntryName {
			return nil, nil, nil, ErrSource
		}
	}

	var identity map[string]json.RawMessage
	if err := json.Unmarshal(metadata, &identity); err != nil || identity == nil {
		return nil, nil, nil, ErrManifest
	}

	identity[worldlightmap.MetadataKey], err = json.Marshal(worldlightmap.Schema)
	if err != nil {
		return nil, nil, nil, err
	}

	features := []string{worldlightmap.Feature}

	var prebaked []level.SourceEntry
	if definition.Lightmap.Prebake != nil {
		prebaked, err = readWorldPrebake(
			directory,
			*definition.Lightmap.Prebake,
			*definition.Lightmap.PrebakeImage,
			layout,
			&document,
			definition,
		)
		if err != nil {
			return nil, nil, nil, err
		}
	} else if definition.Lightmap.Offline {
		prebaked = readAutomaticPrebake(directory, definition, layout, document)
	}

	if len(prebaked) != 0 {
		data = append(data, prebaked...)

		identity[worldlightmap.PrebakeMetadataKey], err = json.Marshal(worldlightmap.PrebakeSchema)
		if err != nil {
			return nil, nil, nil, err
		}

		features = append(features, worldlightmap.PrebakeFeature)
	}

	metadata, err = json.Marshal(identity)
	if err != nil {
		return nil, nil, nil, err
	}

	data = append(data, level.SourceEntry{Name: worldlightmap.EntryName, Kind: level.EntryData, Data: encoded})

	return data, metadata, features, nil
}

func readWorldPrebake(directory, manifestPath, imagePath string, layout worldlightmap.Layout,
	document *world.Document, definition manifest) ([]level.SourceEntry, error) {
	encoded, err := readConfinedFile(directory, manifestPath, worldlightmap.MaxPrebakeManifestSize)
	if err != nil {
		return nil, err
	}

	image, err := readConfinedFile(directory, imagePath, worldlightmap.MaxPrebakeImageSize)
	if err != nil {
		return nil, err
	}

	pair, err := worldlightmap.DecodePrebake(encoded, image, layout, document)
	if err != nil {
		return nil, fmt.Errorf("prebake: %w", err)
	}

	if pair.Manifest.Algorithm == worldlightmap.OfflinePrebakeAlgorithm {
		if err := validateOfflineReflectance(directory, definition, *document, pair.Manifest.ReflectanceSHA256); err != nil {
			return nil, fmt.Errorf("offline prebake reflectance: %w", err)
		}
	}

	return []level.SourceEntry{
		{Name: worldlightmap.PrebakeEntryName, Kind: level.EntryData, Data: encoded},
		{Name: worldlightmap.PrebakeImageEntryName, Kind: level.EntryData, Data: image},
	}, nil
}
