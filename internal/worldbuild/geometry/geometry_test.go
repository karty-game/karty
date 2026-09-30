//nolint:wsl_v5 // Assertions intentionally stay adjacent to the compiled fixture they inspect.
package geometry_test

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"testing"

	sdkworld "github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty/internal/worldbuild/geometry"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

func TestCompileTriangulatesConcaveRoomsAndBuildsReciprocalPortals(t *testing.T) {
	t.Parallel()

	expanded := source.Expanded{
		Rooms: []source.Room{
			{
				ID: "hall", SourceRoom: "hall", Boundary: edges([]point{
					{0, 0}, {4, 0}, {4, 2}, {2, 2}, {2, 4}, {0, 4},
				}, []string{"south", "door", "inset-south", "inset-east", "north", "west"}),
				Floor: worldsource.Plane{A: .05}, Ceiling: worldsource.Plane{B: -.05, C: 4},
				FloorMaterial: "floor", CeilingMaterial: "ceiling",
				Contents: []source.Content{{
					ID: "hall/spawn", SourceID: "spawn", Kind: "spawn", Position: worldsource.Vec3{X: 1, Y: 1, Z: 1},
					Actor: &source.Actor{
						Scale: worldsource.Vec3{X: 1, Y: 1, Z: 1}, Tags: []string{"player"},
						Sprite: &worldsource.Sprite{
							Texture: "actor", Facing: "upright", Alpha: "blend",
							Width: 1, Height: 2, OriginX: .5, OriginY: 1,
						},
					},
				}},
			},
			{
				ID: "repeat/room", SourceRoom: "room", Instance: "repeat",
				Boundary: edges([]point{{4, 0}, {6, 0}, {6, 2}, {4, 2}}, []string{"south", "east", "north", "entrance"}),
				Floor:    worldsource.Plane{}, Ceiling: worldsource.Plane{C: 4},
				FloorMaterial: "floor", CeilingMaterial: "ceiling",
			},
		},
		Connections: []source.Connection{{
			ID: "hall-to-repeat",
			A:  source.Endpoint{Room: "hall", Edge: "door"},
			B:  source.Endpoint{Room: "repeat/room", Edge: "entrance"},
		}},
	}

	document, err := geometry.Compile(expanded, map[string]uint32{"floor": 1, "ceiling": 2, "wall": 3, "actor": 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Sectors) != 3 || len(document.Contents) != 1 {
		t.Fatalf("compiled world = %d sectors, %d contents", len(document.Sectors), len(document.Contents))
	}
	if document.Contents[0].ID != "hall/spawn" || document.Contents[0].SourceID != "spawn" {
		t.Fatalf("compiled content = %+v", document.Contents[0])
	}
	if actor := document.Contents[0].Actor; actor == nil || actor.Sprite == nil || actor.Sprite.AssetID != 4 ||
		actor.Sprite.Facing != sdkworld.SpriteUpright || actor.Sprite.Alpha != sdkworld.SpriteBlend || !slices.Equal(actor.Tags, []string{"player"}) {
		t.Fatalf("compiled actor = %+v", actor)
	}
	portalWalls := 0
	roomSectors := map[string]int{}
	roomDoubleArea := map[string]float64{}
	sourceEdges := map[string]int{}
	for sectorIndex, sector := range document.Sectors {
		roomSectors[sector.SourceRoom]++
		for _, wall := range sector.Walls {
			roomDoubleArea[sector.SourceRoom] += wall.Start.X*wall.End.Y - wall.End.X*wall.Start.Y
			if wall.SourceEdge != "" {
				sourceEdges[sector.SourceRoom+"/"+wall.SourceEdge]++
			}
			if wall.Portal >= 0 {
				portalWalls++
				if int(wall.Portal) == sectorIndex {
					t.Fatal("self portal")
				}
			}
		}
	}
	if portalWalls != 4 {
		t.Fatalf("portal walls = %d, want 4", portalWalls)
	}
	if roomSectors["hall"] != 2 || roomSectors["room"] != 1 ||
		roomDoubleArea["hall"] != 24 || roomDoubleArea["room"] != 8 {
		t.Fatalf("merged rooms = sectors %v, double areas %v", roomSectors, roomDoubleArea)
	}
	for _, edge := range []string{
		"hall/south", "hall/door", "hall/inset-south", "hall/inset-east", "hall/north", "hall/west",
		"room/south", "room/east", "room/north", "room/entrance",
	} {
		if sourceEdges[edge] != 1 {
			t.Fatalf("authored edge %q appears %d times", edge, sourceEdges[edge])
		}
	}

	first, err := sdkworld.Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	secondDocument, err := geometry.Compile(expanded, map[string]uint32{"wall": 3, "ceiling": 2, "floor": 1, "actor": 4})
	if err != nil {
		t.Fatal(err)
	}
	second, err := sdkworld.Encode(secondDocument)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("compiled world is not deterministic")
	}
}

type point struct{ x, y float64 }

func edges(points []point, names []string) []worldsource.Edge {
	result := make([]worldsource.Edge, len(points))
	for index, value := range points {
		next := points[(index+1)%len(points)]
		result[index] = worldsource.Edge{
			ID: names[index], Start: worldsource.Vec2{X: value.x, Y: value.y},
			End: worldsource.Vec2{X: next.x, Y: next.y}, Material: "wall",
		}
	}

	return result
}

func TestCompileDirectedNonEuclideanPortalGraph(t *testing.T) {
	t.Parallel()

	rooms := []source.Room{
		{
			ID:              "a",
			SourceRoom:      "a",
			Boundary:        edges([]point{{0, 0}, {2, 0}, {2, 2}, {0, 2}}, []string{"south", "east", "north", "west"}),
			FloorMaterial:   "floor",
			CeilingMaterial: "ceiling",
			Ceiling:         worldsource.Plane{C: 4},
		},
		{
			ID:              "b",
			SourceRoom:      "b",
			Boundary:        edges([]point{{10, 10}, {12, 10}, {12, 12}, {10, 12}}, []string{"south", "east", "north", "west"}),
			FloorMaterial:   "floor",
			CeilingMaterial: "ceiling",
			Ceiling:         worldsource.Plane{C: 4},
		},
	}
	expanded := source.Expanded{Rooms: rooms, Connections: []source.Connection{{
		ID: "a-to-b", A: source.Endpoint{Room: "a", Edge: "north"}, B: source.Endpoint{Room: "b", Edge: "north"},
		Direction: worldsource.PortalAToB, NonEuclidean: true,
	}}}
	document, err := geometry.Compile(expanded, map[string]uint32{"floor": 1, "ceiling": 2, "wall": 3})
	if err != nil {
		t.Fatal(err)
	}
	wallA := findSourceWall(t, document, "a")
	wallB := findSourceWall(t, document, "b")
	if wallA.Portal < 0 || wallA.PortalWall == 0 || wallB.Portal >= 0 || wallB.PortalWall != 0 {
		t.Fatalf("one-way walls: wallA=%+v wallB=%+v", wallA, wallB)
	}

	expanded.Connections = append(expanded.Connections, source.Connection{
		ID: "b-to-a", A: source.Endpoint{Room: "b", Edge: "north"}, B: source.Endpoint{Room: "a", Edge: "north"},
		Direction: worldsource.PortalAToB, NonEuclidean: true,
	})
	document, err = geometry.Compile(expanded, map[string]uint32{"floor": 1, "ceiling": 2, "wall": 3})
	if err != nil {
		t.Fatal(err)
	}
	wallA = findSourceWall(t, document, "a")
	wallB = findSourceWall(t, document, "b")
	if wallA.Portal < 0 || wallB.Portal < 0 || wallA.PortalWall == 0 || wallB.PortalWall == 0 {
		t.Fatalf("independent outgoing walls: wallA=%+v wallB=%+v", wallA, wallB)
	}
}

func TestCompileCountsActorSpritesInMaterialLimit(t *testing.T) {
	t.Parallel()
	expanded := source.Expanded{}
	materials := make(map[string]uint32, geometry.MaxMaterials+1)
	for index := range 65 {
		x := float64(index * 3)
		floor, ceiling, wall := fmt.Sprintf("floor-%d", index), fmt.Sprintf("ceiling-%d", index), fmt.Sprintf("wall-%d", index)
		boundary := edges([]point{{x, 0}, {x + 2, 0}, {x + 2, 2}, {x, 2}}, []string{"south", "east", "north", "west"})
		for edge := range boundary {
			boundary[edge].Material = wall
		}
		expanded.Rooms = append(expanded.Rooms, source.Room{
			ID: fmt.Sprintf("room-%d", index), SourceRoom: fmt.Sprintf("room-%d", index), Boundary: boundary,
			FloorMaterial: floor, CeilingMaterial: ceiling, Ceiling: worldsource.Plane{C: 4},
		})
		materials[floor], materials[ceiling], materials[wall] = uint32(index*3+1), uint32(index*3+2), uint32(index*3+3)
	}
	actor := func(id, texture string, x float64) source.Content {
		return source.Content{ID: id, SourceID: id, Kind: "actor", Position: worldsource.Vec3{X: x, Y: 1}, Actor: &source.Actor{
			Scale: worldsource.Vec3{
				X: 1,
				Y: 1,
				Z: 1,
			},
			Sprite: &worldsource.Sprite{Texture: texture, Facing: "upright", Alpha: "cutout", Width: 1, Height: 1},
		}}
	}
	materials["sprite-196"] = 196
	expanded.Rooms[0].Contents = append(expanded.Rooms[0].Contents, actor("first", "sprite-196", 1))
	if _, err := geometry.Compile(expanded, materials); err != nil {
		t.Fatalf("196 materials: %v", err)
	}
	materials["sprite-197"] = 197
	expanded.Rooms[0].Contents = append(expanded.Rooms[0].Contents, actor("second", "sprite-197", 1.5))
	if _, err := geometry.Compile(expanded, materials); !errors.Is(err, geometry.ErrMaterial) {
		t.Fatalf("197 materials error = %v", err)
	}
}

func findSourceWall(t *testing.T, document sdkworld.Document, room string) sdkworld.Wall {
	t.Helper()
	for _, sector := range document.Sectors {
		if sector.SourceRoom != room {
			continue
		}
		for _, wall := range sector.Walls {
			if wall.SourceEdge == "north" {
				return wall
			}
		}
	}
	t.Fatalf("missing %s/north", room)

	return sdkworld.Wall{}
}
