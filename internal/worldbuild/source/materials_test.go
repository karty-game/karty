package source

import (
	"strings"
	"testing"
)

const materialPrefabFixture = `version: 7
prefabs:
  - id: p
    rooms:
      - id: r
        floor: {a: 0, b: 0, c: 0}
        ceiling: {a: 0, b: 0, c: 2}
        floor_material: base
        ceiling_material: base
        wall_secondary: {texture: noise, strength: 0.4, uv: {scale: {x: 4, y: 6}}}
        wall_bands:
          top: {texture: template, height: 0.3}
          bottom: {texture: base, height: 0.5}
        boundary:
          - {id: a, start: {x: 0, y: 0}, end: {x: 2, y: 0}, material: base, bands: {top: {enabled: false}}, secondary: {strength: 0.2}}
          - {id: b, start: {x: 2, y: 0}, end: {x: 2, y: 2}, material: base}
          - {id: c, start: {x: 2, y: 2}, end: {x: 0, y: 2}, material: base}
          - {id: d, start: {x: 0, y: 2}, end: {x: 0, y: 0}, material: base}
instances:
  - id: moved
    prefab: p
    transform: {translation: {x: 10, y: 3, z: 1}, yaw_degrees: 90, scale: 2}
    materials:
      - {from: template, to: stone_top}
      - {from: noise, to: detail}
`

func TestMaterialPrefabOverridesAndPresence(t *testing.T) {
	t.Parallel()

	expanded, err := Decode([]byte(materialPrefabFixture))
	if err != nil {
		t.Fatal(err)
	}

	room := expanded.Rooms[0]

	edge := room.Boundary[0]
	if edge.Bands.Top.Texture != "stone_top" || *edge.Bands.Top.Height != 0.3 || *edge.Bands.Top.Enabled ||
		*edge.Bands.Bottom.Height != 0.5 ||
		edge.Bands.Bottom.Texture != "base" {
		t.Fatal("band field inheritance or substitution changed")
	}

	if edge.Secondary.Texture != "detail" || *edge.Secondary.Strength != 0.2 || edge.Secondary.UV.Scale.X != 4 ||
		edge.Secondary.UV.Scale.Y != 6 {
		t.Fatal("secondary inheritance or substitution changed")
	}

	if room.Floor.C != 1 || room.Ceiling.C != 5 {
		t.Fatal("prefab geometry was not transformed")
	}

	for _, fixture := range []string{
		strings.Replace(materialPrefabFixture, "version: 7", "version: 6", 1),
		strings.Replace(materialPrefabFixture, "texture: template", "texture: null", 1),
		strings.Replace(materialPrefabFixture, "height: 0.3", "height: null", 1),
		strings.Replace(materialPrefabFixture, "strength: 0.4", "strength: null", 1),
		strings.Replace(materialPrefabFixture, "top: {texture: template, height: 0.3}", "frame: obsolete", 1),
		strings.Replace(materialPrefabFixture, "top: {texture: template, height: 0.3}", "vertical: {width: 0.2}", 1),
	} {
		if _, err := Decode([]byte(fixture)); err == nil {
			t.Fatal("invalid material source presence accepted")
		}
	}
}
