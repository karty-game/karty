package village

import (
	"math"
	"testing"

	"github.com/karty-game/controllers-topdown"
)

// These smoothed routes were produced by the real engine navigation solver.
// Following must respect the sample's accepted collision poses at both speeds,
// rather than teleporting through endpoints that merely appear reachable.
func TestEngineRoutesFollowAroundCottageAndAcrossBridge(t *testing.T) {
	for _, sample := range []struct {
		name  string
		from  Point
		route []Point
	}{
		{"cottage", Spawn, []Point{{1220, 1356}, {1160, 1440}}},
		{"bridge", Spawn, []Point{{1988, 1452}, {2252, 1452}, {2250, 1380}}},
		{"bridge return", Point{2250, 1380}, []Point{{2252, 1452}, {1868, 1452}, Spawn}},
		{"garden", Spawn, []Point{{1748, 1116}, {1892, 1116}, QuestStations[1]}},
		{"quest finish beside boundary", Point{2304, 1286.1904}, []Point{{2304, 1347}}},
		{"keyboard stop beside boundary", Point{2305.8, 1286.1904}, []Point{{2304, 1347}}},
	} {
		for _, speed := range []float64{WalkSpeed(), RunSpeed()} {
			followEngineRoute(t, sample.name, sample.from, sample.route, speed)
		}
	}
}

func followEngineRoute(t *testing.T, name string, from Point, route []Point, speed float64) {
	t.Helper()
	if !ClearRoute(from, route) {
		t.Fatal("engine route violates physical collisions", name)
	}
	options := controllers.DefaultTopDown2DOptions()
	options.Speed = speed
	controller, err := controllers.NewTopDown2D(options)
	if err != nil {
		t.Fatal(err)
	}
	target, position, bridge := route[len(route)-1], from, false
	for range 4000 {
		route = AdvanceRoute(route, position, .5)
		if len(route) == 0 {
			if math.Hypot(position.X-target.X, position.Y-target.Y) > .5 {
				t.Fatal("ground click stopped before its exact destination", name, position, target)
			}
			if (name == "bridge" || name == "bridge return") && !bridge {
				t.Fatal("engine route skipped the only dry river crossing", name)
			}
			return
		}
		controller.SetTarget(controllers.Vec2{X: route[0].X, Y: route[0].Y})
		delta := controller.Step(controllers.Vec2{X: position.X, Y: position.Y})
		next := Move(position, Point{delta.X, delta.Y})
		if !Walkable(next) || math.Hypot(next.X-position.X, next.Y-position.Y) < .00001 {
			t.Fatal("engine route stalled against authored collision", name, speed, position, route[0])
		}
		position = next
		bridge = bridge || InRegion("bridge", position)
	}
	t.Fatal("engine route exceeded movement budget", name, speed)
}
