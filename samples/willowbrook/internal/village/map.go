// Package village contains gameplay rules over the compiled Tiled map.
package village

import (
	"math"

	"github.com/karty-game/tiled"
)

type Point = tiled.Point

var Level *tiled.Map
var Spawn Point
var QuestStations [3]Point
var QuestMessages [4]string
var ActorLayer int
var Radius float64
var RadiusY float64
var Colliders []tiled.Region

// Configure installs one complete, compiled map before gameplay starts.
func Configure(level *tiled.Map) {
	Level = level
	Radius = Level.Properties.Number("playerRadius", 7.5)
	// Feet are wider than they are deep in the frontal artwork. Keep this
	// ground ellipse separate from the much taller, naturally occluding body.
	RadiusY = Level.Properties.Number("playerRadiusY", Radius)
	QuestMessages[0] = Level.Properties.String("intro", "A note waits beside the lower cottage.")
	for i, name := range Level.LayerNames {
		if name == "Scenery" {
			ActorLayer = i
		}
	}
	Colliders = nil
	found := [4]bool{}
	for _, o := range Level.Objects {
		switch o.Class {
		case "collision":
			Colliders = append(Colliders, o)
		case "spawn":
			Spawn = Point{o.X, o.Y}
			found[0] = true
		case "clue", "treasure":
			i := int(o.Properties.Number("order", -1))
			if i < 0 || i >= 3 {
				panic("quest object requires order 0..2")
			}
			QuestStations[i] = Point{o.X, o.Y}
			QuestMessages[i+1] = o.Properties.String("message", "")
			found[i+1] = true
		}
	}
	for _, ok := range found {
		if !ok {
			panic("Tiled gameplay layer requires a spawn, two clues and treasure")
		}
	}
	if !Walkable(Spawn) {
		panic("Tiled spawn is inside a collider")
	}
}

func Width() float64  { return float64(Level.Width * Level.TileWidth) }
func Height() float64 { return float64(Level.Height * Level.TileHeight) }
func Walkable(p Point) bool {
	return walkableWithRadii(p, Radius, RadiusY)
}

func walkableWithRadii(p Point, radiusX, radiusY float64) bool {
	if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) || p.X < radiusX || p.Y < radiusY ||
		p.X >= Width()-radiusX ||
		p.Y >= Height()-radiusY {
		return false
	}
	for _, b := range Colliders {
		if len(b.Polygon) != 0 {
			inside := false
			for i, a := range b.Polygon {
				c := b.Polygon[(i+1)%len(b.Polygon)]
				a.X += b.X
				a.Y += b.Y
				c.X += b.X
				c.Y += b.Y
				if (a.Y > p.Y) != (c.Y > p.Y) && p.X < (c.X-a.X)*(p.Y-a.Y)/(c.Y-a.Y)+a.X {
					inside = !inside
				}
				// Scaling into foot-radius space turns an ellipse/segment
				// distance into an ordinary unit-circle distance. This keeps
				// sloped rock outlines and corners solid on both axes.
				dx, dy := (c.X-a.X)/radiusX, (c.Y-a.Y)/radiusY
				px, py := (p.X-a.X)/radiusX, (p.Y-a.Y)/radiusY
				length := dx*dx + dy*dy
				u := 0.0
				if length > 0 {
					u = max(0, min(1, (px*dx+py*dy)/length))
				}
				ex, ey := px-u*dx, py-u*dy
				if ex*ex+ey*ey < 1 {
					return false
				}
			}
			if inside {
				return false
			}
		} else if p.X+radiusX > b.X && p.X-radiusX < b.X+b.Width && p.Y+radiusY > b.Y && p.Y-radiusY < b.Y+b.Height {
			return false
		}
	}
	return true
}

// Move uses the same Tiled collision objects for walking and route edges.
func Move(p, delta Point) Point {
	if delta.X == 0 && delta.Y == 0 {
		return p
	}
	n := max(1, int(math.Ceil(math.Hypot(delta.X, delta.Y)/(min(Radius, RadiusY)/3))))
	if n > 1024 {
		return p
	}
	step := Point{X: delta.X / float64(n), Y: delta.Y / float64(n)}
	for range n {
		before := p
		for _, d := range [2]Point{{X: step.X}, {Y: step.Y}} {
			q := Point{p.X + d.X, p.Y + d.Y}
			if Walkable(q) {
				p = q
			}
		}
		if p == before && step.X != 0 && step.Y != 0 {
			p = slideAlongPolygon(p, step)
		}
	}
	return p
}

// slideAlongPolygon handles rounded foot clearance at an authored corner.
// Axis rejection alone can leave the feet slightly inside the straight wall's
// eventual clearance, unable to progress along it. Project onto the nearest
// contact tangent in the same ellipse space used by Walkable, then validate
// the entire result against every solid. It never relaxes collision bounds.
func slideAlongPolygon(p, step Point) Point {
	sx, sy := step.X/Radius, step.Y/RadiusY
	margin := 1 + math.Hypot(sx, sy)
	nearest := margin * margin
	nx, ny := 0.0, 0.0
	for _, body := range Colliders {
		for i, a := range body.Polygon {
			b := body.Polygon[(i+1)%len(body.Polygon)]
			dx, dy := (b.X-a.X)/Radius, (b.Y-a.Y)/RadiusY
			px, py := (p.X-body.X-a.X)/Radius, (p.Y-body.Y-a.Y)/RadiusY
			u := 0.0
			if length := dx*dx + dy*dy; length > 0 {
				u = max(0, min(1, (px*dx+py*dy)/length))
			}
			x, y := px-u*dx, py-u*dy
			if distance := x*x + y*y; distance < nearest {
				nx, ny, nearest = x, y, distance
			}
		}
	}
	if nearest == 0 || nx == 0 && ny == 0 {
		return p
	}
	inward := (sx*nx + sy*ny) / nearest
	if inward >= 0 {
		return p
	}
	tangent := Point{X: (sx - inward*nx) * Radius, Y: (sy - inward*ny) * RadiusY}
	// Projecting in ellipse space must not increase the world-space speed.
	if length, limit := tangent.X*tangent.X+tangent.Y*tangent.Y, step.X*step.X+step.Y*step.Y; length > limit {
		scale := math.Sqrt(limit / length)
		tangent.X *= scale
		tangent.Y *= scale
	}
	q := Point{X: p.X + tangent.X, Y: p.Y + tangent.Y}
	if Walkable(q) {
		return q
	}
	return p
}
func InRegion(class string, p Point) bool {
	for _, r := range Level.Objects {
		if r.Class == class && p.X >= r.X && p.X < r.X+r.Width && p.Y >= r.Y && p.Y < r.Y+r.Height {
			return true
		}
	}
	return false
}
func CameraFocus(p Point) Point {
	return Point{p.X, p.Y + Level.Properties.Number("cameraOffsetY", -110)}
}
func Zoom() float64              { return Level.Properties.Number("cameraZoom", 1) }
func WalkSpeed() float64         { return Level.Properties.Number("walkSpeed", 108.8) }
func RunSpeed() float64          { return Level.Properties.Number("runSpeed", 176) }
func InteractionRadius() float64 { return Level.Properties.Number("interactionRadius", 40) }
