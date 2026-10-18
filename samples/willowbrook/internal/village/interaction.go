package village

import "math"

// CanInteract uses the same authored solids and player clearance as walking.
// Being close to a sign on the other side of a railing is not a usable approach.
func CanInteract(from, target Point) bool {
	distance := math.Hypot(target.X-from.X, target.Y-from.Y)
	if distance > InteractionRadius() || !Walkable(from) || !Walkable(target) {
		return false
	}
	steps := max(1, int(math.Ceil(distance/(min(Radius, RadiusY)/3))))
	for i := 1; i < steps; i++ {
		fraction := float64(i) / float64(steps)
		if !Walkable(Point{from.X + (target.X-from.X)*fraction, from.Y + (target.Y-from.Y)*fraction}) {
			return false
		}
	}
	return true
}

// QuestVisualSize is shared by drawing and pointer hit testing. The notice
// board's full-height post stays grounded, while its painted face stays readable.
func QuestVisualSize(index int) (width, height float64) {
	if index == 2 {
		return 48, 48
	}
	return 64, 64
}

// QuestHit tests the actual full sprite, including its raised sign face.
// Padding is in logical screen pixels to retain a usable touch target at zoom.
func QuestHit(index int, x, y, footX, footY, zoom float64) bool {
	width, height := QuestVisualSize(index)
	return math.Abs(x-footX) <= width*zoom/2+12 && y >= footY-height*zoom-12 && y <= footY+12
}
