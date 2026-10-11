package geometry

import (
	"fmt"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldsource"
)

func compileAnimationBinding(binding *worldsource.AnimationBinding) *world.AnimationBinding {
	if binding == nil {
		return nil
	}

	return &world.AnimationBinding{Preset: binding.Preset, PhaseSeconds: binding.PhaseSeconds}
}

func compileAnimations(
	document *world.Document,
	animations *worldsource.Animations,
	materials map[string]uint32,
	used map[uint32]struct{},
) error {
	if animations == nil {
		return nil
	}

	document.Animations = &world.Animations{Version: animations.Version}
	for _, preset := range animations.Presets {
		compiled, err := worldsource.CompileAnimationPreset(preset, materials)
		if err != nil {
			return err
		}

		document.Animations.Presets = append(document.Animations.Presets, compiled)
	}

	for _, binding := range animations.Materials {
		textureID, err := materialID(materials, binding.Material)
		if err != nil {
			return fmt.Errorf("animation material: %w", err)
		}

		document.Animations.Materials = append(
			document.Animations.Materials,
			world.MaterialAnimation{Material: textureID, Preset: binding.Preset, PhaseSeconds: binding.PhaseSeconds},
		)
		used[textureID] = struct{}{}
	}

	for _, preset := range document.Animations.Presets {
		for _, textureID := range preset.Frames {
			used[textureID] = struct{}{}
		}
	}

	return world.ValidateAnimations(document)
}
