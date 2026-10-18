package village

import (
	"math"
	"testing"

	"github.com/karty-game/tiled"
)

func TestRiverbankRocksHaveSolidBodiesFromEverySide(t *testing.T) {
	original := Colliders
	t.Cleanup(func() { Colliders = original })
	count := 0
	for _, layer := range authoredLevel.Layers {
		for _, rock := range layer.Objects {
			if rock.Class != "rocks" {
				continue
			}
			count++
			var body *tiled.Region
			for i := range original {
				if original[i].Name == rock.Name+"-body" {
					body = &original[i]
				}
			}
			if body == nil {
				t.Errorf("%s has no solid body; its visible stones can be walked through", rock.Name)
				continue
			}
			// Exclude river/cliff colliders: they otherwise hide missing rock
			// coverage on these bank-side props.
			Colliders = []tiled.Region{*body}
			anchor := Point{rock.X, rock.Y - rock.Height*.45}
			for _, p := range []Point{
				{rock.X - rock.Width*.15, rock.Y - rock.Height*.65}, // Tall stone's body.
				{rock.X + rock.Width*.25, rock.Y - rock.Height*.22}, // Lower right-hand boulder.
			} {
				if Walkable(p) {
					t.Errorf("%s leaves a visible boulder walkable at %+v", rock.Name, p)
				}
			}
			for _, direction := range []Point{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
				start := Point{anchor.X - direction.X*120, anchor.Y - direction.Y*120}
				end := Move(start, Point{X: direction.X * 240, Y: direction.Y * 240})
				if !Walkable(end) || (end.X-anchor.X)*direction.X+(end.Y-anchor.Y)*direction.Y >= 0 {
					t.Errorf("movement crossed %s from %+v: %+v -> %+v", rock.Name, direction, start, end)
				}
			}
		}
	}
	if count < 16 {
		t.Fatalf("expected the authored riverbank boulders, found %d", count)
	}
}

func TestWaterfallBasinBanksAreSolidOutsideRiverChannel(t *testing.T) {
	original := Colliders
	t.Cleanup(func() { Colliders = original })
	Colliders = nil
	for _, body := range original {
		if body.Name == "waterfall-basin-west-rim" || body.Name == "waterfall-basin-east-rim" {
			Colliders = append(Colliders, body)
		}
	}
	if len(Colliders) != 2 {
		t.Fatal("waterfall requires both authored basin rims")
	}
	// These visible bank stones extend outside the old straight water collider.
	// Isolate their shapes so the river cannot hide a missing bank footprint.
	for _, p := range []Point{{2018, 1290}, {2206, 1290}} {
		if Walkable(p) {
			t.Errorf("waterfall bank rock is walkable at %+v", p)
		}
	}
	if !Walkable(Point{2112, 1290}) {
		t.Fatal("basin bank shapes cover the open water channel")
	}
}

func TestStairWallsKeepWalkingSolesOutsideStone(t *testing.T) {
	original := Colliders
	t.Cleanup(func() { Colliders = original })
	Colliders = nil
	for _, collider := range original {
		if collider.Name == "staircase-left-cheek" || collider.Name == "staircase-right-cheek" {
			Colliders = append(Colliders, collider)
		}
	}
	if len(Colliders) != 2 {
		t.Fatal("missing independently authored stair walls")
	}
	// The scaled 32-pixel hero has soles about 18 pixels wide in neutral
	// stance, wider while walking. Keep a 28-pixel walking footprint clear
	// rather than merely stopping the actor's centre inside a stone cheek.
	const soleHalfWidth = 14.0
	for _, y := range []float64{1146, 1170, 1200, 1240, 1276} {
		for _, direction := range []float64{-1, 1} {
			end := Move(Point{1728, y}, Point{X: direction * 200})
			if !Walkable(end) || end.X-soleHalfWidth < 1678-.01 || end.X+soleHalfWidth > 1778+.01 {
				t.Errorf("walking soles enter stair stone at height %g, direction %g: %+v", y, direction, end)
			}
		}
	}
	// Wider side clearance must preserve the complete climb and descent.
	for _, route := range [][2]Point{{{1728, 1324}, {1728, 1110}}, {{1728, 1110}, {1728, 1324}}} {
		end := Move(route[0], Point{Y: route[1].Y - route[0].Y})
		if math.Hypot(end.X-route[1].X, end.Y-route[1].Y) > .001 {
			t.Errorf("stair centre is blocked: %+v -> %+v, wanted %+v", route[0], end, route[1])
		}
	}
	// The actual landing between the upper cottage and the cheeks is only
	// 28 pixels deep. The wider walking soles must still fit there and turn
	// onto the terrace instead of becoming trapped above the first tread.
	Colliders = original
	start := Point{1728, 1324}
	landing := Move(start, Point{Y: -204})
	terrace := Move(landing, Point{X: -160})
	if math.Hypot(landing.X-1728, landing.Y-1120) > .001 || terrace.X > 1568.001 {
		t.Errorf("cannot leave the top stair landing: %+v -> %+v -> %+v", start, landing, terrace)
	}
}

func TestDiagonalWalkingSlidesPastCottageAnnexCorner(t *testing.T) {
	var house tiled.Object
	for _, layer := range authoredLevel.Layers {
		for _, object := range layer.Objects {
			if object.Class == "cottage" && object.Y > house.Y {
				house = object
			}
		}
	}
	if house.Name == "" {
		t.Fatal("missing lower cottage")
	}
	for _, approach := range []struct{ from, delta Point }{
		{Point{house.X + 192, house.Y - 30}, Point{X: -240}},
		{Point{house.X, house.Y + 50}, Point{Y: -240}},
	} {
		end := Move(approach.from, approach.delta)
		if !Walkable(end) || approach.delta.Y == 0 && end.Y != approach.from.Y || approach.delta.X == 0 && end.X != approach.from.X {
			t.Fatalf("straight push drifted along a wall: %+v -> %+v", approach.from, end)
		}
		if approach.delta.X < 0 && end.X < house.X+124+Radius || approach.delta.Y < 0 && end.Y < house.Y+RadiusY {
			t.Fatalf("straight push crossed a wall: %+v -> %+v", approach.from, end)
		}
	}
	step := WalkSpeed() / 60 / math.Sqrt2
	// Actual failed browser approach, plus nearby phases. Rejecting a whole
	// axis step at the rounded corner used to strand otherwise valid feet
	// just inside the straight side's eventual clearance (x≈1481.996).
	for _, offset := range []float64{-.1505, -.65, .35} {
		p := Point{house.X + 192 + offset, house.Y + 50.0411}
		for range 90 {
			p = Move(p, Point{X: -step, Y: -step})
			if !Walkable(p) {
				t.Fatalf("corner slide escaped solid validation at %+v", p)
			}
			if p.Y < house.Y-6 && p.Y > house.Y-88 && p.X < house.X+124+Radius-.001 {
				t.Fatalf("corner slide penetrated annex's straight side at %+v", p)
			}
		}
		if p.X < house.X+124+Radius-2 || p.Y >= house.Y-20 {
			t.Errorf("held diagonal input snagged at annex corner (phase %g): %+v", offset, p)
		}
	}
}
