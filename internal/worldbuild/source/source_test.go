//nolint:wsl_v5 // Assertions intentionally stay adjacent to the expanded fixture they inspect.
package source_test

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

func TestDecodeExpandsNestedPrefabTransformsAndMaterials(t *testing.T) {
	t.Parallel()

	expanded, err := source.Decode([]byte(`
version: 1
prefabs:
  - id: cell
    rooms:
      - id: room
        boundary:
          - {id: south, start: {x: 0, y: 0}, end: {x: 2, y: 0}, material: stone}
          - {id: east, start: {x: 2, y: 0}, end: {x: 2, y: 2}, material: stone}
          - {id: north, start: {x: 2, y: 2}, end: {x: 0, y: 2}, material: stone}
          - {id: west, start: {x: 0, y: 2}, end: {x: 0, y: 0}, material: stone}
        floor: {a: 0.25, b: 0, c: 0}
        ceiling: {a: 0.25, b: 0, c: 4}
        floor_material: floor
        ceiling_material: ceiling
        contents:
          - {id: spawn, kind: spawn, position: {x: 1, y: 1, z: 1}}
    ports:
      - id: entrance
        endpoint: {room: room, edge: west}
  - id: wrapper
    instances:
      - id: inner
        prefab: cell
        transform: {translation: {x: 3, y: 0, z: 1}, scale: 1}
    ports:
      - id: entrance
        endpoint: {instance: inner, port: entrance}
instances:
  - id: outer
    prefab: wrapper
    transform: {translation: {x: 10, y: 5, z: 0}, yaw_degrees: 90, scale: 2}
    materials:
      - {from: stone, to: brick}
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(expanded.Rooms) != 1 {
		t.Fatalf("expanded rooms = %d", len(expanded.Rooms))
	}
	room := expanded.Rooms[0]
	if room.ID != "outer/inner/room" || room.Instance != "outer/inner" || room.Boundary[0].Material != "brick" {
		t.Fatalf("expanded identity/material = %+v", room)
	}
	if room.Boundary[0].Start.X != 10 || room.Boundary[0].Start.Y != 11 || room.Floor.B != .25 || room.Floor.C != -.75 {
		t.Fatalf("transformed room = %+v", room)
	}
	if len(room.Contents) != 1 || room.Contents[0].Position.X != 8 || room.Contents[0].Position.Y != 13 ||
		room.Contents[0].Position.Z != 4 {
		t.Fatalf("transformed content = %+v", room.Contents)
	}
}

func TestDecodeRejectsUnknownFieldsAliasesAndTrailingDocuments(t *testing.T) {
	t.Parallel()

	for name, input := range map[string]string{
		"unknown":  "version: 1\nrooms: []\nunknown: true\n",
		"alias":    "version: 1\nrooms: &rooms []\nprefabs: []\ninstances: *rooms\n",
		"trailing": "version: 1\nrooms: []\n---\nversion: 1\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := source.Decode([]byte(input)); !errors.Is(err, source.ErrYAML) {
				t.Fatalf("Decode() error = %v", err)
			}
		})
	}
}

func TestDecodeComposesActorTransformAndInstanceTagOverride(t *testing.T) {
	t.Parallel()

	expanded, err := source.Decode([]byte(`
version: 2
prefabs:
  - id: grove
    rooms:
      - id: room
        boundary:
          - {id: south, start: {x: 0, y: 0}, end: {x: 2, y: 0}, material: bark}
          - {id: east, start: {x: 2, y: 0}, end: {x: 2, y: 2}, material: bark}
          - {id: north, start: {x: 2, y: 2}, end: {x: 0, y: 2}, material: bark}
          - {id: west, start: {x: 0, y: 2}, end: {x: 0, y: 0}, material: bark}
        floor: {c: 0}
        ceiling: {c: 4}
        floor_material: floor
        ceiling_material: ceiling
        contents:
          - id: tree
            kind: prop
            position: {x: 1, y: 1, z: 0}
            actor:
              yaw_degrees: 15
              scale: {x: 1, y: 2, z: 3}
              tags: [tree, vegetation, tree]
              sprite: {texture: tree, facing: cross, alpha: cutout, width: 2, height: 3, origin_x: 0.5, origin_y: 1}
instances:
  - id: repeated
    prefab: grove
    transform: {translation: {x: 4, y: 0, z: 0}, yaw_degrees: 90, scale: 2}
    tags: [interactive, highlighted]
`))
	if err != nil {
		t.Fatal(err)
	}
	actor := expanded.Rooms[0].Contents[0].Actor
	if actor == nil || math.Abs(actor.Yaw-105*math.Pi/180) > 1e-12 || actor.Scale != (worldsource.Vec3{X: 2, Y: 4, Z: 6}) ||
		!slices.Equal(actor.Tags, []string{"highlighted", "interactive"}) || actor.Sprite == nil || actor.Sprite.Texture != "tree" {
		t.Fatalf("expanded actor = %+v", actor)
	}
}

func TestLoadRejectsPathAndSymlinkEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "world.yaml")
	if err := os.WriteFile(outside, []byte("version: 1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.yaml")); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"../world.yaml", "escape.yaml"} {
		if _, err := source.Load(root, path); !errors.Is(err, source.ErrPath) {
			t.Fatalf("Load(%q) error = %v", path, err)
		}
	}
}
