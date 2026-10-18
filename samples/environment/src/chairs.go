package main

import (
	"math"

	"example.com/environment/.karty/engine"
)

// Match the generator's three table centers. Four chairs face each tabletop,
// leaving the doorway and central walking route clear.
var diningTables = [...]struct{ x, y float32 }{{-5.5, -8.3}, {6.5, -9.9}, {-6, 2}}
var diningSeats = [...]struct{ x, y, yaw float32 }{
	{0, -1.4, 0}, {0, 1.4, math.Pi}, {-2.05, 0, -math.Pi / 2}, {2.05, 0, math.Pi / 2},
}

func (game *Game) createChairs() {
	for tableIndex, table := range diningTables {
		for seatIndex, seat := range diningSeats {
			transform := engine.NewTransform3D(engine.NewVec3(table.x+seat.x, table.y+seat.y, .555))
			transform.Yaw = seat.yaw
			chair := game.NewWorldSolid(game.level, transform, engine.SolidRenderer{Prefab: "chair"})
			if _, err := game.World(game.level).
				AttachBody(chair.Ref(), engine.BodySettings{Kind: engine.BodyDynamic, Collider: engine.BoxCollider{HalfExtents: engine.NewVec3(.38, .4, .54)}, Rotate: true, Forces: engine.BodyForces{GravityScale: 1, Buoyancy: true}, Mass: 6, Friction: .55, Restitution: .02}); err != nil {
				panic(err)
			}

			game.chairs[tableIndex*len(diningSeats)+seatIndex] = chair
		}
	}
}
func (game *Game) clearChairs() {
	for index, chair := range game.chairs {
		if chair != nil {
			chair.Destroy()
			game.chairs[index] = nil
		}
	}
}
