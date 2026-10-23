package main

import "example.com/hangar/.karty/engine"

type hangarScene struct {
	level         engine.LevelHandle
	crates        [len(freightStacks)]*engine.Solid3D
	elevatorFrame uint64
}

// Initialize physics and freight only for the level this scene requested.
func (scene *hangarScene) ready(host *engine.Game, event engine.LevelReady) {
	scene.level = event.Handle
	scene.elevatorFrame = 0
	host.World(scene.level).SetFloorZ("acid-control-west-lift#0", .2)
	if err := host.World(scene.level).
		Configure(engine.WorldSettings{Mode: engine.WorldSpatial, Gravity: engine.Vec3{Z: -9.81}}); err != nil {
		panic(err)
	}
	scene.createCrates(host)
}
