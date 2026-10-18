package dungeon

import (
	"math"
	"testing"
)

func TestSightStopsAtWallsAndReopensAtCorners(t *testing.T) {
	for _, test := range []struct {
		from, to Point
		want     bool
	}{
		{Spawn, Point{12, 23}, true}, // long open nave is not a radial reveal
		{Spawn, Point{2, 9}, false},
		{Spawn, Point{22, 9}, false},
		{Point{12, 12}, Shrines[0], true},
		{Point{12, 12}, Shrines[1], true},
		{Point{2, 16}, Point{12, 24}, false},
		{Point{12, 12}, Point{24, 12}, true}, // first wall remains visible
		{Point{12, 12}, Point{25, 12}, false},
		{Point{12, 12}, Point{8, 6}, true}, // low pew is not a sight wall
	} {
		if got := Visible(test.from, test.to); got != test.want {
			t.Fatalf("sight %v -> %v = %v", test.from, test.to, got)
		}
		if test.want && !Visible(test.to, test.from) && inside(test.to.X, test.to.Y) {
			t.Fatal("sight not reciprocal", test)
		}
	}
}

func masked(v *Vision, p Point) bool {
	for _, b := range v.Fog[:v.Count] {
		if p.X >= b.X && p.X < b.X+b.W && p.Y >= b.Y && p.Y < b.Y+b.D {
			return true
		}
	}
	return false
}

func TestProjectedFogRevealsAndConcealsRooms(t *testing.T) {
	var v Vision
	for _, from := range []Point{Spawn, {12, 12}, Spawn} {
		focus := Point{from.X + .8, from.Y + .8}
		v.Update(from, 14, focus)
		for _, p := range []Point{{2, 9}, {12, 8}} {
			screen := GroundToScreen(p, 0, 14, focus)
			if screen.X < 0 || screen.X > 1280 || screen.Y < 0 || screen.Y > 720 {
				continue
			}
			if got, want := masked(&v, screen), !Visible(from, p); got != want {
				t.Fatalf("mask from=%v target=%v screen=%v got=%v want=%v", from, p, screen, got, want)
			}
		}
	}
}

func TestVisionStorageAndProjection(t *testing.T) {
	var v Vision
	for _, height := range []float64{10, 14, 22} {
		for _, from := range []Point{Spawn, {12, 8.5}, {4, 12}, {21, 12}, {12, 22}, {6.5, 16.5}, {17.5, 8.5}} {
			focus := Point{from.X + .8, from.Y + .8}
			v.Update(from, height, focus)
			if v.Count < 1 || v.Count >= MaxFog || v.Fog[0].D == 720 {
				t.Fatalf("mask storage/fallback at %v: %d", from, v.Count)
			}
			p := GroundToScreen(from, 0, height, focus)
			if back := ScreenToGround(p.X, p.Y, height, focus); Distance(back, from) > 1e-8 {
				t.Fatal(back, from)
			}
			if masked(&v, p) {
				t.Fatal("player floor masked", from)
			}
		}
	}
	if n := testing.AllocsPerRun(10, func() { v.Update(Spawn, 14, Spawn) }); n != 0 {
		t.Fatal("vision allocations", n)
	}
	// Sweep the whole footprint, including both sides of concave corners.
	for y := .5; y < 25; y++ {
		for x := .5; x < 24; x++ {
			p := Point{x, y}
			if !Walkable(p) {
				continue
			}
			v.Update(p, 14, Point{x + .8, y + .8})
			if v.Count >= MaxFog || v.Fog[0].D == 720 {
				t.Fatal("mask pool exhausted", p, v.Count)
			}
		}
	}
}

func TestVisibleColumnCrownRetainsItsGeometry(t *testing.T) {
	from := Point{12, 22.7}
	focus := Point{from.X + .8, from.Y + .8}
	var v Vision
	v.Update(from, 14, focus)
	for _, sprite := range sightSprites {
		if sprite.position != (Point{16.45, 15.4}) {
			continue
		}
		if !Visible(from, sprite.position) {
			t.Fatal("fixture column must be visible")
		}
		column := Obstacles[7]
		crown := GroundToScreen(Point{column.X + column.W/2, column.Y + column.D/2}, 3.22, 14, focus)
		if masked(&v, crown) {
			t.Fatal("ground-only mask clipped visible column crown", crown)
		}
		return
	}
	t.Fatal("authored column geometry missing")
}

func TestRenderedHeroRetainsItsSilhouette(t *testing.T) {
	// Hasten toward screen-right beside the entrance corner at maximum zoom.
	// The east-facing silhouette moves beyond the old hull, while accepted sight
	// must remain at from. Pose 5 supplies a real submitted clipping witness.
	from := Point{17.65, .35}
	offset := Point{.075 / math.Sqrt2, -.075 / math.Sqrt2}
	next := Move(from, offset)
	if Distance(next, Point{from.X + offset.X, from.Y + offset.Y}) > 1e-9 {
		t.Fatal("fixture move must be unobstructed", next)
	}
	focus := Point{from.X + .8, from.Y + .8}
	base := GroundToScreen(next, .03, 10, focus)
	witnessed := false
	for frame := range WalkFrames {
		pose := Facing(offset.X, offset.Y)*WalkFrames + frame
		old := Vision{HeroFrame: pose}
		current := Vision{HeroFrame: pose, HeroOffset: offset}
		old.Update(from, 10, focus)
		current.Update(from, 10, focus)
		if current.rays != old.rays {
			t.Fatal("rendered hero offset advanced accepted line of sight")
		}
		hull := artHulls[pilgrimHulls[pose]]
		for i, a := range hull.points[:hull.count] {
			b := hull.points[(i+1)%hull.count]
			for _, point := range [...]Point{a, {(a.X + b.X) / 2, (a.Y + b.Y) / 2}} {
				p := Point{base.X + (point.X-HeroOriginX)*HeroProjectedSize*72, base.Y + (point.Y-HeroOriginY)*HeroProjectedSize*72}
				if masked(&old, p) && !masked(&current, p) {
					witnessed = true // A real submitted point exposes the old clipping.
				}
			}
		}
	}
	if !witnessed {
		t.Fatal("fixture must expose the trailing-mask regression in a complete walk cycle")
	}
}

func TestWalkCadenceAndSeam(t *testing.T) {
	var w WalkCycle
	for cycle := range 3 {
		for frame := range WalkFrames {
			got := w.Advance(WalkStride / WalkFrames)
			if want := (frame + 1) % WalkFrames; got != want {
				t.Fatalf("cycle %d frame %d: got %d want %d", cycle, frame, got, want)
			}
			if pause := w.Advance(0); pause != got {
				t.Fatal("waypoint pause reset phase")
			}
		}
	}
	var walk, run WalkCycle
	for range 60 {
		walk.Advance(MoveSpeed / 60)
		run.Advance(HastenSpeed / 60)
	}
	if math.Abs(walk.Distance-math.Mod(MoveSpeed, WalkStride)) > 1e-9 {
		t.Fatal("walk stride drift", walk)
	}
	if math.Abs(run.Distance-math.Mod(HastenSpeed, WalkStride)) > 1e-9 {
		t.Fatal("run stride drift", run)
	}
}
