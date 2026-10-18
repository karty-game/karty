package source_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/worldbuild/geometry"
	"github.com/karty-game/karty/internal/worldbuild/schema"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

func TestCompactRoomCompilesToTheSameWorld(t *testing.T) {
	t.Parallel()

	const (
		prefix   = "version: 19\nrooms:\n  - id: room\n    ceiling: {c: 3}\n    floor_material: stone\n    ceiling_material: stone\n"
		compact  = prefix + "    rect: {width: 8, depth: 6, material: stone}\n"
		explicit = prefix + `    boundary:
      - {id: south, start: {x: 0, y: 0}, end: {x: 8, y: 0}, material: stone}
      - {id: east, start: {x: 8, y: 0}, end: {x: 8, y: 6}, material: stone}
      - {id: north, start: {x: 8, y: 6}, end: {x: 0, y: 6}, material: stone}
      - {id: west, start: {x: 0, y: 6}, end: {x: 0, y: 0}, material: stone}
`
	)

	compactWorld, err := source.Decode([]byte(compact))
	if err != nil {
		t.Fatal(err)
	}

	explicitWorld, err := source.Decode([]byte(explicit))
	if err != nil {
		t.Fatal(err)
	}

	actual, err := geometry.Compile(compactWorld, map[string]uint32{"stone": 1})
	if err != nil {
		t.Fatal(err)
	}

	want, err := geometry.Compile(explicitWorld, map[string]uint32{"stone": 1})
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(actual, want) {
		t.Fatal("compact source changed compiled geometry or identity")
	}

	for _, input := range []string{compact, strings.Replace(compact, "version: 19", "version: 18", 1), strings.Replace(compact, "material: stone}", "material: missing}", 1), strings.Replace(compact, "width: 8", "width: 0", 1), compact + "    boundary: []\n"} {
		value, err := schema.Parse([]byte(input))
		if err != nil {
			t.Fatal(err)
		}

		err = schema.Validate(value, schema.Level(value, []string{"stone"}))
		if (err == nil) != (input == compact) {
			t.Fatalf("schema disagrees for %s: %v", input, err)
		}
	}
}
