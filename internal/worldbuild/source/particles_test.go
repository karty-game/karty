package source_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

const particleSource = `version: 10
particles:
  version: 1
  presets:
    - name: sparkle
      capacity: 8
      emission: {rate: 4, duration_seconds: 1}
      lifetime_seconds: {min: 0.3, max: 0.5}
      spawn: {shape: point}
      sprite: {frames: [spark], playback: cycle, facing: camera-facing, blend: additive}
      size_over_life: [{age: 0, value: 0.05}, {age: 1, value: 0}]
      color_over_life: [{age: 0, rgba: [0, 1, 0, 1]}, {age: 1, rgba: [0, 1, 0, 0]}]
prefabs:
  - id: emitter
    particle_emitters:
      - {id: sparks, preset: sparkle, position: {x: 1, y: 0, z: 1}, yaw_degrees: 10, seed: 42, playback: {mode: cycle, period_seconds: 1}}
instances:
  - id: placed
    prefab: emitter
    transform: {translation: {x: 5, y: 6, z: 2}, yaw_degrees: 90, scale: 2}
rooms:
  - id: room
    boundary:
      - {id: a, start: {x: 0, y: 0}, end: {x: 10, y: 0}, material: wall}
      - {id: b, start: {x: 10, y: 0}, end: {x: 10, y: 10}, material: wall}
      - {id: c, start: {x: 10, y: 10}, end: {x: 0, y: 10}, material: wall}
      - {id: d, start: {x: 0, y: 10}, end: {x: 0, y: 0}, material: wall}
    ceiling: {c: 6}
    floor_material: wall
    ceiling_material: wall
    particle_emitters:
      - {id: pool, preset: sparkle, position: {x: 1, y: 1, z: 1}, seed: 17, playback: {mode: once}}
`

func TestParticleSourcePrefabTransformsAndCompleteValidation(t *testing.T) {
	t.Parallel()

	expanded, err := source.Decode([]byte(particleSource))
	if err != nil {
		t.Fatal(err)
	}

	if len(expanded.ParticleEmitters) != 2 || len(expanded.Contents) != 0 || expanded.ParticleEmitterRooms["room/pool"] != "room" {
		t.Fatal(expanded)
	}

	e := expanded.ParticleEmitters[1]

	if e.ID != "placed/sparks" || e.Position != (worldsource.Vec3{X: 5, Y: 8, Z: 4}) || e.YawDegrees != 100 || e.Scale != 2 {
		t.Fatalf("prefab transform: %+v", e)
	}

	for _, test := range []struct{ from, to string }{{"capacity: 8", "capacity: 3"}, {"preset: sparkle", "preset: missing"}, {"rate: 4", "rate: null"}, {"shape: point", "shape: point, oops: 0"}, {"mode: once", "mode: invalid"}} {
		_, err := source.Decode([]byte(strings.Replace(particleSource, test.from, test.to, 1)))
		if err == nil {
			t.Fatalf("accepted %s", test.to)
		}
	}

	for version := 1; version < 10; version++ {
		for _, field := range []string{"particles: null", "particle_emitters: null"} {
			input := fmt.Sprintf("version: %d\n%s\n", version, field)

			if _, err := source.Decode([]byte(input)); !errors.Is(err, worldsource.ErrVersion) {
				t.Fatalf("version/null gate v%d: %v", version, err)
			}
		}
	}

	for _, input := range []string{"version: 10\nparticles: null\n", "version: 10\nparticles: {version: 1, presets: null}\n", "version: 10\nrooms: [{id: room, particle_emitters: null}]\n"} {
		if _, err := source.Decode([]byte(input)); !errors.Is(err, worldsource.ErrContent) {
			t.Fatalf("null accepted: %v", err)
		}
	}
}

func TestParticleSourceBoundsExpandedPrefabEmitters(t *testing.T) {
	t.Parallel()

	header, _, _ := strings.Cut(particleSource, "instances:")

	_, rooms, _ := strings.Cut(particleSource, "\nrooms:")

	var instances strings.Builder
	for i := range 257 {
		fmt.Fprintf(&instances, "  - {id: placed%d, prefab: emitter}\n", i)
	}

	input := header + "instances:\n" + instances.String() + "rooms:" + rooms
	if _, err := source.Decode([]byte(input)); !errors.Is(err, source.ErrExpansion) {
		t.Fatalf("expanded particle limit not enforced: %v", err)
	}
}

func TestParticleSourceRequiresExplicitZeroValuedFields(t *testing.T) {
	t.Parallel()

	for _, field := range []string{
		"position: {x: 1, y: 0, z: 1}, ",
		"seed: 42, ",
		"age: 0, ",
		"value: 0.05",
		"rgba: [0, 1, 0, 1]",
		"min: 0.3, ",
	} {
		input := strings.Replace(particleSource, field, "", 1)
		if _, err := source.Decode([]byte(input)); !errors.Is(err, worldsource.ErrContent) {
			t.Fatalf("required particle field %q was accepted: %v", field, err)
		}
	}

	input := strings.Replace(particleSource, "emission: {rate: 4, duration_seconds: 1}", "emission: {bursts: [{count: 4}]}", 1)
	if _, err := source.Decode([]byte(input)); !errors.Is(err, worldsource.ErrContent) {
		t.Fatalf("missing zero-time burst field was accepted: %v", err)
	}
}

func TestParticleSourcePreservesZeroScaleAndOmittedVectorAxes(t *testing.T) {
	t.Parallel()

	input := strings.Replace(particleSource, "yaw_degrees: 10, seed:", "yaw_degrees: 10, scale: 0, seed:", 1)
	input = strings.Replace(input, "position: {x: 1, y: 0, z: 1}", "position: {z: 1}", 1)

	expanded, err := source.Decode([]byte(input))
	if err != nil {
		t.Fatal(err)
	}

	emitter := expanded.ParticleEmitters[1]
	if emitter.Scale != 2 || emitter.Position != (worldsource.Vec3{X: 5, Y: 6, Z: 4}) {
		t.Fatalf("source zero defaults changed during prefab composition: %+v", emitter)
	}
}
