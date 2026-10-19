package main

import (
	"math"

	"example.com/willowbrook/.karty/engine"
	"example.com/willowbrook/.karty/processed"
	"example.com/willowbrook/internal/village"
	"github.com/karty-game/controllers"
	"github.com/karty-game/tiled"
)

const hudLayer = 1000000000

type Game struct {
	engine.Game
	request      engine.RequestID
	level        engine.LevelHandle
	view         tiledView
	loaded       bool
	camera       engine.Camera2DSettings
	focus        village.Point
	hero         *engine.EntitySprite2D
	shadow       *engine.EntitySprite2D
	label        *engine.EntityText2D
	position     village.Point
	controller   *controllers.Planar
	facing, pose int
	phase        float64
	quest        treasureQuest
	navigation   village.NavigationRequest
}

//go:wasmexport karty_register
func main() {
	g := &Game{pose: -1}
	engine.Run(engine.Hooks{
		OnStart: g.start, OnLevelReady: g.ready, OnUpdate: g.update,
		OnKeyDown: func(k engine.Key) { g.key(k, true) }, OnKeyUp: func(k engine.Key) { g.key(k, false) },
		OnInputReset: g.resetInput, OnPointer: g.pointer,
		OnPathResult:  g.pathResult,
		OnLevelFailed: func(e engine.LevelFailed) { g.label.SetText("Village loading failed: " + e.Diagnostic) },
	})
}
func (g *Game) start() {
	compiledMap, ok := processed.Read("tiled.compile", "internal/village/village.bin")
	if !ok {
		panic("compiled Tiled map is missing")
	}
	levelMap, mapErr := tiled.Decode(compiledMap)
	if mapErr != nil {
		panic(mapErr)
	}
	village.Configure(levelMap)
	g.position = village.Spawn

	options := controllers.DefaultTopDown2DOptions()
	options.Speed, options.RunSpeed = village.WalkSpeed(), village.RunSpeed()
	options.FollowOffset = controllers.Vec2{Y: village.Level.Properties.Number("cameraOffsetY", -110)}
	options.Zoom = village.Zoom()
	var err error
	g.controller, err = controllers.NewTopDown2D(options)
	if err != nil {
		panic(err)
	}

	plate := g.NewVector2D(engine.Rectangle(184, 38), engine.NewVec2(548, 12))
	plate.SetFill(engine.RGBA(245, 235, 197, 235))
	plate.SetLayer(hudLayer)
	g.label = g.NewText2D("LOADING VILLAGE", engine.NewVec2(560, 24))
	g.label.SetSize(16)
	g.label.SetColor(engine.RGBA(48, 66, 47, 255))
	g.label.SetLayer(hudLayer + 1)
	g.request = g.RequestLevel("levels.village")
}
func (g *Game) ready(e engine.LevelReady) {
	if e.RequestID != g.request || g.level != 0 {
		return
	}
	g.level = e.Handle
	g.configureNavigation()
	compiledRender, ok := processed.Read("tiled.compile", "internal/village/village.render.bin")
	if !ok {
		panic("compiled Tiled render map is missing")
	}
	renderMap, err := tiled.Decode(compiledRender)
	if err != nil {
		panic(err)
	}
	g.view = tiledView{game: &g.Game, level: e.Handle, levelMap: renderMap}
	g.camera = engine.Camera2DSettings{Zoom: float32(village.Zoom()), WorldMinLayer: 0, WorldMaxLayer: 100000000}
	g.focus = village.CameraFocus(g.position)
	g.follow(true)
}
func (g *Game) resetInput() {
	g.controller.Reset()
	g.quest.interactHeld = false
	g.cancelQuestWalk()
}
func (g *Game) key(k engine.Key, down bool) {
	var control controllers.Control
	movement := true
	switch k {
	case engine.KeyW:
		control = controllers.Forward
	case engine.KeyArrowUp:
		control = controllers.ForwardAlt
	case engine.KeyS:
		control = controllers.Backward
	case engine.KeyArrowDown:
		control = controllers.BackwardAlt
	case engine.KeyA:
		control = controllers.Left
	case engine.KeyLeft:
		control = controllers.LeftAlt
	case engine.KeyD:
		control = controllers.Right
	case engine.KeyRight:
		control = controllers.RightAlt
	case engine.KeyShiftLeft:
		g.controller.Key(controllers.RunLeft, down)
		return
	case engine.KeyShiftRight:
		g.controller.Key(controllers.RunRight, down)
		return
	case engine.KeyEscape:
		if down {
			if g.loaded {
				g.closeJournal()
			}
			g.resetInput()
		}
		return
	case engine.KeyE, engine.KeySpace:
		movement = false
		if down && !g.quest.interactHeld && g.loaded {
			g.inspectNearby()
		}
		g.quest.interactHeld = down
	case engine.KeyR:
		movement = false
		if down && g.loaded {
			g.position = village.Spawn
			g.phase = 0
			g.resetInput()
			g.restartQuest()
			g.follow(true)
			g.paint(false)
		}
	default:
		movement = false
	}
	if movement {
		g.controller.Key(control, down)
		if down {
			g.controller.CancelTarget()
			g.cancelQuestWalk()
		}
	}
}
func (g *Game) pointer(p engine.PointerInput) {
	if p.Phase == engine.PointerCancel {
		g.resetInput()
		return
	}
	if p.Phase == engine.PointerDown && g.loaded {
		g.controller.CancelTarget()
		g.cancelQuestWalk()
		if g.selectQuest(float64(p.X), float64(p.Y)) {
			return
		}
		q := g.camera.ScreenToWorld(engine.NewVec2(float32(p.X), float32(p.Y)), 1280, 720)
		g.requestPath(village.Point{X: float64(q.X), Y: float64(q.Y)})
	}
}
func (g *Game) update(engine.Frame) {
	if g.level == 0 {
		return
	}
	if !g.loaded {
		if !g.view.build(g.camera) {
			return
		}
		g.loaded = true
		g.paint(false)
		g.startQuest()
		g.label.SetText("WILLOWBROOK")
		return
	}
	g.view.build(g.camera)
	g.advanceQuestWalk()
	delta := g.controller.Step(controllers.Vec2{X: g.position.X, Y: g.position.Y})
	dx, dy := delta.X, delta.Y
	next := village.Move(g.position, village.Point{X: dx, Y: dy})
	distance := math.Hypot(next.X-g.position.X, next.Y-g.position.Y)
	if distance > .00001 {
		if math.Abs(dx) > math.Abs(dy) {
			if dx < 0 {
				g.facing = 1
			} else {
				g.facing = 2
			}
		} else if dy < 0 {
			g.facing = 3
		} else {
			g.facing = 0
		}
		g.phase = village.AdvanceWalkPhase(g.phase, distance, village.HeroStride(g.controller.Running()))
	} else if g.controller.HasTarget() && (dx != 0 || dy != 0) {
		g.controller.CancelTarget()
		g.cancelQuestWalk()
	}
	g.position = next
	g.updateQuest()
	g.follow(false)
	g.paint(distance > .00001)
}
func (g *Game) follow(snap bool) {
	focus := g.controller.Follow(controllers.Vec2{X: g.focus.X, Y: g.focus.Y}, controllers.Vec2{X: g.position.X, Y: g.position.Y}, snap)
	next := village.Point{X: focus.X, Y: focus.Y}
	if !snap && math.Hypot(next.X-g.focus.X, next.Y-g.focus.Y) < .001 {
		return
	}
	g.focus = next
	g.camera.Position = engine.NewVec2(float32(next.X), float32(next.Y))
	g.SetCamera2D(g.camera)
}
func (g *Game) paint(walking bool) {
	if !walking {
		g.phase = 0
	}
	frame := village.WalkFrame(g.phase, walking)
	pose := g.facing*3 + frame
	w, h := village.Level.Properties.Number("heroWidth", 40), village.Level.Properties.Number("heroHeight", 60)
	shadowAsset := textureIDs["shadow-soft"]
	if g.shadow == nil {
		g.shadow = g.footSprite(shadowAsset, g.position, w*.55, 10)
	}
	dims := textureSizes[shadowAsset]
	g.shadow.Set(
		engine.NewVec2(float32(g.position.X+2), float32(g.position.Y)),
		engine.NewVec2(float32(w*.55)/float32(dims[0]), 10/float32(dims[1])),
		0,
	)
	g.shadow.SetLayer(g.mapLayer(village.ActorLayer, g.position.Y) - 1)
	if g.hero == nil {
		g.hero = g.footSprite(heroTextures[pose], g.position, w, h)
	} else {
		g.hero.Set(engine.NewVec2(float32(g.position.X), float32(g.position.Y-h/2)), engine.NewVec2(float32(w/32), float32(h/48)), 0)
		g.hero.SetLayer(g.mapLayer(village.ActorLayer, g.position.Y))
	}
	if pose != g.pose {
		g.hero.UpdateLevelAsset(engine.LevelTexture(g.level, heroTextures[pose]))
		g.pose = pose
	}
}
