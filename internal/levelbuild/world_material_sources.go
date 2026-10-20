package levelbuild

import (
	"fmt"
	"slices"
	"strings"

	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty-sdk/format/worldparticles"
	"github.com/karty-game/karty-sdk/format/worldvegetation"
	ui "github.com/karty-game/karty-ui/schema"
	"github.com/karty-game/karty/internal/assetpipeline"
)

// Atlas interiors already contain the world material pixels. Keep standalone
// entries only when they are not atlas materials or have another runtime use.
// Run after UI compilation and chunk generation, which still need the sources.
func omitAtlasSourceTextures(entries []level.SourceEntry) ([]level.SourceEntry, error) {
	var layoutBytes, worldBytes []byte

	for _, entry := range entries {
		switch entry.Name {
		case worldmaterial.LayoutEntry:
			layoutBytes = entry.Data
		case world.EntryName:
			worldBytes = entry.Data
		}
	}

	if layoutBytes == nil || worldBytes == nil {
		return entries, nil
	}

	layout, err := worldmaterial.DecodeLayout(layoutBytes)
	if err != nil {
		return nil, err
	}

	document, err := world.Decode(worldBytes)
	if err != nil {
		return nil, err
	}

	omit := make(map[uint32]bool, len(layout.Materials))
	for _, material := range layout.Materials {
		omit[material.MaterialID] = true
	}

	for _, content := range document.Contents {
		if content.Actor != nil && content.Actor.Sprite != nil {
			delete(omit, content.Actor.Sprite.AssetID)
		}
	}

	if document.Animations != nil {
		for _, preset := range document.Animations.Presets {
			for _, id := range preset.Frames {
				delete(omit, id)
			}
		}
	}

	for _, entry := range entries {
		if err := retainAtlasTextureConsumers(omit, entry); err != nil {
			return nil, fmt.Errorf("level texture consumers %q: %w", entry.Name, err)
		}
	}

	result := make([]level.SourceEntry, 0, len(entries))
	for _, entry := range entries {
		id, texture := level.TextureAssetID(entry.Name)
		if entry.Kind != level.EntryTexture || !texture || !omit[id] {
			result = append(result, entry)
		}
	}

	return result, nil
}

func retainAtlasTextureConsumers(omit map[uint32]bool, entry level.SourceEntry) error {
	switch {
	case entry.Name == worldparticles.EntryName:
		document, err := worldparticles.Decode(entry.Data)
		if err != nil {
			return err
		}

		for _, preset := range document.Presets {
			for _, id := range preset.Sprite.Frames {
				delete(omit, id)
			}
		}
	case entry.Name == worldvegetation.EntryName:
		document, err := worldvegetation.Decode(entry.Data)
		if err != nil {
			return err
		}

		for _, preset := range document.Presets {
			delete(omit, preset.Texture)
			delete(omit, preset.NormalTexture)
		}
	case strings.HasPrefix(entry.Name, ui.AssetPrefix):
		template, err := ui.DecodeComposition(entry.Data)
		if err != nil {
			return err
		}

		retainAtlasStyleTextures(omit, template.Panel)
		retainAtlasStyleTextures(omit, template.ResponsivePanel)

		for _, element := range template.Elements {
			retainAtlasStyleTextures(omit, element.Style)
			retainAtlasStyleTextures(omit, element.ResponsiveStyle)
		}
	}

	return nil
}

func retainAtlasStyleTextures(omit map[uint32]bool, style ui.Style) {
	for _, image := range []ui.Image{style.BackgroundImage, style.BackgroundImageHover,
		style.BackgroundImagePressed, style.BackgroundImageDisabled, style.BackgroundImageFocus} {
		if image.Scope == ui.ImageScopeLevel {
			delete(omit, image.AssetID)
		}
	}
}

// Build inputs are cached even when the atlas is their only runtime output.
// Reports and decoded-memory gates must count only the packaged representation.
func packagedTextureReports(
	processed []assetpipeline.Texture,
	metadata []metadataTexture,
	entries []level.SourceEntry,
) []assetpipeline.Texture {
	ids := make(map[string]uint32, len(metadata))
	for _, texture := range metadata {
		ids[texture.Name] = texture.ID
	}

	packaged := make(map[uint32]bool)

	for _, entry := range entries {
		if entry.Kind == level.EntryTexture {
			id, _ := level.TextureAssetID(entry.Name)
			packaged[id] = true
		}
	}

	return slices.DeleteFunc(slices.Clone(processed), func(texture assetpipeline.Texture) bool {
		return texture.Kind == "texture" && !packaged[ids[texture.Name]]
	})
}
