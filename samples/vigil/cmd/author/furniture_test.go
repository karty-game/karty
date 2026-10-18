//nolint:errcheck,forcetypeassert,paralleltest // Author-owned record shapes are invariant; these fixtures mutate shared author state.
package main

import (
	"fmt"
	"math"
	"testing"

	"example.com/vigil/internal/dungeon"
)

func TestFurnitureMatchesNavigationAndHasRealDepth(t *testing.T) {
	previousSolids, previousContents := solids, contents

	t.Cleanup(func() { solids, contents = previousSolids, previousContents })

	for index, bounds := range dungeon.Obstacles[:14] {
		t.Run(fmt.Sprintf("obstacle-%02d", index), func(t *testing.T) {
			solids, contents = nil, nil

			var height float64

			switch {
			case index < 4:
				pew("fixture", bounds)

				height = 1.43
			case index < 8:
				column("fixture", bounds)

				height = 3.22
			default:
				sarcophagus("fixture", bounds)

				height = .99
			}

			if len(contents) != 0 || len(solids) < 4 {
				t.Fatal("furniture must use depth-writing solids instead of a billboard")
			}

			blocking, highest := 0, 0.0

			for _, solid := range solids {
				collides, height := assertFurnitureSolidGeometry(t, solid, bounds)
				highest = math.Max(highest, height)

				if collides {
					blocking++
				}
			}

			if blocking != 1 || math.Abs(highest-height) > 1e-9 {
				t.Fatal("missing collision or silhouette height", blocking, highest)
			}
		})
	}
}

func assertFurnitureSolidGeometry(t *testing.T, solid record, bounds dungeon.Box) (bool, float64) {
	t.Helper()

	points := solid["footprint"].([]record)
	assertFurnitureFootprint(t, solid, points, bounds)
	bottom := solid["bottom"].(record)["c"].(float64)
	highest := 0.0

	for _, point := range points {
		top := furniturePlaneHeight(solid["top"].(record), point)
		if bottom < 0 || top <= bottom {
			t.Fatal("furniture has invalid vertical extent", solid["id"], bottom, top)
		}

		highest = math.Max(highest, top)
	}

	collides := solid["collision"] == true
	if collides {
		assertFurnitureCollisionFootprint(t, points, bounds, bottom)
	}

	return collides, highest
}

func assertFurnitureFootprint(t *testing.T, solid record, points []record, bounds dungeon.Box) {
	t.Helper()

	for index, point := range points {
		worldX, worldY := point["x"].(float64), point["y"].(float64)
		if worldX < bounds.X-1e-9 || worldX > bounds.X+bounds.W+1e-9 || worldY < bounds.Y-1e-9 || worldY > bounds.Y+bounds.D+1e-9 {
			t.Fatal("visible furniture exceeds its walking footprint", solid["id"], point)
		}

		b, c := points[(index+1)%len(points)], points[(index+2)%len(points)]

		bx, by, cx, cy := b["x"].(float64), b["y"].(float64), c["x"].(float64), c["y"].(float64)
		if (bx-worldX)*(cy-by)-(by-worldY)*(cx-bx) <= 0 {
			t.Fatal("furniture solid must be strictly convex CCW", solid["id"])
		}
	}
}

func assertFurnitureCollisionFootprint(t *testing.T, points []record, bounds dungeon.Box, bottom float64) {
	t.Helper()

	want := [...]dungeon.Point{
		{X: bounds.X, Y: bounds.Y},
		{X: bounds.X + bounds.W, Y: bounds.Y},
		{X: bounds.X + bounds.W, Y: bounds.Y + bounds.D},
		{X: bounds.X, Y: bounds.Y + bounds.D},
	}
	if len(points) != len(want) || bottom != 0 {
		t.Fatal("blocking base must match the whole ground footprint")
	}

	for index, point := range points {
		if math.Abs(point["x"].(float64)-want[index].X) > 1e-9 || math.Abs(point["y"].(float64)-want[index].Y) > 1e-9 {
			t.Fatal("host collision differs from navigation", point, want[index])
		}
	}
}

func furniturePlaneHeight(plane, point record) float64 {
	height := plane["c"].(float64)
	if a, ok := plane["a"].(float64); ok {
		height += a * point["x"].(float64)
	}

	if b, ok := plane["b"].(float64); ok {
		height += b * point["y"].(float64)
	}

	return height
}

func TestCarvedPewEndProfileHasContinuousCaps(t *testing.T) {
	previousSolids := solids

	t.Cleanup(func() { solids = previousSolids })

	solids = nil
	bounds := dungeon.Box{X: 7.1, Y: 5.5, W: .18, D: .75}
	pewEnd("fixture", bounds)

	for index := range 5 {
		left, right := solids[index], solids[index+1]

		seam := left["footprint"].([]record)[2]
		if left["top"].(record)["b"] == 0 {
			t.Fatal("carved end profile regressed to level boxes")
		}

		if gap := furniturePlaneHeight(
			left["top"].(record),
			seam,
		) - furniturePlaneHeight(
			right["top"].(record),
			seam,
		); math.Abs(
			gap,
		) > 1e-9 {
			t.Fatal("carved profile has a gap or overlapping height at its seam", index, gap)
		}
	}
}

// All these parts become static, material-batched room geometry. Bound the
// actual source/triangle growth so ornament cannot turn into an unlimited
// collection of per-face decorative solids.
func TestFurnitureDetailGeometryBudget(t *testing.T) {
	previousSolids := solids

	t.Cleanup(func() { solids = previousSolids })

	solids = nil

	for index, bounds := range dungeon.Obstacles[:14] {
		switch {
		case index < 4:
			pew(fmt.Sprintf("pew-%d", index), bounds)
		case index < 8:
			column(fmt.Sprintf("column-%d", index), bounds)
		default:
			sarcophagus(fmt.Sprintf("tomb-%d", index), bounds)
		}
	}

	triangles := 0

	for _, solid := range solids {
		vertices := len(solid["footprint"].([]record))
		triangles += 2*vertices + 2*(vertices-2)
	}

	if len(solids) > 400 || triangles > 8000 {
		t.Fatal("static furniture detail exceeded its geometry budget", len(solids), triangles)
	}

	t.Logf("furniture: %d static solids, at most %d triangles including bottom caps", len(solids), triangles)
}

func TestFurnitureSightHullIncludesSlopingCaps(t *testing.T) {
	previousSolids := solids

	t.Cleanup(func() { solids = previousSolids })

	solids = nil
	bounds := dungeon.Box{X: 2, Y: 3, W: 1, D: 1}

	box("fixture-profile", 2, 3, 1, 1, 0, 1, "wood", false)

	solids[0]["top"] = record{"a": .2, "b": .4, "c": -1.0}
	solids[0]["bottom"] = record{"a": .02, "b": .03, "c": .1}
	hull := obstacleSightHull("fixture", bounds, solids)

	minY, maxY := math.Inf(1), math.Inf(-1)
	for _, p := range hull {
		minY, maxY = math.Min(minY, p[1]), math.Max(maxY, p[1])
	}

	wantMin, wantMax := math.Inf(1), math.Inf(-1)

	for _, p := range solids[0]["footprint"].([]record) {
		x, y := p["x"].(float64), p["y"].(float64)
		for _, face := range []string{"bottom", "top"} {
			z := furniturePlaneHeight(solids[0][face].(record), p)
			projected := -((x-bounds.X)+(y-bounds.Y))*.5/math.Sqrt2 - z*math.Sqrt(3)/2
			wantMin, wantMax = math.Min(wantMin, projected), math.Max(wantMax, projected)
		}
	}

	if len(hull) < 3 || math.Abs(minY-wantMin) > 1e-9 || math.Abs(maxY-wantMax) > 1e-9 {
		t.Fatal("sight hull clipped sloped solid", hull, minY, maxY, wantMin, wantMax)
	}
}
