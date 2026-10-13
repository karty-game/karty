package geometry_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty/internal/worldbuild/geometry"
	"github.com/karty-game/karty/internal/worldbuild/source"
	"github.com/pelletier/go-toml/v2"
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

	// Use the level's complete texture catalog, including the reused smoke
	// animation frames, rather than an older handwritten material list.
	materials := hangarAnimationMaterials(t, root)

	compiled, err := geometry.Compile(expanded, materials)
	if err != nil {
		t.Fatal(err)
	}

	if len(compiled.Animations.Presets) != len(expanded.Animations.Presets) || len(compiled.Animations.Materials) != 3 {
		t.Fatal("missing Hangar preset catalog")
	}

	animated := map[string]int{}

	for _, c := range compiled.Contents {
		if c.Actor != nil && c.Actor.Animation != nil {
			animated[c.Actor.Animation.Preset]++
		}
	}

	for preset, count := range map[string]int{"terminal-glitch": 7, "ventilation-fan": 3, "service-piston": 3} {
		if animated[preset] != count {
			t.Fatalf("Hangar %s actors=%d, want %d", preset, animated[preset], count)
		}
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

func hangarAnimationMaterials(t *testing.T, root string) map[string]uint32 {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(root, "level.toml"))
	if err != nil {
		t.Fatal(err)
	}

	var manifest struct {
		Textures []struct{ Name, Source string }
	}

	if err := toml.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}

	materials := make(map[string]uint32, len(manifest.Textures))
	for index, texture := range manifest.Textures {
		if _, err := os.Stat(filepath.Join(root, texture.Source)); err != nil {
			t.Fatal(err)
		}

		materials[texture.Name] = uint32(index + 1)
	}

	return materials
}
