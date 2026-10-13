package main

import "example.com/hangar/.karty/engine"

type freight struct {
	x, y, z, width, depth, height, mass float32
}

// The five original freight boxes keep their room and material. Arrival's
// crate sits just clear of the player spawn. Loading's three stacks each get
// a medium and a small crate, with a little room for gravity to settle them.
var freightStacks = [...]freight{
	{10, -4.4, .66, 1.2, 1.2, 1.3, 28},
	{-3, 7, .81, 2, 2, 1.6, 48},
	{26.8, 7.8, 1.21, 1.6, 1.6, 1.6, 36},
	{28.9, 7.8, 1.21, 1.6, 1.6, 1.6, 36},
	{31, 7.8, 1.21, 1.6, 1.6, 1.6, 36},
	{26.65, 7.75, 2.42, .8, .8, .8, 8},
	{26.7, 7.8, 3.08, .5, .5, .5, 3},
	{29.05, 7.8, 2.42, .8, .8, .8, 8},
	{29, 7.75, 3.08, .5, .5, .5, 3},
	{30.85, 7.85, 2.42, .8, .8, .8, 8},
	{30.9, 7.8, 3.08, .5, .5, .5, 3},
}

func (game *Game) createCrates() {
	for index, box := range freightStacks {
		transform := engine.NewTransform3D(engine.NewVec3D(box.x, box.y, box.z))
		transform.Scale = engine.NewVec3D(box.width, box.depth, box.height)
		crate := game.NewWorldSolid(game.level, transform, engine.SolidRenderer{Prefab: "cargo-crate"})
		game.SetPhysicsBox(crate, engine.PhysicsBox{
			HalfExtents: engine.NewVec3D(box.width/2, box.depth/2, box.height/2),
			Mass:        box.mass, Friction: .5, Restitution: .02,
		})
		game.WatchTransform(crate, nil)
		game.crates[index] = crate
	}
}
