package main

import (
	"math"

	"example.com/willowbrook/.karty/engine"
	"example.com/willowbrook/internal/village"
	"github.com/karty-game/controllers"
)

// The trail uses existing village landmarks rather than a line of markers.
var questPlaces = village.QuestStations

var questClues = village.QuestMessages

type questJournalLayout struct {
	top, height, actionOffset, actionHeight float32
}

func journalLayout(expanded bool) questJournalLayout {
	if expanded {
		return questJournalLayout{top: 584, height: 112, actionOffset: 72, actionHeight: 34}
	}
	return questJournalLayout{top: 648, height: 50, actionOffset: 24, actionHeight: 22}
}

type treasureQuest struct {
	stage         int
	pending       int
	route         []village.Point
	interactHeld  bool
	label, action *engine.EntityText2D
	plate         *engine.EntityVector2D
	actionPlate   *engine.EntityVector2D
	close         *engine.EntityText2D
	prompt        *engine.EntityText2D
	promptPlate   *engine.EntityVector2D
	cache         *engine.EntitySprite2D
	lastText      string
	lastNearby    int
	journalOpen   bool
	lastOpen      bool
	message       string
	lastLabel     string
}

func (g *Game) startQuest() {
	g.quest.pending = -1
	g.quest.lastNearby = -2
	g.quest.message = questClues[0]
	layout := journalLayout(false)
	g.quest.plate = g.NewVector2D(engine.Rectangle(288, layout.height), engine.NewVec2(496, layout.top))
	g.quest.plate.SetFill(engine.RGBA(245, 235, 197, 230))
	g.quest.plate.SetLayer(hudLayer)
	g.quest.label = g.NewText2D("", engine.NewVec2(508, layout.top+10))
	g.quest.label.SetSize(12)
	g.quest.label.SetColor(engine.RGBA(48, 66, 47, 255))
	g.quest.label.SetLayer(hudLayer + 1)
	g.quest.close = g.NewText2D("", engine.NewVec2(744, layout.top+4))
	g.quest.close.SetSize(11)
	g.quest.close.SetColor(engine.RGBA(104, 70, 36, 255))
	g.quest.close.SetLayer(hudLayer + 2)
	g.quest.actionPlate = g.NewVector2D(engine.Rectangle(272, layout.actionHeight), engine.NewVec2(504, layout.top+layout.actionOffset))
	g.quest.actionPlate.SetLayer(hudLayer + 1)
	g.quest.action = g.NewText2D("", engine.NewVec2(510, layout.top+layout.actionOffset+4))
	g.quest.action.SetSize(12)
	g.quest.action.SetColor(engine.RGBA(104, 70, 36, 255))
	g.quest.action.SetLayer(hudLayer + 2)
	g.quest.promptPlate = g.NewVector2D(engine.Rectangle(66, 18), engine.NewVec2(0, 0))
	g.quest.promptPlate.SetFill(engine.ColorTransparent)
	g.quest.prompt = g.NewText2D("", engine.NewVec2(0, 0))
	g.quest.prompt.SetSize(11)
	g.quest.prompt.SetColor(engine.RGBA(48, 66, 47, 255))
	for i, p := range questPlaces {
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
	g.quest.message = questClues[0]
	g.quest.journalOpen = false
	g.quest.cache.UpdateLevelAsset(engine.LevelTexture(g.level, treasureClosedTexture))
	g.updateQuest()
}

func (g *Game) cancelQuestWalk() {
	g.quest.route = nil
	g.quest.pending = -1
}

func (g *Game) nearQuest(i int) bool {
	return village.CanInteract(g.position, questPlaces[i])
}

func (g *Game) nearbyQuest() int {
	nearest, distance := -1, math.Inf(1)
	for i := range questPlaces {
		p := questPlaces[i]
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
		g.quest.message = questClues[g.quest.stage]
		if g.quest.stage == 3 {
			g.quest.cache.UpdateLevelAsset(engine.LevelTexture(g.level, treasureOpenTexture))
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
		g.quest.message = questClues[i+1]
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
	if label != g.quest.lastLabel {
		g.quest.label.SetText(label)
		g.quest.lastLabel = label
	}
	if g.quest.journalOpen != g.quest.lastOpen {
		layout := journalLayout(g.quest.journalOpen)
		g.quest.plate.SetShape(engine.Rectangle(288, layout.height))
		g.quest.plate.Set(engine.NewVec2(496, layout.top), engine.NewVec2(1, 1), 0)
		g.quest.label.Set(engine.NewVec2(508, layout.top+10), engine.NewVec2(1, 1), 0)
		g.quest.close.Set(engine.NewVec2(744, layout.top+4), engine.NewVec2(1, 1), 0)
		g.quest.actionPlate.SetShape(engine.Rectangle(272, layout.actionHeight))
		g.quest.actionPlate.Set(engine.NewVec2(504, layout.top+layout.actionOffset), engine.NewVec2(1, 1), 0)
		g.quest.action.Set(engine.NewVec2(510, layout.top+layout.actionOffset+4), engine.NewVec2(1, 1), 0)
		if g.quest.journalOpen {
			g.quest.close.SetText("CLOSE")
		} else {
			g.quest.close.SetText("")
		}
		g.quest.lastOpen = g.quest.journalOpen
	}
	if nearby != g.quest.lastNearby {
		fill := engine.RGBA(245, 235, 197, 0)
		if nearby >= 0 {
			fill = engine.RGBA(220, 201, 147, 255)
			p := questPlaces[nearby]
			_, height := village.QuestVisualSize(nearby)
			g.quest.promptPlate.Set(engine.NewVec2(float32(p.X-33), float32(p.Y-height-22)), engine.NewVec2(1, 1), 0)
			g.quest.promptPlate.SetLayer(g.mapLayer(village.ActorLayer, p.Y) + 3)
			g.quest.prompt.Set(engine.NewVec2(float32(p.X-29), float32(p.Y-height-19)), engine.NewVec2(1, 1), 0)
			g.quest.prompt.SetLayer(g.mapLayer(village.ActorLayer, p.Y) + 4)
		}
		g.quest.promptPlate.SetFill(fill)
		g.quest.actionPlate.SetFill(fill)
		g.quest.lastNearby = nearby
	}
	prompt := ""
	if nearby >= 0 {
		prompt = "READ [E]"
		if nearby == 2 {
			prompt = "OPEN [E]"
			if g.quest.stage == 3 {
				prompt = "VIEW [E]"
			}
		}
	}
	if g.quest.prompt.Text != prompt {
		g.quest.prompt.SetText(prompt)
	}
	if text != g.quest.lastText {
		g.quest.action.SetText(text)
		g.quest.lastText = text
	}
}

// Test the projected sprite, including its height, so tapping a note's picture
// works on the upper terrace as well as tapping its ground contact.
func (g *Game) selectQuest(x, y float64) bool {
	// The highlighted footer is a generous touch action, inside the portrait
	// safe area. Clicking any part of the journal must not walk through it.
	layout := journalLayout(g.quest.journalOpen)
	if x >= 496 && x <= 784 && y >= float64(layout.top) && y <= float64(layout.top+layout.height) {
		if g.quest.journalOpen {
			g.closeJournal()
		} else if y >= float64(layout.top+layout.actionOffset) && g.nearbyQuest() >= 0 {
			g.inspectNearby()
		} else {
			g.quest.journalOpen = true
			g.updateQuest()
		}
		return true
	}
	for i, p := range questPlaces {
		screen := g.camera.WorldToScreen(engine.NewVec2(float32(p.X), float32(p.Y)), 1280, 720)
		sx, sy, zoom := float64(screen.X), float64(screen.Y), float64(g.camera.Zoom)
		_, height := village.QuestVisualSize(i)
		promptHit := g.nearQuest(i) && math.Abs(x-sx) <= 33*zoom && y >= sy-(height+22)*zoom && y <= sy-(height+4)*zoom
		if !promptHit && !village.QuestHit(i, x, y, sx, sy, zoom) {
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
	for len(g.quest.route) > 0 && math.Hypot(g.quest.route[0].X-g.position.X, g.quest.route[0].Y-g.position.Y) < 4 {
		g.quest.route = g.quest.route[1:]
	}
	if len(g.quest.route) > 0 {
		g.controller.SetTarget(controllers.Vec2{X: g.quest.route[0].X, Y: g.quest.route[0].Y})
	}
}
