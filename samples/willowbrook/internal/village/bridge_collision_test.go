package village

import (
	"math"
	"strings"
	"testing"

	"github.com/karty-game/tiled"
)

func bridgeDeck(t *testing.T) (centreX, north, south float64) {
	t.Helper()
	for _, layer := range authoredLevel.Layers {
		for _, object := range layer.Objects {
			if object.Name == "intact-bridge" {
				// Independently traced from the approved 384x147 image: the
				// planks occupy native rows30..88, not merely a centreline.
				return object.X, object.Y - object.Height + object.Height*30/147, object.Y - object.Height + object.Height*88/147
			}
		}
	}
	t.Fatal("missing intact bridge image")
	return 0, 0, 0
}

func TestBridgeWholeVisibleDeckIsWalkableInBothDirections(t *testing.T) {
	x, north, south := bridgeDeck(t)
	for _, y := range []float64{math.Ceil(north) + 2, (north + south) / 2, math.Floor(south) - 2} {
		for _, direction := range []float64{-1, 1} {
			from := Point{x - direction*48, y}
			if !Walkable(from) {
				t.Errorf("visible plank lane is blocked at %+v", from)
				continue
			}
			end := Move(from, Point{X: direction * 96})
			if math.Hypot(end.X-(x+direction*48), end.Y-y) > .001 {
				t.Errorf("invisible midspan barrier in plank lane %g: %+v -> %+v", y, from, end)
			}
		}
	}
	// Keep both full bank landings connected, with all four post solids live.
	for _, direction := range []float64{-1, 1} {
		from := Point{x - direction*136, (north + south) / 2}
		end := Move(from, Point{X: direction * 272})
		if math.Hypot(end.X-(x+direction*136), end.Y-from.Y) > .001 {
			t.Errorf("bridge landing blocks the crossing: %+v -> %+v", from, end)
		}
	}
}

func TestBridgeDiagonalWalkingSlidesAlongVisibleDeckEdges(t *testing.T) {
	x, north, south := bridgeDeck(t)
	step := RunSpeed() / 60 / math.Sqrt2
	for _, direction := range []float64{-1, 1} {
		for _, vertical := range []float64{-1, 1} {
			p := Point{x - direction*48, (north + south) / 2}
			for range 45 {
				p = Move(p, Point{X: direction * step, Y: vertical * step})
				if !Walkable(p) || p.Y < north-.5 || p.Y > south+.5 {
					t.Fatalf("diagonal bridge motion escaped its visible floor: %+v", p)
				}
			}
			if (p.X-x)*direction < 40 || vertical < 0 && p.Y > north+3 || vertical > 0 && p.Y < south-3 {
				t.Errorf("diagonal motion stalled before reaching the visible deck edge: %+v", p)
			}
		}
	}
	for _, direction := range []float64{-1, 1} {
		p := Move(Point{x, (north + south) / 2}, Point{Y: direction * 120})
		if p.Y < north-.5 || p.Y > south+.5 || direction < 0 && p.Y > north+3 || direction > 0 && p.Y < south-3 {
			t.Errorf("bridge edge blocks visible planks or allows walking into water: %+v", p)
		}
	}
}

func TestBridgeSupportsRemainSolid(t *testing.T) {
	original := Colliders
	t.Cleanup(func() { Colliders = original })
	count := 0
	for _, body := range original {
		if !strings.HasPrefix(body.Name, "bridge-") {
			continue
		}
		count++
		Colliders = []tiled.Region{body}
		centre := Point{body.X + body.Width/2, body.Y + body.Height/2}
		from := Point{centre.X - 80, centre.Y}
		end := Move(from, Point{X: 160})
		if end.X >= body.X-Radius+.001 || end.Y != from.Y {
			t.Errorf("visible bridge support lost its solid footprint: %s %+v", body.Name, end)
		}
	}
	if count != 4 {
		t.Fatalf("expected all four visible bridge supports, found %d", count)
	}
}

func TestQuestRoutesStartOnOffCentreBridgePlanks(t *testing.T) {
	x, north, south := bridgeDeck(t)
	for _, trip := range []struct{ from, station Point }{
		{Point{x, math.Ceil(north) + 2}, QuestStations[0]},
		{Point{x, math.Floor(south) - 2}, QuestStations[2]},
		{Point{x - 122.3156579317044, 1448}, QuestStations[2]},
		// Actual failed west approach: a route along y1448 was exactly
		// tangent to the far post. The sub-ULP Y correction never advanced.
		{Point{x - 122.3156579317044, 1447.9999999999998}, QuestStations[2]},
	} {
		followQuestRoute(t, trip.from, trip.station, WalkSpeed()/60)
	}
}
