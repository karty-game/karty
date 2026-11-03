package levelbuild

import (
	"bytes"
	"fmt"
	"math"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

func TestGPUWholeRoomIdentityBeyondLegacyMeshDeliveryBound(t *testing.T) {
	t.Parallel()
	document, _, _ := incrementalBakeFixture(t)
	document.StaticSolids = &world.StaticSolids{Version: world.StaticSolidsVersion}

	solids := make([]int, 800)
	for i := range solids {
		solids[i] = i

		solid := world.Solid{ID: fmt.Sprintf("column-%d", i), Bottom: world.Plane{}, Top: world.Plane{C: 1}, SideMaterial: 1,
			TopMaterial: 1, BottomMaterial: 1}

		for corner := range 8 {
			angle := float64(corner) * math.Pi / 4
			solid.Footprint = append(solid.Footprint, world.Vec2{X: 1 + .25*math.Cos(angle), Y: 1 + .25*math.Sin(angle)})
		}

		document.StaticSolids.Items = append(document.StaticSolids.Items, solid)
	}

	mesh, err := worldchunk.CompileMesh(document, []int{0}, solids, worldlightmap.Layout{})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := worldchunk.EncodeMesh(mesh, &document); err == nil {
		t.Fatal("fixture no longer exceeds legacy mesh delivery bound")
	}

	identity, err := gpuSourceMeshIdentity(&mesh, &document)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := worldchunk.CompileReady(&mesh, identity, &document, nil, worldchunk.ReadyOptions{}); err != nil {
		t.Fatal("valid large GPU room rejected", err)
	}

	mesh.Triangles[0], mesh.Triangles[1] = mesh.Triangles[1], mesh.Triangles[0]
	for corner := range 3 {
		mesh.Vertices[corner], mesh.Vertices[corner+3] = mesh.Vertices[corner+3], mesh.Vertices[corner]
	}

	changed, err := gpuSourceMeshIdentity(&mesh, &document)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Equal(identity, changed) {
		t.Fatal("source triangle order change preserved GPU identity")
	}

	mesh.Indices[0] = uint32(len(mesh.Vertices))
	if _, err := gpuSourceMeshIdentity(&mesh, &document); err == nil {
		t.Fatal("invalid mesh accepted")
	}
}
