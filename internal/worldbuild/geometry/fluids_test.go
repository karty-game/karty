package geometry_test

import (
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty/internal/worldbuild/geometry"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

const fluidSource = `version: 17
rooms:
  - id: room
    floor: {c: -2}
    ceiling: {c: 4}
    floor_material: stone
    ceiling_material: stone
    boundary:
      - {id: a, start: {x: 0, y: 0}, end: {x: 4, y: 0}, material: stone}
      - {id: b, start: {x: 4, y: 0}, end: {x: 4, y: 4}, material: stone}
      - {id: c, start: {x: 4, y: 4}, end: {x: 0, y: 4}, material: stone}
      - {id: d, start: {x: 0, y: 4}, end: {x: 0, y: 0}, material: stone}
solids:
  - id: pool
    footprint: [{x: 0, y: 0}, {x: 4, y: 0}, {x: 4, y: 4}, {x: 0, y: 4}]
    bottom: {c: -2}
    top: {c: 0}
    side_material: stone
    top_material: water
    bottom_material: stone
    collision: false
animations:
  version: 1
  presets: [{name: ripple, kind: liquid, frequency: 1}]
  materials: [{material: water, preset: ripple}]
fluids:
  version: 1
  items: [{id: water, solids: [pool], density: 1000, foam_width: 0.2}]
`

func TestFluidAuthoringCompilesAndRejectsReferences(t *testing.T) {
	t.Parallel()

	for name, input := range map[string]string{"valid": fluidSource, "version16": strings.Replace(fluidSource, "version: 17", "version: 16", 1), "unknown": strings.Replace(fluidSource, "solids: [pool]", "solids: [missing]", 1), "null": strings.Replace(fluidSource, "density: 1000", "density: null", 1), "collision": strings.Replace(fluidSource, "collision: false", "collision: true", 1)} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			expanded, err := source.Decode([]byte(input))

			var compiled world.Document
			if err == nil {
				compiled, err = geometry.Compile(expanded, map[string]uint32{"stone": 1, "water": 2})
			}

			if name != "valid" {
				if err == nil {
					t.Fatal("invalid fluid authoring accepted")
				}

				return
			}

			if err != nil || compiled.Fluids == nil || compiled.Fluids.Items[0].Solids[0] != "pool" {
				t.Fatal("fluid compilation", err)
			}

			encoded, err := world.Encode(compiled)
			if err != nil || string(encoded[:4]) != "KWR3" {
				t.Fatal("compiled fluid format", err)
			}
		})
	}
}
