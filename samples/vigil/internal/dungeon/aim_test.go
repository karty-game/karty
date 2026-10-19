package dungeon

import (
	"math"
	"testing"

	"github.com/karty-game/controllers"
)

func TestFireballMatchesProjectedKeyboardMovement(t *testing.T) {
	for _, keys := range [][]controllers.Control{
		{controllers.Forward}, {controllers.Forward, controllers.Right},
		{controllers.Right}, {controllers.Backward, controllers.Right},
		{controllers.Backward}, {controllers.Backward, controllers.Left},
		{controllers.Left}, {controllers.Forward, controllers.Left},
	} {
		controller, err := controllers.NewIsometric(controllers.DefaultIsometricOptions())
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range keys {
			controller.Key(key, true)
		}
		step := controller.Step(controllers.Vec2{X: Spawn.X, Y: Spawn.Y})
		next := Move(Spawn, Point{step.X, step.Y})
		movement := Point{next.X - Spawn.X, next.Y - Spawn.Y}
		direction := AimDirection(FacingDirection(4), movement)
		assertFireballHeading(t, Spawn, movement, direction)
	}
}

func TestFireballMatchesArbitraryClickHeading(t *testing.T) {
	controller, err := controllers.NewIsometric(controllers.DefaultIsometricOptions())
	if err != nil {
		t.Fatal(err)
	}
	for _, offset := range []controllers.Vec2{{X: 70, Y: -25}, {X: -40, Y: -90}, {X: 13, Y: 70}, {X: -80, Y: 5}} {
		target, ok := controller.ScreenToGround(controllers.Vec2{X: 640 + offset.X, Y: 360 + offset.Y},
			controllers.Vec2{X: Spawn.X, Y: Spawn.Y}, 1280, 720)
		if !ok {
			t.Fatal("valid click projection failed")
		}
		controller.SetTarget(target)
		step := controller.Step(controllers.Vec2{X: Spawn.X, Y: Spawn.Y})
		next := Move(Spawn, Point{step.X, step.Y})
		movement := Point{next.X - Spawn.X, next.Y - Spawn.Y}
		direction := AimDirection(FacingDirection(Facing(movement.X, movement.Y)), movement)
		assertFireballHeading(t, Spawn, movement, direction)
	}
}

func TestFireballRetainsAcceptedHeadingWhenStandingOrBlocked(t *testing.T) {
	from := Point{8, 5.1}
	requested := Point{.2, .4}
	next := Move(from, requested)
	movement := Point{next.X - from.X, next.Y - from.Y}
	direction := AimDirection(FacingDirection(4), movement)
	if movement.X <= 0 || movement.Y >= requested.Y {
		t.Fatal("fixture did not slide along the pew", movement)
	}
	assertFireballHeading(t, from, movement, direction)
	if standing := AimDirection(direction, Point{}); standing != direction {
		t.Fatal("standing changed the last movement heading", standing, direction)
	}
	wall := Point{12, .29}
	blocked := Move(wall, Point{Y: -1})
	if blocked != wall {
		t.Fatal("fixture did not block movement", blocked)
	}
	if stopped := AimDirection(direction, Point{blocked.X - wall.X, blocked.Y - wall.Y}); stopped != direction {
		t.Fatal("blocked movement changed aim", stopped, direction)
	}
	initial := FacingDirection(4)
	if AimDirection(initial, Point{}) != initial {
		t.Fatal("stationary spawn changed its north-facing aim")
	}
}

func assertFireballHeading(t *testing.T, from, movement, direction Point) {
	t.Helper()
	if math.Abs(math.Hypot(direction.X, direction.Y)-1) > 1e-12 {
		t.Fatal("projectile heading was not normalized", direction)
	}
	// Project through the actual yaw45/pitch-30 camera basis, independently
	// from the eight sprite-row angles used by Facing/FacingDirection.
	project := func(p Point) Point {
		const yaw, pitch = math.Pi / 4, -math.Pi / 6
		return Point{p.X*math.Cos(yaw) - p.Y*math.Sin(yaw),
			(p.X*math.Sin(yaw) + p.Y*math.Cos(yaw)) * math.Sin(pitch)}
	}
	next, moving := FireballStep(from, direction)
	if !moving {
		t.Fatal("open-floor heading fixture blocked the fireball", from, direction)
	}
	a, b := project(movement), project(Point{next.X - from.X, next.Y - from.Y})
	cross := (a.X*b.Y - a.Y*b.X) / (math.Hypot(a.X, a.Y) * math.Hypot(b.X, b.Y))
	if math.Abs(cross) > 1e-10 || a.X*b.X+a.Y*b.Y <= 0 {
		t.Fatalf("fireball changed the projected movement heading: movement%+v projectile%+v cross%g", a, b, cross)
	}
}
