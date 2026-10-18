package dungeon

import "math"

func pointSegmentDistanceSquared(p, a, b Point) float64 {
	d := sub(b, a)
	l := d.X*d.X + d.Y*d.Y
	if l == 0 {
		d = sub(p, a)
		return d.X*d.X + d.Y*d.Y
	}
	t := math.Max(0, math.Min(1, ((p.X-a.X)*d.X+(p.Y-a.Y)*d.Y)/l))
	d = Point{p.X - a.X - t*d.X, p.Y - a.Y - t*d.Y}
	return d.X*d.X + d.Y*d.Y
}

func segmentDistanceSquared(a, b, c, d Point) float64 {
	r, s := sub(b, a), sub(d, c)
	denominator := cross(r, s)
	if math.Abs(denominator) > 1e-12 {
		delta := sub(c, a)
		t, u := cross(delta, s)/denominator, cross(delta, r)/denominator
		if t >= 0 && t <= 1 && u >= 0 && u <= 1 {
			return 0
		}
	}
	return math.Min(math.Min(pointSegmentDistanceSquared(a, c, d), pointSegmentDistanceSquared(b, c, d)),
		math.Min(pointSegmentDistanceSquared(c, a, b), pointSegmentDistanceSquared(d, a, b)))
}

// ClearSegment tests the complete swept player capsule against the same wall
// outline and furniture footprints as Move, including rounded corner clearance.
func ClearSegment(from, to Point) bool {
	if !Walkable(from) || !Walkable(to) {
		return false
	}
	const clearanceSquared = Radius*Radius - 1e-10
	for i, a := range Outline {
		if segmentDistanceSquared(from, to, a, Outline[(i+1)%len(Outline)]) < clearanceSquared {
			return false
		}
	}
	for _, obstacle := range Obstacles {
		a := Point{obstacle.X, obstacle.Y}
		b := Point{obstacle.X + obstacle.W, obstacle.Y}
		c := Point{obstacle.X + obstacle.W, obstacle.Y + obstacle.D}
		d := Point{obstacle.X, obstacle.Y + obstacle.D}
		if segmentDistanceSquared(from, to, a, b) < clearanceSquared ||
			segmentDistanceSquared(from, to, b, c) < clearanceSquared ||
			segmentDistanceSquared(from, to, c, d) < clearanceSquared ||
			segmentDistanceSquared(from, to, d, a) < clearanceSquared {
			return false
		}
	}
	return true
}

const MaxRoutePoints = 128

// Navigation retains only a borrowed-result copy, request identity and follower
// state. Search working storage and smoothing belong to the engine.
type Navigation struct {
	Path           [MaxRoutePoints + 1]Point
	Count, Next    int
	Pending, Level uint32
	Via            Point
}

func (n *Navigation) Cancel() { n.Count, n.Next, n.Pending, n.Level = 0, 0, 0, 0 }
func (n *Navigation) Begin(request, level uint32, via Point) {
	n.Cancel()
	n.Pending, n.Level, n.Via = request, level, via
}

// Accept ignores superseded replies and copies matching points before their SDK
// borrow expires. The actor waits while a query is pending, so Via remains a
// safe connection to the original accepted pose. Open routes skip recentering.
func (n *Navigation) Accept(request, level uint32, from Point, points []Point) (matched, usable bool) {
	if request == 0 || request != n.Pending || level != n.Level {
		return false, false
	}
	via := n.Via
	n.Cancel()
	if len(points) == 0 || len(points) > MaxRoutePoints {
		return true, false
	}
	anchor := via
	for _, p := range points {
		if !ClearSegment(anchor, p) {
			return true, false
		}
		anchor = p
	}
	if !ClearSegment(from, points[0]) {
		if !ClearSegment(from, via) {
			return true, false
		}
		n.Path[0] = via
		n.Count = 1
	}
	copy(n.Path[n.Count:], points)
	n.Count += len(points)
	return true, true
}

// Step carries its distance budget through short waypoints. The final submitted
// segment must also be clear: a turn beside masonry cannot cut its corner merely
// because the two separate legs were safe.
func (n *Navigation) Step(from Point, budget float64) Point {
	p := from
	for n.Next < n.Count && budget > 1e-9 {
		target := n.Path[n.Next]
		distance := Distance(p, target)
		if distance < 1e-9 {
			p = target
			n.Next++
			continue
		}
		travel := math.Min(distance, budget)
		next := Point{p.X + (target.X-p.X)/distance*travel, p.Y + (target.Y-p.Y)/distance*travel}
		if !ClearSegment(from, next) {
			if p == from {
				return Move(from, sub(next, from))
			}
			return p
		}
		p = next
		budget -= travel
		if travel == distance {
			p = target
			n.Next++
		}
	}
	return p
}
