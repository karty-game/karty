package dungeon

import (
	"math"
	"testing"
)

func TestFireballFacingAndCollision(t *testing.T) {
	for facing := range WalkDirections {
		d := FacingDirection(facing)
		if Facing(d.X, d.Y) != facing || math.Abs(math.Hypot(d.X, d.Y)-1) > 1e-12 {
			t.Fatalf("direction %d does not match the projected pose: %v", facing, d)
		}
	}
	for _, route := range []struct{ from, direction Point }{
		{Point{12, 1}, Point{0, -1}}, // outer wall
		{Point{8, 5}, Point{0, 1}},   // pew
		{Point{7, 9}, Point{0, 1}},   // column
	} {
		p, stopped := route.from, false
		for range 30 {
			next, moving := FireballStep(p, route.direction)
			if !moving {
				stopped = true
				break
			}
			if !Walkable(next) {
				t.Fatalf("projectile entered an obstacle at %v", next)
			}
			p = next
		}
		if !stopped {
			t.Fatalf("projectile passed obstruction from %v", route.from)
		}
	}
	if next, moving := FireballStep(Spawn, FacingDirection(4)); !moving || math.Abs(Distance(Spawn, next)-.1) > 1e-12 {
		t.Fatalf("open-floor projectile failed to advance: %v %v", next, moving)
	}
}
