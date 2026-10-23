package main

import (
	"example.com/willowbrook/.karty/engine"
	"example.com/willowbrook/internal/village"
	"github.com/karty-game/controllers-topdown"
)

func navigationPoint(point village.Point) engine.Vec2 {
	return engine.NewVec2(float32(point.X/village.NavigationScale), float32(point.Y/village.NavigationScale))
}

// Publish authored collision geometry for routing; movement still checks village.Move.
func (p *villagePlayer) configureWorld(game *engine.Game, level engine.LevelHandle) {
	geometry, err := village.NavigationMap()
	if err != nil {
		panic(err)
	}
	world := game.World(level)
	if err := world.Configure(engine.WorldSettings{
		Mode: engine.WorldPlanar, Min: navigationPoint(geometry.Min), Max: navigationPoint(geometry.Max),
		CellSize: village.NavigationCell / village.NavigationScale,
	}); err != nil {
		panic(err)
	}
	for _, obstacle := range geometry.Obstacles {
		var points [16]engine.Vec2
		for index, point := range obstacle {
			points[index] = navigationPoint(point)
		}
		if err := world.AddObstacle(points[:len(obstacle)]); err != nil {
			panic(err)
		}
	}
}

func (p *villagePlayer) requestPath(game *engine.Game, level engine.LevelHandle, target village.Point) {
	p.controller.CancelTarget()
	start, startOK := village.NavigationEndpoint(p.position)
	goal, goalOK := village.NavigationEndpoint(target)
	if !startOK || !goalOK {
		return
	}
	from, to := navigationPoint(start), navigationPoint(goal)
	request, err := game.World(level).FindPath(engine.NavigationAgent{
		Position: engine.NewVec3(from.X, from.Y, 0),
		Radius: engine.NewVec2(float32(village.Radius/village.NavigationScale),
			float32(village.RadiusY/village.NavigationScale)),
		Height: 1,
	}, engine.NewVec3(to.X, to.Y, 0))
	if err == nil {
		p.navigation.Begin(uint32(request), uint32(level), start, target)
	}
}

// Restore map-pixel waypoints within the fixed bound; oversized results become empty routes.
func (p *villagePlayer) pathResult(result engine.PathResult, level engine.LevelHandle) {
	if uint32(result.RequestID) != p.navigation.Pending || result.Handle != level {
		return
	}
	var points [128]village.Point
	count := 0
	if result.Status == engine.PathFound && len(result.Points) <= len(points) {
		count = len(result.Points)
		for index, point := range result.Points {
			points[index] = village.Point{X: float64(point.X) * village.NavigationScale, Y: float64(point.Y) * village.NavigationScale}
		}
	}
	p.route = p.navigation.Accept(uint32(result.RequestID), uint32(result.Handle), p.position, points[:count])
}

// Clear the arrival action with the request so late results cannot reopen a journal page.
func (p *villagePlayer) clearRoute() {
	p.route = nil
	p.inspectTarget = -1
	p.navigation.Cancel()
}

func (p *villagePlayer) advanceRoute() {
	tolerance := 4.0
	if p.inspectTarget < 0 {
		// Smoothed engine routes may pass close to an obstacle corner. Reach
		// their waypoints before turning; quest-grid routes retain their margin.
		tolerance = .5
	}
	p.route = village.AdvanceRoute(p.route, p.position, tolerance)
	if len(p.route) > 0 {
		p.controller.SetTarget(controllers.Vec2{X: p.route[0].X, Y: p.route[0].Y})
	}
}
