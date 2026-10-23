package main

import (
	"errors"

	"example.com/vigil/.karty/engine"
	"example.com/vigil/internal/dungeon"
)

var errPassageBlocked = errors.New("The passage is blocked.")

type navigator struct {
	path               dungeon.Navigation
	held               bool
	pointer            uint32
	pointerX, pointerY float32
	routePending       bool
}

// requestPath queues a bounded route; failed starts and searches share the HUD message.
func (n *navigator) requestPath(game *engine.Game, level engine.LevelHandle, position, target dungeon.Point) error {
	start, ok := dungeon.RouteStart(position)
	if !ok {
		return errPassageBlocked
	}
	goal := dungeon.ProjectGoal(target)
	request, err := game.World(level).FindPath(engine.NavigationAgent{
		Position: engine.Vec3{
			X: float32(start.X),
			Y: float32(start.Y),
		},
		Radius:         engine.Vec2{X: dungeon.Radius + .001, Y: dungeon.Radius + .001},
		Height:         1.6,
		MaxSearchCells: dungeon.Cells,
	}, engine.Vec3{X: float32(goal.X), Y: float32(goal.Y)})
	if err != nil {
		return errPassageBlocked
	}

	n.path.Begin(uint32(request), uint32(level), start)
	return nil
}

// acceptPath preserves the fixed waypoint bound, including rejected or oversized results.
func (n *navigator) acceptPath(result engine.PathResult, position dungeon.Point) error {
	var points [dungeon.MaxRoutePoints]dungeon.Point
	count := 0
	if result.Status == engine.PathFound && len(result.Points) <= len(points) {
		count = len(result.Points)
		for i, p := range result.Points {
			points[i] = dungeon.Point{X: float64(p.X), Y: float64(p.Y)}
		}
	}
	matched, usable := n.path.Accept(uint32(result.RequestID), uint32(result.Handle), position, points[:count])
	if matched && !usable {
		return errPassageBlocked
	}
	return nil
}

// Retain the latest drag sample; the update loop sends at most one query per frame.
func (n *navigator) point(p engine.PointerInput) {
	if p.Phase == engine.PointerDown && p.Buttons&1 != 0 {
		n.pointer = p.ID
		n.held = true
		n.pointerX, n.pointerY = p.X, p.Y
		n.routePending = true
	} else if p.Phase == engine.PointerUp && p.ID == n.pointer {
		n.held = false
		if n.routePending {
			n.pointerX, n.pointerY = p.X, p.Y
		}
	} else if p.Phase == engine.PointerMove && p.ID == n.pointer && n.held && p.Buttons&1 != 0 {
		n.pointerX, n.pointerY = p.X, p.Y
		n.routePending = true
	}
}
