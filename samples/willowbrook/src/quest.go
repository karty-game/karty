package main

import (
	"math"

	"example.com/willowbrook/.karty/engine"
	"example.com/willowbrook/internal/village"
	"github.com/karty-game/controllers"
)

type treasureQuest struct {
	stage        int
	pending      int
	route        []village.Point
	interactHeld bool
	cache        *engine.Sprite2D
	journalOpen  bool
	lastOpen     bool
	message      string
	text         string
}

func (g *Game) startQuest() {
	g.quest.pending = -1
	g.quest.message = village.QuestMessages[0]
	for i, p := range village.QuestStations {
		asset := clueNoteTexture
		width, height := village.QuestVisualSize(i)
		if i == 2 {
			asset = treasureClosedTexture
		}
		sprite := g.footSprite(asset, p, width, height)
		if i == 2 {
			g.quest.cache = sprite
		}
	}
	g.updateQuest()
}

func (g *Game) restartQuest() {
	g.quest.stage = 0
	g.quest.message = village.QuestMessages[0]
	g.quest.journalOpen = false
	g.quest.cache.SetLevelTexture(engine.LevelTexture(g.level, treasureClosedTexture))
	g.updateQuest()
}

func (g *Game) cancelQuestWalk() {
	g.quest.route = nil
	g.quest.pending = -1
	g.navigation.Cancel()
}

func (g *Game) nearQuest(i int) bool {
	return village.CanInteract(g.position, village.QuestStations[i])
}

func (g *Game) nearbyQuest() int {
	nearest, distance := -1, math.Inf(1)
	for i := range village.QuestStations {
		p := village.QuestStations[i]
		d := math.Hypot(p.X-g.position.X, p.Y-g.position.Y)
		if d < distance && g.nearQuest(i) {
			nearest, distance = i, d
		}
	}
	return nearest
}

func (g *Game) inspectNearby() {
	if g.quest.journalOpen {
		g.closeJournal()
		return
	}
	if i := g.nearbyQuest(); i >= 0 {
		g.inspectQuest(i)
	}
}

func (g *Game) inspectQuest(i int) {
	if !g.nearQuest(i) {
		return
	}
	g.controller.CancelTarget()
	g.cancelQuestWalk()
	if i == g.quest.stage {
		g.quest.stage++
		g.quest.message = village.QuestMessages[g.quest.stage]
		if g.quest.stage == 3 {
			g.quest.cache.SetLevelTexture(engine.LevelTexture(g.level, treasureOpenTexture))
		}
	} else if i > g.quest.stage {
		if g.quest.stage == 0 {
			g.quest.message = "A GARDENER'S SEAL\nStart with the sign beside\nthe lower cottage."
		} else {
			g.quest.message = "THE CACHE IS STILL LOCKED\nA second sign waits beside\nthe garden fence, up the steps."
		}
	} else {
		// Revisiting a panel shows its own inscription; quest progression and
		// the compact objective remain unchanged.
		g.quest.message = village.QuestMessages[i+1]
	}
	g.quest.journalOpen = true
	g.updateQuest()
}

func (g *Game) closeJournal() {
	g.quest.journalOpen = false
	g.updateQuest()
}

func (g *Game) updateQuest() {
	if g.quest.pending >= 0 && g.nearQuest(g.quest.pending) {
		g.inspectQuest(g.quest.pending)
		return
	}
	text := "Tap sign / chest | WASD: walk | R: reset"
	label := [4]string{"JOURNAL: FIND THE COTTAGE SIGN", "JOURNAL: FIND THE GARDEN SIGN", "JOURNAL: FIND THE HIDDEN CHEST", "TREASURE FOUND! TAP FOR JOURNAL"}[g.quest.stage]
	nearby := g.nearbyQuest()
	if nearby >= 0 {
		i := nearby
		if i < 2 {
			text = "E / SPACE / TAP HERE: READ SIGN"
		} else if g.quest.stage == 3 {
			text = "TAP HERE: VIEW REWARD   R: reset"
		} else {
			text = "E / SPACE / TAP HERE: OPEN CHEST"
		}
	} else if g.quest.pending >= 0 {
		text = "Walking to sign...  WASD / tap: cancel"
		if g.quest.pending == 2 {
			text = "Walking to chest... WASD / tap: cancel"
		}
	}
	if g.quest.journalOpen {
		label = g.quest.message
		text = "E / SPACE / TAP HERE: CLOSE\nTap a sign or chest to inspect it"
	}
	combined := label + "\n" + text
	if combined != g.quest.text || g.quest.journalOpen != g.quest.lastOpen {
		g.quest.text = combined
		g.quest.lastOpen = g.quest.journalOpen
		g.hud.Invalidate()
	}
}

// The journal is presentation UI, so its action consumes pointer input before
// world picking and stays readable independently of game-image resolution.
func (g *Game) activateJournal() {
	if !g.loaded {
		return
	}
	g.controller.CancelTarget()
	g.cancelQuestWalk()
	if g.quest.journalOpen {
		g.closeJournal()
	} else if g.nearbyQuest() >= 0 {
		g.inspectNearby()
	} else {
		g.quest.journalOpen = true
		g.updateQuest()
	}
}

// Test the projected sprite, including its height, so tapping a note's picture
// works on the upper terrace as well as tapping its ground contact.
func (g *Game) selectQuest(x, y float64) bool {
	for i, p := range village.QuestStations {
		screen := g.camera.WorldToScreen(engine.NewVec2(float32(p.X), float32(p.Y)), 1280, 720)
		sx, sy, zoom := float64(screen.X), float64(screen.Y), float64(g.camera.Zoom)
		if !village.QuestHit(i, x, y, sx, sy, zoom) {
			continue
		}
		if g.nearQuest(i) {
			g.inspectQuest(i)
			return true
		}
		g.quest.route = village.QuestRoute(g.position, p)
		g.quest.journalOpen = false
		g.controller.CancelTarget()
		if len(g.quest.route) != 0 {
			g.quest.pending = i
		} else {
			g.quest.message = "FIND A CLEAR APPROACH\nWalk around the nearby scenery,\nthen tap the sign or chest again."
			g.quest.journalOpen = true
		}
		g.updateQuest()
		return true
	}
	return false
}

func (g *Game) advanceQuestWalk() {
	tolerance := 4.0
	if g.quest.pending < 0 {
		// Smoothed engine routes may pass close to an obstacle corner. Reach
		// their waypoints before turning; quest-grid routes retain their margin.
		tolerance = .5
	}
	g.quest.route = village.AdvanceRoute(g.quest.route, g.position, tolerance)
	if len(g.quest.route) > 0 {
		g.controller.SetTarget(controllers.Vec2{X: g.quest.route[0].X, Y: g.quest.route[0].Y})
	}
}
