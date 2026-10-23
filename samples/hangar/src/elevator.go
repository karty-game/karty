package main

import (
	"example.com/hangar/.karty/engine"
	"example.com/hangar/internal/elevator"
)

func (scene *hangarScene) updateElevator(host *engine.Game) {
	if scene.level == 0 {
		return
	}
	z := elevator.Floor(scene.elevatorFrame)
	scene.elevatorFrame++
	host.World(scene.level).SetFloorZ("acid-control-west-lift#0", z)
	// The host moves the collider and carries its rider without camera teleports.
}
