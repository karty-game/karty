package main

import (
	"example.com/crate-room/.karty/engine"
)

const crateCount = 80

// crateStack keeps the visible crates and their physics bodies in matching slots.
type crateStack struct {
	crates [crateCount]*engine.Solid3D
	bodies [crateCount]*engine.Body
}

func (stack *crateStack) create(host *engine.Game, level engine.LevelHandle) {
	var err error
	for i := range stack.crates {
		stack.crates[i] = host.NewSolid3D(level, crateTransform(i), engine.SolidRenderer{Prefab: "crate"})
		stack.bodies[i], err = host.World(level).AttachBody(stack.crates[i].Ref(), crateBox())
		if err != nil {
			panic(err)
		}
	}
}

// reset rebuilds each body so falling and spinning crates return to rest.
func (stack *crateStack) reset(host *engine.Game, level engine.LevelHandle) {
	var err error
	for i, crate := range stack.crates {
		if err := stack.bodies[i].Detach(); err != nil {
			panic(err)
		}
		if err := crate.Ref().SetPose(crateTransform(i)); err != nil {
			panic(err)
		}
		stack.bodies[i], err = host.World(level).AttachBody(crate.Ref(), crateBox())
		if err != nil {
			panic(err)
		}
	}
}

// Stack four crates per column in a five-column grid.
func crateTransform(i int) engine.Transform3D {
	return engine.NewTransform3D(engine.NewVec3(float32((i/4)%5-2)*3.5, -1+float32(i/20)*3.5, .51+float32(i%4)*1.015))
}

func crateBox() engine.BodySettings {
	return engine.BodySettings{
		Kind:          engine.BodyDynamic,
		Collider:      engine.BoxCollider{HalfExtents: engine.NewVec3(.5, .5, .5)},
		AllowRotation: true,
		Forces:        engine.BodyForceSettings{GravityScale: 1, Buoyancy: true},
		Mass:          16,
		Friction:      .5,
		Restitution:   .08,
	}
}
