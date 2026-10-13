package fps

// These dimensions match the unchanged attached physics box in src/main.go.
const playerRadius = float32(.3)
const playerHeight = float32(1.7)
const maxStep = float32(.25)
const groundTolerance = float32(.035)

// Feather's speculative contacts need this gap throughout the riser transfer;
// a one-frame lift can settle back against the vertical face before crossing.
// Static speculative contacts include velocity reach: .02 + 6.6/60 = .13
// metres at running speed. The actual compiled-world walk/run audit requires
// margin beyond that reach while the box crosses a riser.
const stepClearance = float32(.15)

// Proposed motion already accounts for velocity reach. Open portal lookahead
// also includes the .02 speculative contact margin and .005 contact slop, so
// it sees the next floor before the solver blocks its vertical face. Opaque
// walls and collision solids retain their clearance guards.
const supportContactTolerance = float32(.025)

// Walker retains only a supported upward transition, never general airborne
// height. Reset it with input reset, view replacement and cursor release.
type Walker struct {
	active        bool
	floor, dx, dy float32
	lastX, lastY  float32
	age, stalled  uint16
	blocked       bool
	blockedFloor  float32
}

func (walker *Walker) Reset() { *walker = Walker{} }
func (walker *Walker) clearTransfer() {
	blocked, floor := walker.blocked, walker.blockedFloor
	*walker = Walker{blocked: blocked, blockedFloor: floor}
}

type walkPoint struct{ x, y float32 }
type walkWall struct {
	a, b   walkPoint
	portal int
}
type walkCell struct {
	name                               string
	floor, ceiling                     float32
	floorA, floorB, ceilingA, ceilingB float32
	walls                              []walkWall
}
type walkSolid struct {
	bottom, top float32
	points      []walkPoint
}
type walkFootprint struct {
	seen                                   [walkCellCount]bool
	floor, minFloor, supportFloor, ceiling float32
	supported, blocked, sloped             bool
}

// StepUp assists only grounded, reachable, small upward changes. Ordinary
// walking, looking, stopping and descent leave Z and gravity to the host.
// The full box footprint reaches a riser before the camera center does.
func StepUp(px, py, pz, x, y float32) float32 {
	var walker Walker
	return walker.StepUp(px, py, pz, x, y)
}

func (walker *Walker) StepUp(px, py, pz, x, y float32) float32 {
	if x == px && y == py {
		walker.Reset()
		return pz
	}
	feet := pz - playerHeight
	root := -1
	for index, cell := range walkCells {
		if cell.floorAt(px, py) <= feet+groundTolerance && cell.ceilingAt(px, py) >= pz && cellContains(cell, px, py) {
			root = index
			break
		}
	}
	if root < 0 {
		walker.clearTransfer()
		return pz
	}
	current := visitFootprint(root, px, py, feet)
	if walker.blocked && current.minFloor >= walker.blockedFloor-.001 && footprintHasFloor(current, walker.blockedFloor) {
		walker.blocked = false
	}
	dx, dy := x-px, y-py
	dot := dx*walker.dx + dy*walker.dy
	// Active support must still intersect the actual target floor, with feet
	// inside the small transfer envelope and motion advancing across the riser.
	forward := dot > 0 && dot*dot >= .0625*(dx*dx+dy*dy)*(walker.dx*walker.dx+walker.dy*walker.dy)
	transfer := walker.active && forward &&
		feet >= walker.floor-groundTolerance && feet <= walker.floor+stepClearance+groundTolerance &&
		footprintHasFloor(current, walker.floor)
	if walker.active && forward && !footprintHasFloor(current, walker.floor) {
		// A blocked first lift may never reach its intended support footprint.
		// Suppress that failed target too, instead of retrying after each fall.
		*walker = Walker{blocked: true, blockedFloor: walker.floor}
	}
	if transfer {
		walker.age++
		progress := (px-walker.lastX)*walker.dx + (py-walker.lastY)*walker.dy
		if progress <= .00001*(absFloor(walker.dx)+absFloor(walker.dy)) {
			walker.stalled++
		} else {
			walker.stalled = 0
		}
		walker.lastX, walker.lastY = px, py
		// Dynamic obstacles are absent from the generated static graph. Stop
		// assisting a blocked transfer instead of holding it above a crate forever.
		transfer = walker.stalled < 12 && walker.age < 120
		if !transfer {
			*walker = Walker{blocked: true, blockedFloor: walker.floor}
		}
	}
	if !transfer {
		walker.clearTransfer()
	}
	next := visitFootprint(root, x, y, feet)
	// Continuous slopes are resolved by the host box solver. Step assistance
	// must never turn a ramp into repeated positional lifts.
	if current.sloped || next.sloped {
		walker.clearTransfer()
		return pz
	}
	base := current.supportFloor
	if transfer {
		base = walker.floor
	}
	rise := next.floor - base
	allowedTarget := !walker.blocked || absFloor(walker.blockedFloor-next.floor) > .001
	newStep := allowedTarget && (current.supported || transfer) && rise > groundTolerance && rise <= maxStep
	// Gravity can leave the box grounded on an upper corner after input stops,
	// while its center remains in the lower cell. Resume only toward that higher
	// floor through a real portal with a bounded actual rise. This never assists
	// motion back down the edge, or retries a stalled target without input reset.
	rearm := current.supported && absFloor(next.floor-current.supportFloor) < .001 &&
		walkCells[root].floor < current.supportFloor-groundTolerance &&
		allowedTarget &&
		movesTowardHigherFloor(current, dx, dy, current.supportFloor)
	newStep = newStep || rearm
	hold := transfer && current.minFloor < walker.floor-.001 && footprintHasFloor(next, walker.floor)
	// Once the full footprint reaches the target, gravity closes the contact
	// gap. Retain its bounded support witness during that settling interval so
	// a following riser remains eligible; this branch never teleports Z.
	if !newStep && !hold && transfer && !next.blocked && rise <= maxStep &&
		current.minFloor >= walker.floor-.001 && feet > walker.floor+groundTolerance {
		return pz
	}
	if (!newStep && !hold) || next.blocked || rise > maxStep {
		walker.clearTransfer()
		return pz
	}
	target := walker.floor
	if newStep {
		target = next.floor
	}
	head := max(pz, target+playerHeight+stepClearance)
	if head > min(current.ceiling, next.ceiling) || !clearSolids(px, py, feet, head) || !clearSolids(x, y, feet, head) {
		walker.clearTransfer()
		return pz
	}
	if newStep {
		walker.blocked = false
		walker.active, walker.floor, walker.dx, walker.dy = true, target, dx, dy
		walker.lastX, walker.lastY, walker.age, walker.stalled = px, py, 0, 0
	}
	return head
}

func movesTowardHigherFloor(footprint walkFootprint, dx, dy, target float32) bool {
	for index, seen := range footprint.seen {
		if !seen {
			continue
		}
		cell := walkCells[index]
		rise := target - cell.floor
		if rise <= groundTolerance || rise > maxStep {
			continue
		}
		for _, wall := range cell.walls {
			if wall.portal >= 0 && footprint.seen[wall.portal] && absFloor(walkCells[wall.portal].floor-target) < .001 &&
				(wall.b.y-wall.a.y)*dx-(wall.b.x-wall.a.x)*dy > 0 {
				return true
			}
		}
	}
	return false
}

func footprintHasFloor(footprint walkFootprint, height float32) bool {
	for index, seen := range footprint.seen {
		if seen && absFloor(walkCells[index].floor-height) < .001 {
			return true
		}
	}
	return false
}
func absFloor(value float32) float32 { return max(value, -value) }

func visitFootprint(root int, x, y, feet float32) walkFootprint {
	result := walkFootprint{floor: -1e6, minFloor: 1e6, supportFloor: -1e6, ceiling: 1e6}
	var queue [walkCellCount]int
	queue[0] = root
	result.seen[root] = true
	count := 1
	for at := 0; at < count; at++ {
		cell := walkCells[queue[at]]
		floor := cell.floorAt(x, y)
		floorRange := playerRadius * (absFloor(cell.floorA) + absFloor(cell.floorB))
		ceiling := cell.ceilingAt(x, y) - playerRadius*(absFloor(cell.ceilingA)+absFloor(cell.ceilingB))
		result.sloped = result.sloped || cell.floorA != 0 || cell.floorB != 0
		result.floor = max(result.floor, floor+floorRange)
		result.minFloor = min(result.minFloor, floor-floorRange)
		result.ceiling = min(result.ceiling, ceiling)
		if cell.floor <= feet+groundTolerance && cell.floor >= feet-groundTolerance {
			result.supported = true
			result.supportFloor = max(result.supportFloor, cell.floor)
		}
		for _, wall := range cell.walls {
			radius := playerRadius - .001
			if wall.portal >= 0 {
				radius = playerRadius + supportContactTolerance
			}
			if !edgeTouchesBox(wall.a, wall.b, x, y, radius) {
				continue
			}
			if wall.portal < 0 {
				result.blocked = true
				continue
			}
			if !result.seen[wall.portal] {
				result.seen[wall.portal] = true
				queue[count] = wall.portal
				count++
			}
		}
	}
	return result
}

func (cell walkCell) floorAt(x, y float32) float32 { return cell.floorA*x + cell.floorB*y + cell.floor }
func (cell walkCell) ceilingAt(x, y float32) float32 {
	return cell.ceilingA*x + cell.ceilingB*y + cell.ceiling
}

func cellContains(cell walkCell, x, y float32) bool {
	for _, wall := range cell.walls {
		if cross(wall.a, wall.b, walkPoint{x, y}) < -.00001 {
			return false
		}
	}
	return true
}
func cross(a, b, p walkPoint) float32 { return (b.x-a.x)*(p.y-a.y) - (b.y-a.y)*(p.x-a.x) }

// Slab clipping covers short/oblique portal edges and narrow treads even when
// no box corner lies inside the destination cell.
func edgeTouchesBox(a, b walkPoint, x, y, r float32) bool {
	low, high := float32(0), float32(1)
	for axis := 0; axis < 2; axis++ {
		start, delta, center := a.x, b.x-a.x, x
		if axis == 1 {
			start, delta, center = a.y, b.y-a.y, y
		}
		if delta == 0 {
			if start <= center-r || start >= center+r {
				return false
			}
			continue
		}
		t0, t1 := (center-r-start)/delta, (center+r-start)/delta
		if t0 > t1 {
			t0, t1 = t1, t0
		}
		low, high = max(low, t0), min(high, t1)
		if low >= high {
			return false
		}
	}
	return true
}

func clearSolids(x, y, feet, head float32) bool {
	for _, solid := range walkSolids {
		if solid.top <= feet+.001 || solid.bottom >= head {
			continue
		}
		if polygonTouchesBox(solid.points, x, y, playerRadius-.001) {
			return false
		}
	}
	return true
}
func polygonTouchesBox(points []walkPoint, x, y, r float32) bool {
	minX, maxX, minY, maxY := points[0].x, points[0].x, points[0].y, points[0].y
	for index, a := range points {
		minX, maxX, minY, maxY = min(minX, a.x), max(maxX, a.x), min(minY, a.y), max(maxY, a.y)
		b := points[(index+1)%len(points)]
		if max(
			cross(a, b, walkPoint{x - r, y - r}),
			cross(a, b, walkPoint{x - r, y + r}),
			cross(a, b, walkPoint{x + r, y - r}),
			cross(a, b, walkPoint{x + r, y + r}),
		) < 0 {
			return false
		}
	}
	return maxX > x-r && minX < x+r && maxY > y-r && minY < y+r
}
