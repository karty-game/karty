package levelbuild

import (
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
)

func TestAnimationFrameDimensionsAndSDKGate(t *testing.T) {
	t.Parallel()

	document := world.Document{
		Animations: &world.Animations{
			Version: 1,
			Presets: []world.AnimationPreset{{Name: "glitch", Kind: "flipbook", Frames: []uint32{1, 2}}},
		},
	}

	textures := []metadataTexture{{ID: 1, Width: 32, Height: 16}, {ID: 2, Width: 32, Height: 16}}
	if err := validateAnimationFrames(document, textures); err != nil {
		t.Fatal(err)
	}

	textures[1].Height = 32
	if err := validateAnimationFrames(document, textures); err == nil {
		t.Fatal("mismatched frame dimensions accepted")
	}

	if _, err := worldAnimationFeatures(nil, document); err == nil {
		t.Fatal("animation accepted without SDK capability")
	}
}
