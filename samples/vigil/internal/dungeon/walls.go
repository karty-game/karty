package dungeon

import "math"

const WallThickness = .5
const CutawayHeight = .55

// Outline is the inner masonry face, preserving the authored walking space.
// Adjacent outward offsets meet at a miter; no corner has an open paper edge.
func OuterCorner(i int) Point {
	p := Outline[i]
	previous, next := Outline[(i+len(Outline)-1)%len(Outline)], Outline[(i+1)%len(Outline)]
	a, b := sub(p, previous), sub(next, p)
	la, lb := math.Hypot(a.X, a.Y), math.Hypot(b.X, b.Y)
	na, nb := Point{a.Y / la, -a.X / la}, Point{b.Y / lb, -b.X / lb}
	scale := WallThickness / (1 + na.X*nb.X + na.Y*nb.Y)
	return Point{p.X + (na.X+nb.X)*scale, p.Y + (na.Y+nb.Y)*scale}
}

// Low camera-facing sections are an architectural cutaway. They retain the
// same opaque inner sight boundary as the full-height wall they represent.
func WallHeight(edge int) float64 {
	a, b := Outline[edge], Outline[(edge+1)%len(Outline)]
	if b.X-a.X >= b.Y-a.Y {
		return CutawayHeight
	}
	return VisionHeight
}

func outerWallPoint(edge int, p Point) Point {
	a, b := Outline[edge], Outline[(edge+1)%len(Outline)]
	d := sub(b, a)
	t := ((p.X-a.X)*d.X + (p.Y-a.Y)*d.Y) / (d.X*d.X + d.Y*d.Y)
	oa, ob := OuterCorner(edge), OuterCorner((edge+1)%len(Outline))
	return Point{oa.X + (ob.X-oa.X)*t, oa.Y + (ob.Y-oa.Y)*t}
}
