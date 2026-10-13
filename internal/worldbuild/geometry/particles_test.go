package geometry_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/worldparticles"
	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty/internal/worldbuild/geometry"
	"github.com/karty-game/karty/internal/worldbuild/schema"
	"github.com/karty-game/karty/internal/worldbuild/source"
	"github.com/pelletier/go-toml/v2"
)

func authoredParticle() worldsource.ParticlePreset {
	return worldsource.ParticlePreset{
		Name:            "spark",
		Capacity:        4,
		Emission:        worldparticles.Emission{Rate: 4, DurationSeconds: 1},
		LifetimeSeconds: worldparticles.Range{Min: .2, Max: .5},
		Spawn:           worldsource.ParticleSpawn{Shape: "point"},
		VelocityMin:     worldsource.Vec3{Z: .1},
		VelocityMax:     worldsource.Vec3{Z: .2},
		Sprite: worldsource.ParticleSprite{
			Frames:   []string{"spark"},
			Facing:   "camera-facing",
			Blend:    "additive",
			Playback: "cycle",
		},
		SizeOverLife:  []worldparticles.ScalarKey{{Age: 0, Value: .1}, {Age: 1, Value: 0}},
		ColorOverLife: []worldparticles.ColorKey{{Age: 0, RGBA: [4]float64{0, 1, 0, 1}}, {Age: 1, RGBA: [4]float64{0, 1, 0, 0}}},
	}
}

func TestCompileParticlesResolvesFramesAndRejectsEscapingEnvelope(t *testing.T) {
	t.Parallel()

	expanded := lightingExpanded(0)
	p := authoredParticle()
	expanded.Particles = &worldsource.Particles{Version: 1, Presets: []worldsource.ParticlePreset{p}}
	expanded.ParticleEmitters = []worldsource.ParticleEmitter{
		{
			ID:       "pool",
			Preset:   "spark",
			Position: worldsource.Vec3{X: 1, Y: 1, Z: 1},
			Scale:    1,
			Seed:     42,
			Playback: worldparticles.Playback{Mode: "cycle", PeriodSeconds: 1},
		},
	}

	compiledWorld, err := geometry.Compile(expanded, map[string]uint32{"wall": 1})
	if err != nil {
		t.Fatal(err)
	}

	particles, err := geometry.CompileParticles(expanded, &compiledWorld, map[string]uint32{"wall": 1, "spark": 2})
	if err != nil {
		t.Fatal(err)
	}

	if particles.Presets[0].Sprite.Frames[0] != 2 || particles.Emitters[0].Sector != 0 {
		t.Fatal(particles)
	}

	particles.Presets[0].SizeOverLife[0].Value = .2

	if expanded.Particles.Presets[0].SizeOverLife[0].Value != .1 {
		t.Fatal("compiled curve aliases authoring")
	}

	expanded.ParticleEmitterRooms = map[string]string{"pool": "other-room"}
	if _, err := geometry.CompileParticles(
		expanded,
		&compiledWorld,
		map[string]uint32{"wall": 1, "spark": 2},
	); !errors.Is(
		err,
		worldparticles.ErrConfinement,
	) || !strings.Contains(err.Error(), `in room "other-room"`) || !strings.Contains(err.Error(), "reduce spawn extent") {
		t.Fatalf("room emitter escaped declared owner: %v", err)
	}

	expanded.ParticleEmitterRooms = nil
	if _, err := geometry.CompileParticles(expanded, &compiledWorld, map[string]uint32{"wall": 1}); err == nil {
		t.Fatal("undeclared frame accepted")
	}

	expanded.ParticleEmitters[0].Position.X = .01

	if _, err := geometry.CompileParticles(
		expanded,
		&compiledWorld,
		map[string]uint32{"spark": 2},
	); !errors.Is(
		err,
		worldparticles.ErrConfinement,
	) {
		t.Fatalf("escaped billboard accepted: %v", err)
	}
}

func TestHangarParticleExamplesFitCompiledSectors(t *testing.T) {
	t.Parallel()

	directory := filepath.Join("..", "..", "..", "samples", "hangar", "levels", "hangar")

	input, err := os.ReadFile(filepath.Join(directory, "level.toml"))
	if err != nil {
		t.Fatal(err)
	}

	var manifest struct {
		Textures []struct {
			Name   string `toml:"name"`
			Source string `toml:"source"`
		} `toml:"textures"`
	}

	if err := toml.Unmarshal(input, &manifest); err != nil {
		t.Fatal(err)
	}

	textures := map[string]uint32{}
	textureSources := map[string]string{}

	for i, texture := range manifest.Textures {
		textures[texture.Name] = uint32(i + 1)
		textureSources[texture.Name] = texture.Source
	}

	expanded, err := source.Load(directory, "world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	compiledWorld, err := geometry.Compile(expanded, textures)
	if err != nil {
		t.Fatal(err)
	}

	particles, err := geometry.CompileParticles(expanded, &compiledWorld, textures)
	if err != nil {
		t.Fatal(err)
	}

	if particles.Version != worldparticles.AlphaVersion || len(particles.Presets) != 14 || len(particles.Emitters) != 130 {
		t.Fatalf(
			"shared fire/discharge/acid catalog: version %d, %d presets, %d emitters",
			particles.Version,
			len(particles.Presets),
			len(particles.Emitters),
		)
	}

	checkHangarSharedEffectResources(t, directory, expanded, textureSources)
	checkHangarBarrelSmoke(t, particles)

	for _, emitter := range particles.Emitters {
		if !worldparticles.BoundsInSector(emitter.Bounds, &compiledWorld.Sectors[emitter.Sector]) {
			t.Fatalf("emitter %s escapes", emitter.ID)
		}
	}

	encoded, err := worldparticles.Encode(*particles)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := worldparticles.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	if err := worldparticles.ValidateForWorld(&decoded, &compiledWorld); err != nil {
		t.Fatal(err)
	}
}

func checkHangarSharedEffectResources(t *testing.T, directory string, expanded source.Expanded, textures map[string]string) {
	t.Helper()

	libraryPath := filepath.Join("..", "..", "..", "samples", "particle-lab", "effects", "library.yaml")
	library := decodeParticleLabDocument(t, readParticleLabFile(t, libraryPath))
	presets := map[string]worldsource.ParticlePreset{}
	frames := map[string]bool{}

	for _, preset := range library.Particles.Presets {
		presets[preset.Name] = preset
	}

	adaptations := 0

	for _, preset := range expanded.Particles.Presets {
		expected := presets[preset.Name]
		// Only the Hangar importer owns these two barrel adaptations. Compare
		// every field against the library after applying that narrow contract;
		// vertical spawn, motion, lifetimes, colors and texture references remain
		// identical, with the deliberate tongue size increase checked exactly.
		switch preset.Name {
		case "hangar-barrel-flamesmoke":
			expected = presets["brackeys-fire-combined-flamesmoke-01"]
			expected.Name = preset.Name
			expected.Spawn.Shape = "box"
			expected.Spawn.HalfExtent.X, expected.Spawn.HalfExtent.Y = .24, .18
			adaptations++
		case "hangar-barrel-tongues":
			expected = presets["lab-fire-shaped-tongues"]
			expected.Name = preset.Name
			expected.Spawn.HalfExtent.X, expected.Spawn.HalfExtent.Y = .24, .18

			expected.SizeOverLife = append([]worldparticles.ScalarKey(nil), expected.SizeOverLife...)
			for index := range expected.SizeOverLife {
				expected.SizeOverLife[index].Value *= 1.8
			}

			adaptations++
		}

		if !reflect.DeepEqual(preset, expected) {
			t.Fatalf("Hangar preset %s diverged from the reusable library", preset.Name)
		}

		for _, frame := range preset.Sprite.Frames {
			frames[frame] = true
		}
	}

	if adaptations != 2 {
		t.Fatalf("expected exactly two importer-owned barrel adaptations, got %d", adaptations)
	}

	for _, animation := range expanded.Animations.Presets {
		if animation.Name == "industrial-fire-smoke" {
			for _, frame := range animation.Frames {
				frames[frame] = true
			}
		}
	}

	for frame := range frames {
		copied := readParticleLabFile(t, filepath.Join(directory, textures[frame]))
		original := readParticleLabOriginalTexture(t, particleLabRoot(), frame)

		if !bytes.Equal(copied, original) {
			t.Fatalf("Hangar texture %s diverged from its retained library asset", frame)
		}
	}
}

func checkHangarBarrelSmoke(t *testing.T, particles *worldparticles.Document) {
	t.Helper()

	count := 0

	for _, emitter := range particles.Emitters {
		if !strings.Contains(emitter.ID, "barrel-effects") {
			continue
		}

		if emitter.Bounds.Max.Z >= 2.60 {
			t.Fatalf("barrel layer %s reaches low ceiling: %v", emitter.ID, emitter.Bounds)
		}

		if emitter.Preset != "hangar-barrel-flamesmoke" {
			continue
		}

		count++

		for _, preset := range particles.Presets {
			if preset.Name == emitter.Preset && (preset.Sprite.Blend != "premultiplied" || len(preset.Sprite.Frames) != 1) {
				t.Fatal("barrel fire lost its reviewed single-image GPU flame/smoke layer")
			}
		}
	}

	if count != 1 {
		t.Fatalf("expected one reused barrel smoke emitter, got %d", count)
	}
}

func TestDocumentedParticleCycleAndOnceExamplesCompile(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "particles.md"))
	if err != nil {
		t.Fatal(err)
	}

	var examples []string

	remaining := string(data)
	for {
		_, after, found := strings.Cut(remaining, "```yaml\n")
		if !found {
			break
		}

		example, rest, _ := strings.Cut(after, "```")
		examples = append(examples, strings.TrimSpace(example))
		remaining = rest
	}

	if len(examples) != 3 {
		t.Fatalf("documented particle examples changed: %d", len(examples))
	}

	const room = `
rooms:
  - id: room
    boundary:
      - {id: a, start: {x: 0, y: 0}, end: {x: 10, y: 0}, material: wall}
      - {id: b, start: {x: 10, y: 0}, end: {x: 10, y: 10}, material: wall}
      - {id: c, start: {x: 10, y: 10}, end: {x: 0, y: 10}, material: wall}
      - {id: d, start: {x: 0, y: 10}, end: {x: 0, y: 0}, material: wall}
    ceiling: {c: 4}
    floor_material: wall
    ceiling_material: wall
`

	cycle := examples[0]
	once := strings.Replace(cycle, "name: acid-sparkle", "name: vent-sparks", 1)
	once = strings.Replace(once, "capacity: 48", "capacity: 24", 1)
	once = strings.Replace(
		once,
		"      emission: {rate: 12, duration_seconds: 3}",
		"      "+strings.ReplaceAll(examples[1], "\n", "\n      "),
		1,
	)
	once, _, _ = strings.Cut(once, "  emitters:")
	once += "  emitters:\n    " + strings.ReplaceAll(examples[2], "\n", "\n    ")

	for _, test := range []struct {
		input string
		mode  string
		count int
	}{{cycle, "cycle", 36}, {once, "once", 24}} {
		input := test.input + room

		value, err := schema.Parse([]byte(input))
		if err != nil {
			t.Fatal(err)
		}

		if err := schema.Validate(value, schema.Level(value, []string{"wall", "acid-sparkle"})); err != nil {
			t.Fatalf("documented %s schema: %v", test.mode, err)
		}

		expanded, err := source.Decode([]byte(input))
		if err != nil {
			t.Fatalf("documented %s YAML: %v", test.mode, err)
		}

		textures := map[string]uint32{"wall": 1, "acid-sparkle": 2}

		compiledWorld, err := geometry.Compile(expanded, textures)
		if err != nil {
			t.Fatal(err)
		}

		particles, err := geometry.CompileParticles(expanded, &compiledWorld, textures)
		if err != nil {
			t.Fatalf("documented %s envelope: %v", test.mode, err)
		}

		births, err := worldparticles.BirthSchedule(particles.Presets[0])
		if err != nil || len(births) != test.count || particles.Emitters[0].Playback.Mode != test.mode {
			t.Fatalf("documented %s schedule: %v, %v", test.mode, births, err)
		}
	}
}
