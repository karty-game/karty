package main

import (
	"example.com/willowbrook/.karty/engine"
	"example.com/willowbrook/internal/village"
)

func navigationPoint(point village.Point) engine.Vec2 {
	return engine.NewVec2(float32(point.X/village.NavigationScale), float32(point.Y/village.NavigationScale))
}

func (g *Game) configureNavigation() {
	geometry, err := village.NavigationMap()
	if err != nil {
		panic(err)
	}
	world := g.World(g.level)
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

func (g *Game) requestPath(target village.Point) {
	g.controller.CancelTarget()
	start, startOK := village.NavigationEndpoint(g.position)
	goal, goalOK := village.NavigationEndpoint(target)
	if !startOK || !goalOK {
		return
	}
	from, to := navigationPoint(start), navigationPoint(goal)
	request, err := g.World(g.level).FindPath(engine.NavigationAgent{
		Position: engine.NewVec3(from.X, from.Y, 0),
		Radius: engine.NewVec2(float32(village.Radius/village.NavigationScale),
			float32(village.RadiusY/village.NavigationScale)),
		Height: 1,
	}, engine.NewVec3(to.X, to.Y, 0))
	if err == nil {
		g.navigation.Begin(uint32(request), uint32(g.level), start, target)
	}
}

func (g *Game) pathResult(result engine.PathResult) {
	if uint32(result.RequestID) != g.navigation.Pending || result.Handle != g.level {
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
	g.quest.route = g.navigation.Accept(uint32(result.RequestID), uint32(result.Handle), g.position, points[:count])
}
