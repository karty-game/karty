package main

import "example.com/hangar/internal/fps"

func (game *Game) updateElevator() {
	if game.level == 0 {
		return
	}
	z := fps.ElevatorFloor(game.elevatorFrame)
	game.elevatorFrame++
	game.World(game.level).SetFloorZ("acid-control-west-lift#0", z)
	// Step assistance uses the same live floor. The host's kinematic collider
	// carries the player; the cartridge never changes the camera's Z for a ride.
	fps.SetFloorZ("acid-control-west-lift#0", z)
}
