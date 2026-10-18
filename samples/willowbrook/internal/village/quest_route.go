package village

import "math"

// QuestRoute finds a bounded four-neighbour route toward a quest object.
// Edge tests use the same swept collision rules as ordinary walking.
// Ordinary frame updates allocate no search storage, and regular taps retain
// their direct movement behaviour.
func QuestRoute(from, to Point) []Point {
	if !Walkable(from) || !Walkable(to) {
		return nil
	}
	const capacity = 32767
	// The map can include a painted camera buffer. Search only its authored
	// navigation area, at quarter-tile resolution for narrow scenery gaps.
	areaX, areaY, areaW, areaH := 0.0, 0.0, Width(), Height()
	for _, r := range Level.Objects {
		if r.Class == "navigation" {
			areaX, areaY, areaW, areaH = r.X, r.Y, r.Width, r.Height
			break
		}
	}
	step := Level.Properties.Number("navigationStep", float64(Level.TileWidth)/4)
	if step < 1 || areaW <= 0 || areaH <= 0 {
		return nil
	}
	cols, rows := int(areaW/step)-1, int(areaH/step)-1
	if cols <= 0 || rows <= 0 || cols*rows > capacity {
		return nil
	}
	count := cols * rows
	cell := func(i int) Point {
		return Point{X: areaX + step + float64(i%cols)*step, Y: areaY + step + float64(i/cols)*step}
	}
	// A grid line exactly tangent to a post is mathematically walkable but
	// cannot be followed reliably from fractional positions: the final tiny
	// correction can be smaller than a coordinate's floating-point increment.
	// Reserve clearance for planned paths only; ordinary walking keeps its
	// full authored floor and unchanged physical collision bounds.
	open := func(p Point) bool { return walkableWithRadii(p, Radius+.25, RadiusY+.25) }
	connected := func(a, b Point) bool {
		q := Move(a, Point{X: b.X - a.X, Y: b.Y - a.Y})
		if math.Hypot(q.X-b.X, q.Y-b.Y) >= .001 {
			return false
		}
		n := max(1, int(math.Ceil(math.Hypot(b.X-a.X, b.Y-a.Y)/(min(Radius, RadiusY)/3))))
		for i := 1; i < n; i++ {
			u := float64(i) / float64(n)
			if !open(Point{X: a.X + (b.X-a.X)*u, Y: a.Y + (b.Y-a.Y)*u}) {
				return false
			}
		}
		return true
	}
	x := max(0, min(cols-1, int(math.Round((from.X-areaX-step)/step))))
	y := max(0, min(rows-1, int(math.Round((from.Y-areaY-step)/step))))
	start := -1
	best := math.Inf(1)
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			cx, cy := x+dx, y+dy
			if cx < 0 || cy < 0 || cx >= cols || cy >= rows {
				continue
			}
			i := cy*cols + cx
			p := cell(i)
			distance := math.Hypot(p.X-from.X, p.Y-from.Y)
			if distance < best && open(p) && connected(from, p) {
				start, best = i, distance
			}
		}
	}
	if start < 0 {
		return nil
	}
	var parent [capacity]int16
	for i := range parent[:count] {
		parent[i] = -2
	}
	var queue [capacity]int16
	queue[0], parent[start] = int16(start), -1
	head, tail, end := 0, 1, -1
	for head < tail {
		i := int(queue[head])
		head++
		p := cell(i)
		if math.Hypot(p.X-to.X, p.Y-to.Y) <= InteractionRadius()*.82 && CanInteract(p, to) {
			end = i
			break
		}
		for _, delta := range [][2]int{{0, 1}, {1, 0}, {0, -1}, {-1, 0}} {
			nx, ny := i%cols+delta[0], i/cols+delta[1]
			if nx < 0 || ny < 0 || nx >= cols || ny >= rows {
				continue
			}
			n := ny*cols + nx
			if parent[n] != -2 {
				continue
			}
			q := cell(n)
			if !open(q) || !connected(p, q) {
				continue
			}
			parent[n] = int16(i)
			queue[tail] = int16(n)
			tail++
		}
	}
	if end < 0 {
		return nil
	}
	var reverse []Point
	for i := end; i >= 0; i = int(parent[i]) {
		reverse = append(reverse, cell(i))
	}
	for a, b := 0, len(reverse)-1; a < b; a, b = a+1, b-1 {
		reverse[a], reverse[b] = reverse[b], reverse[a]
	}
	return reverse
}
