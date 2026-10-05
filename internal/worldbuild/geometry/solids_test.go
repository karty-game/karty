package geometry_test

import (
	"math"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty/internal/worldbuild/geometry"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

func TestStaticSolidFixtureDoesNotPartitionRooms(t *testing.T) {
	t.Parallel()

	encoded, err := os.ReadFile("testdata/static-solids.world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	expanded, err := source.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	materials := map[string]uint32{"floor": 1, "ceiling": 2, "wall": 3, "detail": 4}

	document, err := geometry.Compile(expanded, materials)
	if err != nil {
		t.Fatal(err)
	}

	if len(document.Sectors) != 1 || len(document.Sectors[0].Walls) != 4 || len(document.StaticSolids.Items) != 4 {
		t.Fatalf("detail changed graph: %+v", document)
	}

	shaft := document.StaticSolids.Items[0]
	if shaft.ID != "column-a/shaft" || len(shaft.Footprint) != 10 || !shaft.Collision || shaft.SideMaterial != 4 {
		t.Fatalf("shaft lost fields: %+v", shaft)
	}

	square := expanded
	square.Solids = append(square.Solids[:0:0], expanded.Solids...)
	square.Solids[0].Footprint = square.Solids[1].Footprint

	other, err := geometry.Compile(square, materials)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(document.Sectors, other.Sectors) {
		t.Fatal("square vs decagon changed sector graph")
	}

	document.StaticSolids.Items[0].Footprint[0].X = 999

	document.StaticSolids.Items[0].TopUV.Projections[0].U.X = 999
	if expanded.Solids[0].Footprint[0].X == 999 {
		t.Fatal("compiled footprint aliases source")
	}

	if _, err := world.Encode(other); err != nil {
		t.Fatal(err)
	}
}

func TestNestedDetailAndSpriteOnlyPrefabsTransform(t *testing.T) {
	t.Parallel()

	encoded, err := os.ReadFile("testdata/static-solids.world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	text := string(encoded)
	cut := strings.Index(text, "instances:\n")
	text = text[:cut] + `  - id: sprite-only
    contents:
      - id: sign
        kind: decoration
        position: {x: 0, y: 0, z: 1}
        actor:
          sprite: {texture: detail, facing: fixed, alpha: cutout, width: 1, height: 1}
  - id: compound
    instances:
      - id: nested-column
        prefab: column
      - id: nested-sprite
        prefab: sprite-only
instances:
  - id: compound-a
    prefab: compound
    transform: {translation: {x: 5, y: 5, z: 0}, yaw_degrees: 90, scale: 0.5}
    materials: [{from: detail, to: wall}]
`

	expanded, err := source.Decode([]byte(text))
	if err != nil {
		t.Fatal(err)
	}

	document, err := geometry.Compile(expanded, map[string]uint32{"floor": 1, "ceiling": 2, "wall": 3, "detail": 4})
	if err != nil {
		t.Fatal(err)
	}

	if len(document.Sectors) != 1 || len(document.StaticSolids.Items) != 1 || len(document.Contents) != 1 {
		t.Fatal("prefabs generated room geometry")
	}

	solid, actor := document.StaticSolids.Items[0], document.Contents[0]
	if math.Abs(solid.Footprint[0].X-5) > 1e-8 || math.Abs(solid.Footprint[0].Y-5.25) > 1e-8 || solid.Top.C != 2.5 ||
		solid.SideMaterial != 3 {
		t.Fatalf("solid transform lost: %+v", solid)
	}

	if actor.ID != "compound-a/nested-sprite/sign" || actor.SourceID != "sign" || actor.Instance != "compound-a/nested-sprite" ||
		actor.Position.Z != 0.5 ||
		actor.Actor.Scale.X != 0.5 ||
		math.Abs(actor.Actor.Yaw-math.Pi/2) > 1e-8 ||
		actor.Actor.Sprite.AssetID != 3 {
		t.Fatalf("sprite transform lost: %+v", actor)
	}

	outside := strings.Replace(text, "translation: {x: 5, y: 5, z: 0}", "translation: {x: 50, y: 50, z: 0}", 1)

	expanded, err = source.Decode([]byte(outside))
	if err != nil {
		t.Fatal(err)
	}

	rejected, err := geometry.Compile(expanded, map[string]uint32{"floor": 1, "ceiling": 2, "wall": 3, "detail": 4})
	if err == nil || len(rejected.Sectors) != 0 || rejected.StaticSolids != nil {
		t.Fatal("outside sprite published partial world")
	}
}

func TestAbsentSolidsPreserveSourceFiveCompiledBytes(t *testing.T) {
	t.Parallel()

	encoded, err := os.ReadFile("testdata/static-solids.world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	prefix := string(encoded[:strings.Index(string(encoded), "prefabs:\n")])

	compile := func(version string) []byte {
		expanded, err := source.Decode([]byte(strings.Replace(prefix, "version: 6", "version: "+version, 1)))
		if err != nil {
			t.Fatal(err)
		}

		document, err := geometry.Compile(expanded, map[string]uint32{"floor": 1, "ceiling": 2, "wall": 3})
		if err != nil {
			t.Fatal(err)
		}

		if document.StaticSolids != nil {
			t.Fatal("absent source gained solid payload")
		}

		result, err := world.Encode(document)
		if err != nil {
			t.Fatal(err)
		}

		return result
	}
	if !reflect.DeepEqual(compile("5"), compile("6")) {
		t.Fatal("absent solids altered source5 compiled encoding")
	}
}
