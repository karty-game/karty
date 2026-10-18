package dungeon

import (
	"slices"
	"testing"
)

func TestVisionCacheTracksCameraPoseAndWallInputs(t *testing.T) {
	// The mutable authored outline is intentionally exercised without parallelism.
	saved := Outline
	defer func() { Outline = saved }()
	from, focus, height := Spawn, Point{Spawn.X + .8, Spawn.Y + .8}, 14.0
	var v Vision
	v.Update(from, height, focus)
	for _, change := range []func(){
		func() { from.X += .2 },
		func() { focus.X += .2 },
		func() { focus.Y -= .2 },
		func() { height = 10 },
		func() { v.HeroFrame = 5 },
		func() { v.HeroOffset = Point{.06, -.06} },
		func() { Outline[0].X += .1 },
		func() { from = Point{-10, -10} },
		func() { from = Spawn },
	} {
		change()
		if !v.Update(from, height, focus) {
			t.Fatal("changed camera, pose or walls reused stale projection")
		}
		fresh := Vision{HeroFrame: v.HeroFrame, HeroOffset: v.HeroOffset}
		fresh.Update(from, height, focus)
		if v.Count != fresh.Count || !slices.Equal(v.Fog[:v.Count], fresh.Fog[:fresh.Count]) {
			t.Fatal("cached projection differs from fresh geometry")
		}
		previous := v.Fog
		if v.Update(from, height, focus) {
			t.Fatal("idle projection recomputed")
		}
		if v.Fog != previous {
			t.Fatal("idle projection changed")
		}
	}
}

func BenchmarkVisionIdle(b *testing.B) {
	var v Vision
	v.Update(Spawn, 14, Spawn)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		v.Update(Spawn, 14, Spawn)
	}
}

func BenchmarkVisionMoving(b *testing.B) {
	var v Vision
	b.ReportAllocs()
	for b.Loop() {
		v.Update(Spawn, 14, Spawn)
		v.Update(Point{Spawn.X + .01, Spawn.Y}, 14, Spawn)
	}
}
