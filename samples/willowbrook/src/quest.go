package main

import (
	"math"

	"example.com/willowbrook/.karty/engine"
	"example.com/willowbrook/internal/village"
)

type treasureQuest struct {
	level       engine.LevelHandle
	stage       int
	cache       *engine.Sprite2D
	journalOpen bool
	lastOpen    bool
	message     string
	text        string
}

func (q *treasureQuest) start(view *tiledView, player *villagePlayer, hud *villageHUD) {
	q.level = view.level
	player.inspectTarget = -1
	q.message = village.QuestMessages[0]
	for i, p := range village.QuestStations {
		asset := clueNoteTexture
		width, height := village.QuestVisualSize(i)
		if i == 2 {
			asset = treasureClosedTexture
		}
		sprite := view.footSprite(asset, p, width, height)
		if i == 2 {
			q.cache = sprite
		}
	}
	q.update(player, hud)
}

func (q *treasureQuest) restart(player *villagePlayer, hud *villageHUD) {
	q.stage = 0
	q.message = village.QuestMessages[0]
	q.journalOpen = false
	q.cache.SetLevelTexture(engine.LevelTexture(q.level, treasureClosedTexture))
	q.update(player, hud)
}

func (q *treasureQuest) nearby(position village.Point) int {
	nearest, distance := -1, math.Inf(1)
	for i := range village.QuestStations {
		p := village.QuestStations[i]
		d := math.Hypot(p.X-position.X, p.Y-position.Y)
		if d < distance && village.CanInteract(position, village.QuestStations[i]) {
			nearest, distance = i, d
		}
	}
	return nearest
}

func (q *treasureQuest) inspectNearby(player *villagePlayer, hud *villageHUD) {
	if q.journalOpen {
		q.close(player, hud)
		return
	}
	if i := q.nearby(player.position); i >= 0 {
		q.inspect(i, player, hud)
	}
}

// Only the next station advances the hunt; revisits show that station's inscription.
func (q *treasureQuest) inspect(i int, player *villagePlayer, hud *villageHUD) {
	if !village.CanInteract(player.position, village.QuestStations[i]) {
		return
	}
	player.controller.CancelTarget()
	player.clearRoute()
	if i == q.stage {
		q.stage++
		q.message = village.QuestMessages[q.stage]
		if q.stage == 3 {
			q.cache.SetLevelTexture(engine.LevelTexture(q.level, treasureOpenTexture))
		}
	} else if i > q.stage {
		if q.stage == 0 {
			q.message = "A GARDENER'S SEAL\nStart with the sign beside\nthe lower cottage."
		} else {
			q.message = "THE CACHE IS STILL LOCKED\nA second sign waits beside\nthe garden fence, up the steps."
		}
	} else {
		// Revisiting a panel shows its own inscription; quest progression and
		// the compact objective remain unchanged.
		q.message = village.QuestMessages[i+1]
	}
	q.journalOpen = true
	q.update(player, hud)
}

// Include the sprite's height when picking a sign on the upper terrace.
func (q *treasureQuest) selectAt(x, y float64, camera engine.Camera2DSettings, player *villagePlayer, hud *villageHUD) bool {
	for i, p := range village.QuestStations {
		screen := camera.WorldToScreen(engine.NewVec2(float32(p.X), float32(p.Y)), 1280, 720)
		sx, sy, zoom := float64(screen.X), float64(screen.Y), float64(camera.Zoom)
		if !village.QuestHit(i, x, y, sx, sy, zoom) {
			continue
		}
		if village.CanInteract(player.position, village.QuestStations[i]) {
			q.inspect(i, player, hud)
			return true
		}
		player.route = village.QuestRoute(player.position, p)
		q.journalOpen = false
		player.controller.CancelTarget()
		if len(player.route) != 0 {
			player.inspectTarget = i
		} else {
			q.message = "FIND A CLEAR APPROACH\nWalk around the nearby scenery,\nthen tap the sign or chest again."
			q.journalOpen = true
		}
		q.update(player, hud)
		return true
	}
	return false
}
