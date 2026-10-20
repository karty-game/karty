package main

import "example.com/hangar/.karty/engine"

type freight struct {
	x, y, z, width, depth, height, mass float32
}

// The east-ramp freight and four smaller floating boxes demonstrate buoyancy.
// Arrival's crate stays clear of the spawn. Loading's three stacks each get
// a medium and a small crate, with room for gravity to settle them.
var freightStacks = [...]freight{
	{10, -4.4, .66, 1.2, 1.2, 1.3, 28},
	{47, 22, .86, 1.2, 1.2, 1.3, 28},
	{26.8, 7.8, 1.21, 1.6, 1.6, 1.6, 36},
	{28.9, 7.8, 1.21, 1.6, 1.6, 1.6, 36},
	{31, 7.8, 1.21, 1.6, 1.6, 1.6, 36},
	{26.65, 7.75, 2.42, .8, .8, .8, 8},
	{26.7, 7.8, 3.08, .5, .5, .5, 3},
	{29.05, 7.8, 2.42, .8, .8, .8, 8},
	{29, 7.75, 3.08, .5, .5, .5, 3},
	{30.85, 7.85, 2.42, .8, .8, .8, 8},
	{30.9, 7.8, 3.08, .5, .5, .5, 3},
	{25.5, 18.5, -1.20, .8, .8, .8, 8},
	{35.8, 18.4, -1.20, .65, .65, .65, 4.5},
	{38.6, 24.8, -1.20, .7, .7, .7, 5.5},
	{25.4, 29.8, -1.20, .6, .6, .6, 3.5},
}

func (game *Game) createCrates() {
	for index, box := range freightStacks {
		transform := engine.NewTransform3D(engine.NewVec3(box.x, box.y, box.z))
		transform.Scale = engine.NewVec3(box.width, box.depth, box.height)
		crate := game.NewSolid3D(game.level, transform, engine.SolidRenderer{Prefab: "cargo-crate"})
		if _, err := game.World(game.level).
			AttachBody(crate.Ref(), engine.BodySettings{Kind: engine.BodyDynamic, Collider: engine.BoxCollider{HalfExtents: engine.NewVec3(box.width/2, box.depth/2, box.height/2)}, AllowRotation: true, Forces: engine.BodyForceSettings{GravityScale: 1, Buoyancy: true}, Mass: box.mass, Friction: .5, Restitution: .02}); err != nil {
			panic(err)
		}

		game.crates[index] = crate
	}
}
