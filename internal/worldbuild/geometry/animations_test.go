package geometry_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty/internal/worldbuild/geometry"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

func TestAnimationCompileResolvesEveryFrameAndOwnsBindings(t *testing.T) {
	t.Parallel()

	expanded := lightingExpanded(0)
	expanded.Animations = &worldsource.Animations{
		Version: 1,
		Presets: []worldsource.AnimationPreset{
			{Name: "glitch", Kind: "flipbook", Frames: []string{"wall", "glitch"}, FPS: 30, IntervalSeconds: 3},
		},
		Materials: []worldsource.MaterialAnimation{{Material: "wall", Preset: "glitch"}},
	}
	expanded.Rooms[0].Contents = []source.Content{
		{
			ID:       "screen",
			SourceID: "screen",
			Kind:     "actor",
			Position: worldsource.Vec3{X: 1, Y: 1, Z: 1},
			Actor: &source.Actor{
				Scale:     worldsource.Vec3{X: 1, Y: 1, Z: 1},
				Sprite:    &worldsource.Sprite{Texture: "wall", Facing: "fixed", Alpha: "cutout", Width: 1, Height: 1},
				Animation: &worldsource.AnimationBinding{Preset: "glitch", PhaseSeconds: .7},
			},
		},
	}

	compiled, err := geometry.Compile(expanded, map[string]uint32{"wall": 1, "glitch": 2})
	if err != nil {
		t.Fatal(err)
	}

	if compiled.Animations.Presets[0].Frames[1] != 2 || compiled.Contents[0].Actor.Animation.PhaseSeconds != .7 {
		t.Fatal(compiled)
	}

	compiled.Contents[0].Actor.Animation.PhaseSeconds = 1
	if expanded.Rooms[0].Contents[0].Actor.Animation.PhaseSeconds != .7 {
		t.Fatal("compiled binding aliases authoring")
	}

	if _, err := geometry.Compile(expanded, map[string]uint32{"wall": 1}); err == nil {
		t.Fatal("undeclared animation frame accepted")
	}
}

func TestHangarAnimationExampleCompiles(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "..", "samples", "hangar", "levels", "hangar")

	expanded, err := source.Load(root, "world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	names := []string{
		"olive",
		"ribbed",
		"brick",
		"floor",
		"grate",
		"ceiling",
		"concrete",
		"hazard",
		"steel",
		"crate",
		"recess",
		"toxic",
		"lamp", "reactor-lamp",
		"screen",
		"arrival-sign",
		"reactor-sign",
		"exit-sign",
		"screen-glitch-1",
		"screen-glitch-2",
		"screen-glitch-3",
		"fan-rotor-detailed",
		"piston",
		"wall-weathering",
		"rust-drips",
		"barrel-coals",
	}

	materials := map[string]uint32{}
	for i, n := range names {
		materials[n] = uint32(i + 1)

		sourceName := n + ".png"
		if n == "reactor-lamp" {
			sourceName = "lamp.png"
		}

		if _, err := os.Stat(filepath.Join(root, sourceName)); err != nil {
			t.Fatal(err)
		}
	}

	materials["acid-boundary"] = uint32(len(materials) + 1)

	compiled, err := geometry.Compile(expanded, materials)
	if err != nil {
		t.Fatal(err)
	}

	if len(compiled.Animations.Presets) != 5 || len(compiled.Animations.Materials) != 3 {
		t.Fatal("missing Hangar preset catalog")
	}

	animated := 0

	for _, c := range compiled.Contents {
		if c.Actor != nil && c.Actor.Animation != nil {
			animated++
		}
	}

	if animated != 13 {
		t.Fatalf("animated Hangar actors=%d", animated)
	}

	ids := world.MaterialIDs(&compiled)

	seen := map[uint32]bool{}
	for _, id := range ids {
		seen[id] = true
	}

	if !seen[materials["screen-glitch-3"]] {
		t.Fatal("last frame missing atlas")
	}

	if seen[materials["fan-rotor-detailed"]] {
		t.Fatal("sprite-only rotor entered atlas")
	}
}
