package dungeon

import (
	"math"
	"testing"
)

func TestMasonryFormsJoinedConvexBandsOutsideWalkingSpace(t *testing.T) {
	var full, cut int
	for i, a := range Outline {
		b := Outline[(i+1)%len(Outline)]
		oa, ob := OuterCorner(i), OuterCorner((i+1)%len(Outline))
		quad := [...]Point{a, oa, ob, b}
		for j, p := range quad {
			if turn := cross(sub(quad[(j+1)%4], p), sub(quad[(j+2)%4], quad[(j+1)%4])); turn <= 0 {
				t.Fatal("masonry not strictly convex CCW", i, quad)
			}
		}
		d := sub(b, a)
		mid := Point{(oa.X + ob.X) / 2, (oa.Y + ob.Y) / 2}
		if width := math.Abs(cross(d, sub(mid, a))) / math.Hypot(d.X, d.Y); math.Abs(width-WallThickness) > 1e-9 {
			t.Fatal("wrong masonry thickness", i, width)
		}
		if Walkable(mid) {
			t.Fatal("wall band entered walking space", i, mid)
		}
		if WallHeight(i) == CutawayHeight {
			cut++
		} else {
			full++
		}
	}
	if full != 6 || cut != 6 {
		t.Fatal("cutaway orientation changed", full, cut)
	}
}

func TestSightRetainsFullAndCutawayWallCaps(t *testing.T) {
	var v Vision
	for _, test := range []struct {
		from, cap Point
		height    float64
	}{
		{Spawn, Point{12, -WallThickness / 2}, CutawayHeight},
		{Point{12, 22}, Point{12, 25 + WallThickness/2}, VisionHeight},
	} {
		focus := Point{test.from.X + .8, test.from.Y + .8}
		v.Update(test.from, 14, focus)
		p := GroundToScreen(test.cap, test.height, 14, focus)
		if masked(&v, p) {
			t.Fatal("masonry top cap masked", test, p)
		}
	}
}
