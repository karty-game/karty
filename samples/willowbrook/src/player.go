package main

import (
	"math"

	"example.com/willowbrook/.karty/engine"
	"example.com/willowbrook/internal/village"
	"github.com/karty-game/controllers-topdown"
)

// Hero art stores three poses per direction in this order.
const (
	faceDown = iota
	faceLeft
	faceRight
	faceUp
)

type villagePlayer struct {
	hero, shadow     *engine.Sprite2D
	position         village.Point
	controller       *controllers.Planar
	facing, lastPose int
	walkPhase        float64
	navigation       village.NavigationRequest
	route            []village.Point
	inspectTarget    int // -1 for ground routes; otherwise inspect this station on arrival.
	interactHeld     bool
}

func (p *villagePlayer) configure() {
	options := controllers.DefaultTopDown2DOptions()
	options.Speed, options.RunSpeed = village.WalkSpeed(), village.RunSpeed()
	options.FollowOffset = controllers.Vec2{Y: village.Level.Properties.Number("cameraOffsetY", -110)}
	options.Zoom = village.Zoom()
	var err error
	p.controller, err = controllers.NewTopDown2D(options)
	if err != nil {
		panic(err)
	}
}

// Animate actual collision-resolved travel; stop automatic walking when blocked.
func (p *villagePlayer) move() bool {
	delta := p.controller.Step(controllers.Vec2{X: p.position.X, Y: p.position.Y})
	dx, dy := delta.X, delta.Y
	next := village.Move(p.position, village.Point{X: dx, Y: dy})
	distance := math.Hypot(next.X-p.position.X, next.Y-p.position.Y)
	if distance > .00001 {
		if math.Abs(dx) > math.Abs(dy) {
			if dx < 0 {
				p.facing = faceLeft
			} else {
				p.facing = faceRight
			}
		} else if dy < 0 {
			p.facing = faceUp
		} else {
			p.facing = faceDown
		}
		p.walkPhase = village.AdvanceWalkPhase(p.walkPhase, distance, village.HeroStride(p.controller.Running()))
	} else if p.controller.HasTarget() && (dx != 0 || dy != 0) {
		p.controller.CancelTarget()
		p.clearRoute()
	}
	p.position = next
	return distance > .00001
}

// Sort both sprites by their feet so scenery can cover the hero naturally.
func (p *villagePlayer) paint(view *tiledView, walking bool) {
	if !walking {
		p.walkPhase = 0
	}
	frame := village.WalkFrame(p.walkPhase, walking)
	pose := p.facing*3 + frame
	width, height := village.Level.Properties.Number("heroWidth", 40), village.Level.Properties.Number("heroHeight", 60)
	shadowAsset := textureIDs["shadow-soft"]
	if p.shadow == nil {
		p.shadow = view.footSprite(shadowAsset, p.position, width*.55, 10)
	}
	dims := textureSizes[shadowAsset]
	p.shadow.SetTransform(
		engine.NewVec2(float32(p.position.X+2), float32(p.position.Y)),
		engine.NewVec2(float32(width*.55)/float32(dims[0]), 10/float32(dims[1])),
		0,
	)
	p.shadow.SetLayer(view.mapLayer(village.ActorLayer, p.position.Y) - 1)
	if p.hero == nil {
		p.hero = view.footSprite(heroTextures[pose], p.position, width, height)
	} else {
		p.hero.SetTransform(
			engine.NewVec2(float32(p.position.X), float32(p.position.Y-height/2)),
			engine.NewVec2(float32(width/32), float32(height/48)),
			0,
		)
		p.hero.SetLayer(view.mapLayer(village.ActorLayer, p.position.Y))
	}
	if pose != p.lastPose {
		p.hero.SetLevelTexture(engine.LevelTexture(view.level, heroTextures[pose]))
		p.lastPose = pose
	}
}

func (p *villagePlayer) resetInput() {
	p.controller.Reset()
	p.interactHeld = false
	p.clearRoute()
}

func movementControl(k engine.Key) (controllers.Control, bool) {
	switch k {
	case engine.KeyW:
		return controllers.Forward, true
	case engine.KeyArrowUp:
		return controllers.ForwardAlt, true
	case engine.KeyS:
		return controllers.Backward, true
	case engine.KeyArrowDown:
		return controllers.BackwardAlt, true
	case engine.KeyA:
		return controllers.Left, true
	case engine.KeyLeft:
		return controllers.LeftAlt, true
	case engine.KeyD:
		return controllers.Right, true
	case engine.KeyRight:
		return controllers.RightAlt, true
	default:
		return 0, false
	}
}
