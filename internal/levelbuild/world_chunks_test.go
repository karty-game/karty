package levelbuild

import (
	"os"
	"path/filepath"
	"testing"
)

const chunkRoomsYAML = `rooms:
  - id: first
    boundary:
      - {id: south, start: {x: 0, y: 0}, end: {x: 2, y: 0}, material: wall}
      - {id: east, start: {x: 2, y: 0}, end: {x: 2, y: 2}, material: wall}
      - {id: north, start: {x: 2, y: 2}, end: {x: 0, y: 2}, material: wall}
      - {id: west, start: {x: 0, y: 2}, end: {x: 0, y: 0}, material: wall}
    floor: {c: 0}
    ceiling: {c: 4}
    floor_material: wall
    ceiling_material: wall
    contents:
      - {id: spawn, kind: spawn, position: {x: 1, y: 1, z: 1}}
  - id: second
    boundary:
      - {id: south, start: {x: 2, y: 0}, end: {x: 4, y: 0}, material: wall}
      - {id: east, start: {x: 4, y: 0}, end: {x: 4, y: 2}, material: wall}
      - {id: north, start: {x: 4, y: 2}, end: {x: 2, y: 2}, material: wall}
      - {id: west, start: {x: 2, y: 2}, end: {x: 2, y: 0}, material: wall}
    floor: {c: 0}
    ceiling: {c: 4}
    floor_material: wall
    ceiling_material: wall
connections:
  - {id: door, a: {room: first, edge: east}, b: {room: second, edge: west}}
`

func enableFixtureChunks(t *testing.T, root, settings string) {
	t.Helper()

	path := filepath.Join(root, "levels", "lighting", "level.toml")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// #nosec G703 -- path is constructed inside the isolated t.TempDir level fixture.
	if err := os.WriteFile(path, append(data, []byte("\n[chunks]\nenabled = true\n"+settings)...), 0o600); err != nil {
		t.Fatal(err)
	}
}
