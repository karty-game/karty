package geometry_test

import (
	"os"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/worldbuild/geometry"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

func TestSolidRendererPrefabSharingAndSourceGates(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/static-solids.world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	input, _, _ := strings.Cut(string(data), "\ninstances:")
	input = strings.Replace(input, "version: 6", "version: 11", 1)
	input = strings.Replace(input, "collision: true", "collision: false", 1)
	input += "\ncontents:\n  - {id: can-a, kind: prop, position: {x: 2, y: 2, z: 1}, actor: {solid_renderer: column}}\n  - {id: can-b, kind: prop, position: {x: 8, y: 2, z: 1}, actor: {solid_renderer: column, roll_degrees: 90}}\n"

	expanded, err := source.Decode([]byte(input))
	if err != nil {
		t.Fatal(err)
	}

	compiled, err := geometry.Compile(expanded, map[string]uint32{"wall": 1, "floor": 2, "ceiling": 3, "detail": 4})
	if err != nil {
		t.Fatal(err)
	}

	if compiled.StaticSolids != nil || compiled.SolidRenderers == nil || len(compiled.SolidRenderers.Prefabs) != 1 ||
		len(compiled.SolidRenderers.Actors) != 2 {
		t.Fatalf("renderer geometry was flattened or duplicated: %+v", compiled.SolidRenderers)
	}

	if compiled.SolidRenderers.Prefabs[0].Solids[0].Footprint[0].X != .5 || compiled.Contents[0].Position.X != 2 ||
		compiled.Contents[1].Position.X != 8 {
		t.Fatal("instance transform changed immutable local geometry")
	}

	export, _, _ := strings.Cut(input, "\ncontents:")
	export = strings.Replace(export, "  - id: column", "  - id: column\n    renderable: true", 1)

	library, err := source.Decode([]byte(export))
	if err != nil {
		t.Fatal(err)
	}

	if len(library.SolidPrefabs) != 1 || len(library.Contents) != 0 {
		t.Fatal("runtime prefab export required an authored instance")
	}

	for _, bad := range []string{
		strings.Replace(input, "solid_renderer: column", "solid_renderer: null", 1),
		strings.Replace(input, "solid_renderer: column", `solid_renderer: ""`, 1),
		strings.Replace(input, "version: 11", "version: 10", 1),
		strings.Replace(input, "solid_renderer: column", "solid_renderer: unknown", 1),
		strings.Replace(input, "collision: false", "collision: true", 1),
	} {
		if _, err := source.Decode([]byte(bad)); err == nil {
			t.Fatal("invalid renderer reference or source version accepted")
		}
	}
}
