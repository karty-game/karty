package dungeon

import "math"

const (
	VisionHeight = 4.5
	FogStep      = 4
	// Cover the complete projected crypt at the closest zoom, including the
	// logical side regions exposed by ultrawide responsive cameras.
	FogLeft    = -3072
	FogRight   = 4352
	MaxFog     = 720
	visionRays = len(Outline) * 3
)

type sightRay struct {
	angle float64
	point Point
	edge  int
}
type screenPolygon struct {
	points     [64]Point
	count      int
	minY, maxY float64
}
type interval struct{ left, right float64 }
type silhouette = screenPolygon
type sightSprite struct {
	position                           Point
	z, width, height, originX, originY float64
	hull                               int
}

// Vision retains all ray, scan conversion and mask storage. The wall outline,
// rather than the elevated camera or the walkability grid, determines sight.
// Low furniture blocks movement but does not hide a whole room.
type visionInputs struct {
	from, focus, offset Point
	height              float64
	frame               int
	outline             [len(Outline)]Point
}

type Vision struct {
	cached    bool
	inputs    visionInputs
	rays      [visionRays]sightRay
	polygons  [visionRays + len(Outline)*3 + 129]screenPolygon
	count     int
	spans     [visionRays + len(Outline)*3 + 129]interval
	HeroFrame int
	// HeroOffset follows the submitted pose without advancing accepted sight.
	HeroOffset Point
	Fog        [MaxFog]Box // logical screen rectangles, including wide-camera overscan, above the world and below UI
	Count      int
}

func cross(a, b Point) float64 { return a.X*b.Y - a.Y*b.X }
func sub(a, b Point) Point     { return Point{a.X - b.X, a.Y - b.Y} }

func cast(from, direction Point) (Point, int, float64) {
	distance, edge := math.MaxFloat64, -1
	for i, a := range Outline {
		b := Outline[(i+1)%len(Outline)]
		wall := sub(b, a)
		denominator := cross(direction, wall)
		if math.Abs(denominator) < 1e-12 {
			continue
		}
		delta := sub(a, from)
		t, u := cross(delta, wall)/denominator, cross(delta, direction)/denominator
		if t >= 0 && u >= -1e-9 && u <= 1+1e-9 && t < distance {
			distance, edge = t, i
		}
	}
	return Point{from.X + direction.X*distance, from.Y + direction.Y*distance}, edge, distance
}

// Visible includes the first wall, but never floor beyond that wall. Sight is
// current, not permanent discovery: leaving a corner hides its far side again.
func Visible(from, target Point) bool {
	if !inside(from.X, from.Y) {
		return false
	}
	d := sub(target, from)
	if d.X == 0 && d.Y == 0 {
		return true
	}
	_, _, t := cast(from, d)
	return t >= 1-1e-8
}

func GroundToScreen(p Point, z, height float64, focus Point) Point {
	dx, dy := p.X-focus.X, p.Y-focus.Y
	s := 720 / height
	return Point{640 + (dx-dy)/math.Sqrt2*s, 360 - ((dx+dy)*.5/math.Sqrt2+z*math.Sqrt(3)/2)*s}
}

func (v *Vision) polygon(points ...Point) {
	p := &v.polygons[v.count]
	p.count = len(points)
	copy(p.points[:], points)
	v.count++
}

// Update casts immediately to either side of every corner, then projects the
// visible floor and the visible, inward-facing wall portions. Wall height is
// included so fog does not erase the very wall that stops the player's sight.
// This sample's camera is fixed; no renderer or wire extension is required.
// It reports whether a new mask was computed so callers can skip publication.
func (v *Vision) Update(from Point, height float64, focus Point) bool {
	// Authored sightSprites/artHulls are immutable generated geometry. Outline
	// is the mutable wall input; compare it along with every submitted camera
	// and hero-pose input so idle frames can reuse the exact projected mask.
	inputs := visionInputs{from: from, focus: focus, offset: v.HeroOffset, height: height, frame: v.HeroFrame, outline: Outline}
	if v.cached && v.inputs == inputs {
		return false
	}
	v.inputs, v.cached = inputs, true
	v.Count, v.count = 0, 0
	if !inside(from.X, from.Y) {
		v.Fog[0], v.Count = Box{FogLeft, 0, FogRight - FogLeft, 720}, 1
		return true
	}
	n := 0
	for _, corner := range Outline {
		angle := math.Atan2(corner.Y-from.Y, corner.X-from.X)
		for _, epsilon := range [...]float64{-1e-7, 0, 1e-7} {
			a := angle + epsilon
			p, edge, _ := cast(from, Point{math.Cos(a), math.Sin(a)})
			v.rays[n] = sightRay{a, p, edge}
			n++
		}
	}
	// Only 36 rays; insertion sort avoids a reflective sort or callback heap.
	for i := 1; i < n; i++ {
		r, j := v.rays[i], i
		for j > 0 && v.rays[j-1].angle > r.angle {
			v.rays[j] = v.rays[j-1]
			j--
		}
		v.rays[j] = r
	}
	center := GroundToScreen(from, 0, height, focus)
	for i, r := range v.rays {
		s := v.rays[(i+1)%n]
		v.polygon(center, GroundToScreen(r.point, 0, height, focus), GroundToScreen(s.point, 0, height, focus))
	}
	for edge, a := range Outline {
		b := Outline[(edge+1)%len(Outline)]
		lo, hi := math.Inf(1), math.Inf(-1)
		var first, last Point
		for _, r := range v.rays {
			if r.edge != edge {
				continue
			}
			t := (r.point.X-a.X)*(b.X-a.X) + (r.point.Y-a.Y)*(b.Y-a.Y)
			if t < lo {
				lo, first = t, r.point
			}
			if t > hi {
				hi, last = t, r.point
			}
		}
		if hi <= lo {
			continue
		}
		wallHeight := WallHeight(edge)
		outerFirst, outerLast := outerWallPoint(edge, first), outerWallPoint(edge, last)
		frontFirst, frontLast := first, last
		if wallHeight == CutawayHeight {
			frontFirst, frontLast = outerFirst, outerLast
		}
		v.polygon(
			GroundToScreen(frontFirst, 0, height, focus),
			GroundToScreen(frontLast, 0, height, focus),
			GroundToScreen(frontLast, wallHeight, height, focus),
			GroundToScreen(frontFirst, wallHeight, height, focus),
		)
		v.polygon(
			GroundToScreen(first, wallHeight, height, focus),
			GroundToScreen(last, wallHeight, height, focus),
			GroundToScreen(outerLast, wallHeight, height, focus),
			GroundToScreen(outerFirst, wallHeight, height, focus),
		)
	}
	for _, sprite := range sightSprites {
		if Visible(from, sprite.position) {
			v.revealSprite(sprite, height, focus)
		}
	}
	hero := Point{from.X + v.HeroOffset.X, from.Y + v.HeroOffset.Y}
	v.revealSprite(
		sightSprite{
			position: hero,
			z:        .03,
			width:    HeroProjectedSize,
			height:   HeroProjectedSize,
			originX:  HeroOriginX,
			originY:  HeroOriginY,
			hull:     pilgrimHulls[v.HeroFrame%len(pilgrimHulls)],
		},
		height,
		focus,
	)
	for i := range v.polygons[:v.count] {
		p := &v.polygons[i]
		p.minY, p.maxY = math.Inf(1), math.Inf(-1)
		for _, point := range p.points[:p.count] {
			p.minY = math.Min(p.minY, point.Y)
			p.maxY = math.Max(p.maxY, point.Y)
		}
	}
	for y := 0; y < 720; y += FogStep {
		count := 0
		sy := float64(y) + FogStep/2.0
		for index := range v.polygons[:v.count] {
			p := &v.polygons[index]
			if sy < p.minY || sy >= p.maxY {
				continue
			}
			lo, hi := math.Inf(1), math.Inf(-1)
			for i, a := range p.points[:p.count] {
				b := p.points[(i+1)%p.count]
				if (a.Y <= sy && b.Y > sy) || (b.Y <= sy && a.Y > sy) {
					x := a.X + (b.X-a.X)*(sy-a.Y)/(b.Y-a.Y)
					lo, hi = math.Min(lo, x), math.Max(hi, x)
				}
			}
			lo, hi = math.Max(FogLeft, lo), math.Min(FogRight, hi)
			if hi <= lo {
				continue
			}
			s, j := interval{lo, hi}, count
			for j > 0 && v.spans[j-1].left > s.left {
				v.spans[j] = v.spans[j-1]
				j--
			}
			v.spans[j] = s
			count++
		}
		x := float64(FogLeft)
		for _, s := range v.spans[:count] {
			if s.left > x {
				v.fog(x, float64(y), s.left-x)
			}
			x = math.Max(x, s.right)
		}
		if x < FogRight {
			v.fog(x, float64(y), FogRight-x)
		}
	}
	return true
}

func (v *Vision) revealSprite(sprite sightSprite, height float64, focus Point) {
	p := &v.polygons[v.count]
	hull := &artHulls[sprite.hull]
	p.count = hull.count
	base := GroundToScreen(sprite.position, sprite.z, height, focus)
	scale := 720 / height
	for i, point := range hull.points[:hull.count] {
		p.points[i] = Point{base.X + (point.X-sprite.originX)*sprite.width*scale, base.Y + (point.Y-sprite.originY)*sprite.height*scale}
	}
	v.count++
}

func (v *Vision) fog(x, y, width float64) {
	if width < .01 {
		return
	}
	if v.Count == len(v.Fog) {
		// Conservative fallback retains protocol/storage bounds.
		v.Fog[0], v.Count = Box{FogLeft, 0, FogRight - FogLeft, 720}, 1
		return
	}
	v.Fog[v.Count] = Box{x, y, width, FogStep + .5}
	v.Count++
}
