package main

import (
	"example.com/vigil/.karty/engine"
	"example.com/vigil/internal/dungeon"
)

func (g *Game) requestPath(target dungeon.Point) {
	g.nav.Cancel()
	if g.level == 0 || g.resetPending {
		return
	}
	start, ok := dungeon.RouteStart(g.position())
	if !ok {
		g.pathBlocked()
		return
	}
	goal := dungeon.ProjectGoal(target)
	request, err := g.World(g.level).FindPath(engine.NavigationAgent{
		From: engine.Vec3{
			X: float32(start.X),
			Y: float32(start.Y),
		},
		Radius:      engine.Vec2{X: dungeon.Radius + .001, Y: dungeon.Radius + .001},
		Height:      1.6,
		MaxExpanded: dungeon.Cells,
	}, engine.Vec3{X: float32(goal.X), Y: float32(goal.Y)})
	if err != nil {
		g.pathBlocked()
		return
	}

	g.nav.Begin(uint32(request), uint32(g.level), start)
}
func (g *Game) pathResult(result engine.PathResult) {
	if g.hero == nil || g.resetPending || result.Handle != g.level || uint32(result.RequestID) != g.nav.Pending {
		return
	}
	var points [dungeon.MaxRoutePoints]dungeon.Point
	count := 0
	if result.Status == engine.PathFound && len(result.Points) <= len(points) {
		count = len(result.Points)
		for i, p := range result.Points {
			points[i] = dungeon.Point{X: float64(p.X), Y: float64(p.Y)}
		}
	}
	if matched, usable := g.nav.Accept(uint32(result.RequestID), uint32(result.Handle), g.position(), points[:count]); matched && !usable {
		g.pathBlocked()
	}
}
func (g *Game) pathBlocked() { g.status = "The passage is blocked."; g.refresh() }
