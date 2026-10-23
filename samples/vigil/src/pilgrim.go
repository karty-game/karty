package main

import (
	"example.com/vigil/.karty/engine"
	"example.com/vigil/internal/dungeon"
	"github.com/karty-game/controllers-isometric"
)

// The shadow is the accepted movement root; the painted body follows its rig.
type pilgrim struct {
	game                *engine.Game
	level               engine.LevelHandle
	hero, shadow        *engine.Sprite3D
	heroTransform       engine.Transform3D
	halo                *engine.Light3D
	watch               engine.PoseSubscription
	controller          *controllers.Planar
	facing, lastTexture int
	aim, lastPosition   dungeon.Point
	walk                dungeon.WalkCycle
	resetPending        bool
	navigation          navigator
}

func (p *pilgrim) configure(height float64) {
	options := controllers.DefaultIsometricOptions()
	options.Speed, options.RunSpeed = dungeon.MoveSpeed, dungeon.HastenSpeed
	options.FollowOffset = controllers.Vec2{X: 1.6, Y: 1.6}
	options.Zoom = height
	var err error
	p.controller, err = controllers.NewIsometric(options)
	if err != nil {
		panic(err)
	}
}

func (p *pilgrim) position() dungeon.Point {
	if p.resetPending {
		return dungeon.Spawn
	}
	return dungeon.Point{float64(p.shadow.Transform().Position.X), float64(p.shadow.Transform().Position.Y)}
}

// acceptedTravel measures host-approved movement, so blocked steps stay silent.
func (p *pilgrim) acceptedTravel(position dungeon.Point) float64 {
	travel := dungeon.Distance(p.lastPosition, position)
	p.lastPosition = position
	return travel
}

// move proposes one step and previews its pose; the host still decides collision results.
func (p *pilgrim) move(position dungeon.Point, travel float64, vision *dungeon.Vision) bool {
	intent := p.controller.Step(controllers.Vec2{X: position.X, Y: position.Y})
	delta := dungeon.Point{X: intent.X, Y: intent.Y}
	if intent.X != 0 || intent.Y != 0 {
		p.navigation.path.Cancel()
		p.navigation.held = false
	} else if p.navigation.path.Next < p.navigation.path.Count {
		target := p.navigation.path.Step(position, p.controller.Speed()/60)
		delta = dungeon.Point{target.X - position.X, target.Y - position.Y}
	}
	next := dungeon.Move(position, delta)
	moved := dungeon.Distance(position, next) > .00001
	if moved {
		p.aim = dungeon.AimDirection(p.aim, dungeon.Point{X: next.X - position.X, Y: next.Y - position.Y})
		p.facing = dungeon.Facing(p.aim.X, p.aim.Y)
		s := p.shadow.Transform()
		s.Position.X = float32(next.X)
		s.Position.Y = float32(next.Y)
		if err := p.shadow.Ref().SetPose(s); err != nil {
			panic(err)
		}
		p.halo.SetPose(engine.NewVec3(float32(next.X), float32(next.Y), 1.63), 0, 0)
	}
	p.walk.Advance(travel)
	phase := p.walk.PhaseAfter(dungeon.Distance(position, next))
	frame := int(phase)
	pose := dungeon.WalkTextureCount + p.facing
	if moved || travel >= .00001 {
		pose = p.facing*dungeon.WalkFrames + frame
	}
	vision.HeroFrame = pose
	vision.HeroOffset = dungeon.Point{next.X - position.X, next.Y - position.Y}
	p.pose(pose, next)
	return moved
}

func movementControl(k engine.Key) (controllers.Control, bool) {
	switch k {
	case engine.KeyW:
		return controllers.Forward, true
	case engine.KeyS:
		return controllers.Backward, true
	case engine.KeyA:
		return controllers.Left, true
	case engine.KeyD:
		return controllers.Right, true
	case engine.KeyArrowUp:
		return controllers.ForwardAlt, true
	case engine.KeyArrowDown:
		return controllers.BackwardAlt, true
	case engine.KeyLeft:
		return controllers.LeftAlt, true
	case engine.KeyRight:
		return controllers.RightAlt, true
	default:
		return 0, false
	}
}
