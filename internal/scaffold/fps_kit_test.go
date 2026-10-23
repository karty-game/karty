package scaffold_test

import (
	"os"
	"path/filepath"
	"testing"

	localpackages "github.com/karty-game/karty/internal/packages"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/scaffold"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/testpackages"
	"github.com/karty-game/karty/internal/worldbuild/geometry"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

func TestFPSStarterHasConnectedRoomsAndSharedCrates(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	directory := filepath.Join(t.TempDir(), "fps-kit-game")
	if err := scaffold.CreateTemplate(directory, "fps-kit-game", manifest, "fps"); err != nil {
		t.Fatal(err)
	}

	stageStarterArchiveFixtures(t, directory)

	set, err := localpackages.Restore(t.Context(), directory)
	if err != nil {
		t.Fatal(err)
	}
	defer set.Close()

	if set.Items["fps-kit"] == nil || set.Items["fps-kit"].Manifest.GoModule != "" {
		t.Fatal("starter must restore its pinned resource-only FPS Kit")
	}

	data, err := os.ReadFile(filepath.Join(directory, "levels/room/world.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	merged, textures, err := localpackages.ResolveWorld(t.Context(), set, data, nil)
	if err != nil {
		t.Fatal(err)
	}

	expanded, err := source.Decode(merged)
	if err != nil {
		t.Fatal(err)
	}

	if len(expanded.Rooms) != 3 || len(expanded.Connections) != 2 || len(expanded.Solids) != 21 {
		t.Fatal("starter lost its connected rooms or three static crates")
	}

	assertFPSStarterCorridor(t, expanded)

	materials := make(map[string]uint32, len(textures))
	for index, texture := range textures {
		materials[texture.Name] = uint32(index + 1)
	}

	compiled, err := geometry.Compile(expanded, materials)
	if err != nil {
		t.Fatal(err)
	}

	if compiled.SolidRenderers == nil || len(compiled.SolidRenderers.Prefabs) != 1 ||
		compiled.SolidRenderers.Prefabs[0].ID != "crate-medium-dynamic" {
		t.Fatal("starter must package its movable crate renderer")
	}

	if len(compiled.Contents) != 4 {
		t.Fatal("starter must include room numbers, a floor arrow and a goal decal")
	}

	for _, content := range compiled.Contents {
		if content.Actor == nil || content.Actor.Sprite == nil {
			t.Fatal("placeholder marker lost its sprite")
		}
	}

	for _, solid := range compiled.StaticSolids.Items {
		if !solid.Collision {
			t.Fatal("static crate lost collision")
		}
	}
}

func assertFPSStarterCorridor(t *testing.T, expanded source.Expanded) {
	t.Helper()

	rooms := make(map[string]source.Room, len(expanded.Rooms))
	for _, room := range expanded.Rooms {
		rooms[room.ID] = room
	}

	for roomID, want := range map[string]struct {
		bounds [4]float64
		height float64
	}{
		"entry":    {bounds: [4]float64{0, 0, 8, 8}, height: 3},
		"corridor": {bounds: [4]float64{3, 8, 5, 12}, height: 2.6},
		"workshop": {bounds: [4]float64{0, 12, 8, 20}, height: 3},
	} {
		room, found := rooms[roomID]
		if !found || len(room.Boundary) == 0 {
			t.Fatalf("starter is missing room %q", roomID)
		}

		first := room.Boundary[0].Start
		bounds := [4]float64{first.X, first.Y, first.X, first.Y}

		for _, edge := range room.Boundary {
			bounds[0] = min(bounds[0], edge.Start.X, edge.End.X)
			bounds[1] = min(bounds[1], edge.Start.Y, edge.End.Y)
			bounds[2] = max(bounds[2], edge.Start.X, edge.End.X)
			bounds[3] = max(bounds[3], edge.Start.Y, edge.End.Y)
		}

		if bounds != want.bounds || room.Floor.A != 0 || room.Floor.B != 0 || room.Floor.C != 0 ||
			room.Ceiling.A != 0 || room.Ceiling.B != 0 || room.Ceiling.C != want.height {
			t.Fatalf("room %q must preserve its footprint and flat floor/ceiling: %+v", roomID, room)
		}
	}

	if len(rooms["corridor"].Boundary) != 4 {
		t.Fatal("the four-meter passage must have four straight boundary edges")
	}

	wantConnections := map[string][2]source.Endpoint{
		"entry-passage":    {{Room: "entry", Edge: "doorway"}, {Room: "corridor", Edge: "entry"}},
		"workshop-passage": {{Room: "corridor", Edge: "workshop"}, {Room: "workshop", Edge: "doorway"}},
	}
	for _, connection := range expanded.Connections {
		want, found := wantConnections[connection.ID]
		if !found || connection.A != want[0] || connection.B != want[1] || connection.NonEuclidean {
			t.Fatalf("unexpected starter connection: %+v", connection)
		}

		first := fpsStarterPortalEdge(t, rooms, connection.A)
		second := fpsStarterPortalEdge(t, rooms, connection.B)

		if first != [4]float64{second[2], second[3], second[0], second[1]} ||
			first[1] != first[3] || min(first[0], first[2]) != 3 || max(first[0], first[2]) != 5 {
			t.Fatalf("passage %q must join coincident reversed two-meter edges", connection.ID)
		}

		delete(wantConnections, connection.ID)
	}

	if len(wantConnections) != 0 {
		t.Fatal("both ends of the corridor must connect to a room")
	}
}

func fpsStarterPortalEdge(t *testing.T, rooms map[string]source.Room, endpoint source.Endpoint) [4]float64 {
	t.Helper()

	for _, edge := range rooms[endpoint.Room].Boundary {
		if edge.ID == endpoint.Edge {
			return [4]float64{edge.Start.X, edge.Start.Y, edge.End.X, edge.End.Y}
		}
	}

	t.Fatalf("missing passage edge %+v", endpoint)

	return [4]float64{}
}

func stageStarterArchiveFixtures(t *testing.T, directory string) {
	t.Helper()

	packageSources, _, err := localpackages.LoadSources(t.Context(), directory)
	if err != nil {
		t.Fatal(err)
	}

	pins, err := testpackages.Pins()
	if err != nil {
		t.Fatal(err)
	}

	// Stage verified release bytes without changing the starter's authored pin.
	archiveDirectory := filepath.Join(directory, ".karty/packages/.downloads")
	if err := os.MkdirAll(archiveDirectory, 0o750); err != nil {
		t.Fatal(err)
	}

	cache, err := os.OpenRoot(archiveDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer cache.Close()

	for name, dependency := range packageSources {
		pin, exists := pins[name]
		if !exists || dependency.Archive != pin.Archive || dependency.Folder != "" || dependency.SHA256 != pin.SHA256 {
			t.Fatalf("starter must pin the versioned %s release archive and checksum: %+v", name, dependency)
		}

		if _, err := os.Stat(filepath.Join(directory, "packages", name)); !os.IsNotExist(err) {
			t.Fatal("starter must declare its release dependency without embedding the package source")
		}

		archive, err := testpackages.Archive(name)
		if err != nil {
			t.Fatal(err)
		}

		data, err := os.ReadFile(archive)
		if err != nil {
			t.Fatal(err)
		}

		if err := cache.WriteFile(pin.SHA256+".kpkg", data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
