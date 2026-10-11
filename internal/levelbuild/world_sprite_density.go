package levelbuild

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty/internal/project"
	worldgeometry "github.com/karty-game/karty/internal/worldbuild/geometry"
	worldsource "github.com/karty-game/karty/internal/worldbuild/source"
)

const defaultSpriteTexelsPerUnit = 128.0

// Density is an authoring preference resolved into existing image recipes.
// Geometry, original artwork and runtime formats do not change.
func prepareSpriteDensity(directory string, definition manifest, assets *assetBuild) (manifest, error) {
	enabled := definition.World.MaxSpriteTexelsPerUnit == nil
	explicitPositive := false
	values := make([]*float64, 0, 1+len(definition.Textures))
	values = append(values, definition.World.MaxSpriteTexelsPerUnit)

	for _, entry := range definition.Textures {
		values = append(values, entry.MaxSpriteTexelsPerUnit)
	}

	for _, value := range values {
		if value == nil {
			continue
		}

		if math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 || *value > asset.MaxTextureDimension {
			return definition, fmt.Errorf("max_sprite_texels_per_unit must be finite in [0,%d]: %w", asset.MaxTextureDimension, ErrManifest)
		}

		enabled = enabled || *value > 0
		explicitPositive = explicitPositive || *value > 0
	}

	if !enabled {
		return definition, nil
	}

	if definition.World.Source == "" || assets == nil {
		if explicitPositive {
			return definition, fmt.Errorf("sprite density requires a world source and asset processing: %w", ErrManifest)
		}

		return definition, nil
	}

	if definition.World.MaxSpriteTexelsPerUnit == nil {
		defaultDensity := defaultSpriteTexelsPerUnit
		definition.World.MaxSpriteTexelsPerUnit = &defaultDensity
	}

	expanded, err := worldsource.Load(directory, definition.World.Source)
	if err != nil {
		return definition, err
	}

	sorted := slices.Clone(definition.Textures)
	slices.SortFunc(sorted, func(a, b textureEntry) int { return strings.Compare(a.Name, b.Name) })

	ids := make(map[string]uint32, len(sorted))
	for index, entry := range sorted {
		ids[entry.Name] = uint32(index + 1)
	}

	document, err := worldgeometry.Compile(expanded, ids)
	if err != nil {
		return definition, err
	}

	return applySpriteDensity(definition, document, ids, assets)
}

type spriteDensityBounds struct{ width, height float64 }

func spriteTextureBounds(document world.Document) map[uint32]spriteDensityBounds {
	result := make(map[uint32]spriteDensityBounds)
	presets := make(map[string][]uint32)

	if document.Animations != nil {
		for _, preset := range document.Animations.Presets {
			if preset.Kind == "flipbook" {
				presets[preset.Name] = preset.Frames
			}
		}
	}

	for _, content := range document.Contents {
		actor := content.Actor
		if actor == nil || actor.Sprite == nil {
			continue
		}

		sprite := actor.Sprite
		widthScale := actor.Scale.X

		if sprite.Facing == world.SpriteCross {
			widthScale = min(widthScale, actor.Scale.Y)
		}

		bounds := spriteDensityBounds{sprite.Width * widthScale, sprite.Height * actor.Scale.Z}
		ids := []uint32{sprite.AssetID}

		if actor.Animation != nil {
			ids = append(ids, presets[actor.Animation.Preset]...)
		}

		for _, id := range ids {
			if previous, found := result[id]; found {
				boundsForID := spriteDensityBounds{min(previous.width, bounds.width), min(previous.height, bounds.height)}
				result[id] = boundsForID
			} else {
				result[id] = bounds
			}
		}
	}

	return result
}

func applySpriteDensity(definition manifest, document world.Document, ids map[string]uint32, assets *assetBuild) (manifest, error) {
	bounds := spriteTextureBounds(document)
	materials := world.MaterialIDs(&document)
	definition.Textures = slices.Clone(definition.Textures)

	for index := range definition.Textures {
		entry := &definition.Textures[index]
		density := definition.World.MaxSpriteTexelsPerUnit

		if entry.MaxSpriteTexelsPerUnit != nil {
			density = entry.MaxSpriteTexelsPerUnit
		}

		if density == nil || *density == 0 {
			continue
		}

		id := ids[entry.Name]
		physical, used := bounds[id]

		if !used || slices.Contains(materials, id) {
			if entry.MaxSpriteTexelsPerUnit != nil {
				return definition, fmt.Errorf(
					"texture %q density requires an exclusively sprite texture; use a separate texture name for surfaces: %w",
					entry.Name, ErrManifest)
			}

			continue
		}

		width := math.Floor(physical.width * *density)
		height := math.Floor(physical.height * *density)

		if width < 1 || height < 1 {
			return definition, fmt.Errorf("texture %q density would require less than one texel: %w", entry.Name, ErrManifest)
		}

		profileName := entry.Profile
		if profileName == "" {
			profileName = project.DefaultTextureProfile
		}

		profile := assets.manifest.Assets.TextureProfiles[profileName]
		entry.Transform.MaxWidth = spriteDensityLimit(entry.Transform.MaxWidth, profile.Transform.MaxWidth, width)
		entry.Transform.MaxHeight = spriteDensityLimit(entry.Transform.MaxHeight, profile.Transform.MaxHeight, height)
	}

	return definition, nil
}

func spriteDensityLimit(authored, inherited uint32, densityLimit float64) uint32 {
	limit := authored
	if limit == 0 {
		limit = inherited
	}
	// Clamp before converting: valid world scales can exceed uint32 range.
	densityPixels := uint32(min(densityLimit, float64(asset.MaxTextureDimension)))
	if limit == 0 {
		return densityPixels
	}

	return min(limit, densityPixels)
}
