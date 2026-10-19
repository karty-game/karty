package main

import (
	"math"
	"strconv"

	"example.com/vigil/.karty/engine"
	"example.com/vigil/.karty/ui"
	"example.com/vigil/internal/dungeon"
	"github.com/karty-game/controllers"
)

type Game struct {
	engine.Game
	request              engine.RequestID
	level                engine.LevelHandle
	camera               *engine.EntityCamera3D
	hero, shadow         *engine.EntityWorldSprite
	heroTransform        engine.Transform3D
	halo                 *engine.EntityLight3D
	hud                  *engine.UI
	watch                engine.TransformSubscription
	nav                  dungeon.Navigation
	focus                dungeon.Point
	aim                  dungeon.Point
	height               float64
	controller           *controllers.Planar
	held, help           bool
	helpPinned           bool
	spellReady           bool
	resetPending         bool
	fireHeld             bool
	fireCooldown         uint8
	fireballs            [4]fireball
	nextFireball         int
	audio                vigilAudio
	pointer              uint32
	pointerX, pointerY   float32
	routePending         bool
	facing, lastTexture  int
	walk                 dungeon.WalkCycle
	lastPosition         dungeon.Point
	vision               dungeon.Vision
	fog                  [dungeon.MaxFog]*engine.EntitySprite2D
	fogBounds            [dungeon.MaxFog]dungeon.Box
	fogUsed, fogReady    int
	curtain              *engine.EntityVector2D
	frame                uint64
	shrines              [3]*engine.EntityWorldSprite
	shrineLights         [3]*engine.EntityLight3D
	lit                  [3]bool
	status, area, prompt string
}

//go:wasmexport karty_register
func main() {
	g := &Game{height: 13.2, help: true, lastTexture: -1}
	engine.Run(engine.Hooks{
		OnStart: g.start, OnUpdate: g.update, OnStop: g.stop,
		OnLevelReady:  g.ready,
		OnPathResult:  g.pathResult,
		OnLevelFailed: func(e engine.LevelFailed) { g.status = "Unable to enter the crypt: " + e.Diagnostic; g.refresh() },
		OnKeyDown:     func(k engine.Key) { g.key(k, true) }, OnKeyUp: func(k engine.Key) { g.key(k, false) },
		OnPointer: g.point, OnInputReset: g.resetInput,
	})
}

func (g *Game) start() {
	options := controllers.DefaultIsometricOptions()
	options.Speed, options.RunSpeed = dungeon.MoveSpeed, dungeon.HastenSpeed
	options.FollowOffset = controllers.Vec2{X: 1.6, Y: 1.6}
	options.Zoom = g.height
	var err error
	g.controller, err = controllers.NewIsometric(options)
	if err != nil {
		panic(err)
	}

	g.status = "Descending beneath the abbey..."
	g.hud = ui.Vigil(ui.VigilProps{
		Area: func() string { return g.area }, Status: func() string { return g.status },
		Progress: func() string { return strconv.Itoa(g.progress()) + " / 3" },
		Prompt:   func() string { return g.prompt }, Help: func() bool { return g.help },
		SpellReady: func() bool { return g.hero != nil && !g.resetPending && g.fireCooldown == 0 },
		ToggleHelp: g.toggleHelp, Interact: g.interact,
		Reset: g.restart, Fireball: g.throwFireball, ZoomIn: func() { g.zoom(.9) }, ZoomOut: func() { g.zoom(1.1) },
	})
	g.request = g.RequestLevel("levels.crypt")
}

func (g *Game) ready(e engine.LevelReady) {
	if e.RequestID != g.request || g.hero != nil {
		return
	}
	g.level = e.Handle
	if err := g.World(g.level).
		Configure(engine.WorldSettings{Mode: engine.WorldSpatial, Min: engine.Vec2{}, Max: engine.Vec2{X: dungeon.Columns * dungeon.Cell, Y: dungeon.Rows * dungeon.Cell}, CellSize: dungeon.Cell}); err != nil {
		panic(err)
	}
	g.lastPosition = dungeon.Spawn
	g.curtain = g.NewVector2D(engine.Rectangle(dungeon.FogRight-dungeon.FogLeft, 720), engine.NewVec2(dungeon.FogLeft, 0))
	g.curtain.SetFill(engine.ColorBlack)
	g.curtain.SetLayer(-8)
	g.focus = dungeon.Point{dungeon.Spawn.X + 1.6, dungeon.Spawn.Y + 1.6}
	g.camera = g.NewCamera3D(e.Handle, engine.CameraSettings{
		Projection: engine.CameraProjectionIsometric, Output: engine.CameraOutputFull,
		Position: engine.NewVec3(float32(g.focus.X-18), float32(g.focus.Y-18), 14.696938),
		Yaw:      math.Pi / 4, Pitch: -math.Pi / 6, FOVY: 1.05, OrthoHeight: float32(g.height),
		Near: .05, Far: 100, ViewportWidth: 1280, ViewportHeight: 720,
	})
	g.camera.SetLayer(-10)
	g.facing = 4
	g.aim = dungeon.FacingDirection(g.facing)
	g.updateHeroPose(dungeon.WalkTextureCount+4, dungeon.Spawn)
	if !g.createMovementRoot() {
		return
	}
	g.createAudio()
	g.halo = g.NewPointLight(
		g.level,
		engine.NewVec3(float32(dungeon.Spawn.X), float32(dungeon.Spawn.Y), 1.63),
		engine.NewVec3(.58, .39, .2),
		3.8,
	)
	for i, p := range dungeon.Shrines {
		g.shrineLights[i] = g.NewPointLight(g.level, engine.NewVec3(float32(p.X), float32(p.Y), 1.6), engine.NewVec3(1, .58, .22), 5)
		g.shrineLights[i].SetEnabled(false)
	}
	g.status = "Rekindle the three vigil lights."
	g.refresh()
}

func (g *Game) position() dungeon.Point {
	if g.resetPending {
		return dungeon.Spawn
	}
	return dungeon.Point{float64(g.shadow.Position.X), float64(g.shadow.Position.Y)}
}

func (g *Game) update(f engine.Frame) {
	if g.hero == nil {
		return
	}
	g.frame = f.Number
	g.updateFireballs()
	ready := g.hero != nil && !g.resetPending && g.fireCooldown == 0
	if ready != g.spellReady {
		g.spellReady = ready
		g.refresh()
	}
	if g.help && !g.helpPinned && g.frame >= 600 {
		g.help = false
		g.refresh()
	}
	p := g.position()
	// Ordered event dispatch can deliver a pre-reset accepted shadow pose after
	// the reset input. Wait for the teleport acknowledgement before moving again.
	if g.resetPending {
		g.updateHeroPose(dungeon.WalkTextureCount+4, dungeon.Spawn)
		g.updateVision(dungeon.Spawn)
		return
	}
	// Consume the latest drag once per update. Input events can arrive on any
	// tick; gating PointerMove by a tick modulus discarded the final target.
	if g.routePending {
		g.routePending = false
		g.route(g.pointerX, g.pointerY)
	}
	travel := dungeon.Distance(g.lastPosition, p)
	g.lastPosition = p
	g.updateAudio(travel, p)
	intent := g.controller.Step(controllers.Vec2{X: p.X, Y: p.Y})
	delta := dungeon.Point{X: intent.X, Y: intent.Y}
	if intent.X != 0 || intent.Y != 0 {
		g.nav.Cancel()
		g.held = false
	} else if g.nav.Next < g.nav.Count {
		target := g.nav.Step(p, g.controller.Speed()/60)
		delta = dungeon.Point{target.X - p.X, target.Y - p.Y}
	}
	next := dungeon.Move(p, delta)
	moved := dungeon.Distance(p, next) > .00001
	if moved {
		g.aim = dungeon.AimDirection(g.aim, dungeon.Point{X: next.X - p.X, Y: next.Y - p.Y})
		g.facing = dungeon.Facing(g.aim.X, g.aim.Y)
		s := g.shadow.Transform3D
		s.Position.X = float32(next.X)
		s.Position.Y = float32(next.Y)
		if err := g.shadow.Ref().SetPose(s); err != nil {
			panic(err)
		}
		g.halo.SetPose(engine.NewVec3(float32(next.X), float32(next.Y), 1.63), 0, 0)
	}
	g.walk.Advance(travel)
	phase := g.walk.PhaseAfter(dungeon.Distance(p, next))
	frame := int(phase)
	pose := dungeon.WalkTextureCount + g.facing
	if moved || travel >= .00001 {
		pose = g.facing*dungeon.WalkFrames + frame
	}
	g.vision.HeroFrame = pose
	g.vision.HeroOffset = dungeon.Point{next.X - p.X, next.Y - p.Y}
	g.updateHeroPose(pose, next)
	// Smooth the camera from accepted positions, keeping a little space ahead.
	focus := g.controller.Follow(controllers.Vec2{X: g.focus.X, Y: g.focus.Y}, controllers.Vec2{X: p.X, Y: p.Y}, false)
	changed := math.Abs(focus.X-g.focus.X)+math.Abs(focus.Y-g.focus.Y) > .00001
	g.focus = dungeon.Point{X: focus.X, Y: focus.Y}
	if changed || moved {
		camera := g.controller.CameraPosition(focus)
		pose := engine.NewTransform3D(engine.NewVec3(float32(camera.X), float32(camera.Y), float32(camera.Z)))
		pose.Yaw, pose.Pitch = math.Pi/4, -math.Pi/6
		if err := g.camera.Ref().SetPose(pose); err != nil {
			panic(err)
		}
	}
	g.updateVision(p)
	area := "The Narthex"
	switch {
	case p.Y > 18:
		area = "Chapel of the Last Light"
	case p.X < 6:
		area = "Transept of Memory"
	case p.X > 18:
		area = "The Brothers' Rest"
	case p.Y > 8:
		area = "The Crossing"
	case p.Y > 4.5:
		area = "Nave of Ash"
	}
	prompt := ""
	if g.nearShrine() >= 0 {
		prompt = "E / SPACE  ·  Rekindle vigil light"
	}
	if area != g.area || prompt != g.prompt {
		g.area, g.prompt = area, prompt
		g.refresh()
	}
	// Only the three lit flames change texture, at authored flipbook cadence.
	if f.Number%7 == 0 {
		for i, lit := range g.lit {
			if lit {
				s := g.shrines[i].WorldSprite
				s.Asset = engine.LevelTexture(g.level, uint32(flameTexture+int(f.Number/7)%3))
				g.shrines[i].SetSprite(s)
			}
		}
	}
}

func (g *Game) key(k engine.Key, down bool) {
	var control controllers.Control
	movement := true
	switch k {
	case engine.KeyW:
		control = controllers.Forward
	case engine.KeyS:
		control = controllers.Backward
	case engine.KeyA:
		control = controllers.Left
	case engine.KeyD:
		control = controllers.Right
	case engine.KeyArrowUp:
		control = controllers.ForwardAlt
	case engine.KeyArrowDown:
		control = controllers.BackwardAlt
	case engine.KeyLeft:
		control = controllers.LeftAlt
	case engine.KeyRight:
		control = controllers.RightAlt
	case engine.KeyShiftLeft:
		g.controller.Key(controllers.RunLeft, down)
		return
	case engine.KeyShiftRight:
		g.controller.Key(controllers.RunRight, down)
		return
	case engine.KeyEscape:
		movement = false
		if down {
			g.resetInput()
		}
	case engine.KeyE, engine.KeySpace:
		movement = false
		if down {
			g.interact()
		}
	case engine.KeyH:
		movement = false
		if down {
			g.toggleHelp()
		}
	case engine.KeyF:
		movement = false
		if down && !g.fireHeld {
			g.throwFireball()
		}
		g.fireHeld = down
	case engine.KeyR:
		movement = false
		if down {
			g.restart()
		}
	case engine.KeyEqual:
		movement = false
		if down {
			g.zoom(.9)
		}
	case engine.KeyMinus:
		movement = false
		if down {
			g.zoom(1.1)
		}
	default:
		movement = false
	}
	if movement {
		g.controller.Key(control, down)
		if down {
			g.routePending = false
			g.nav.Cancel()
			g.held = false
		}
	}
}

func (g *Game) point(p engine.PointerInput) {
	if g.hero == nil {
		return
	}
	if p.Phase == engine.PointerCancel {
		g.resetInput()
		return
	}
	if p.Phase == engine.PointerDown && p.Buttons&1 != 0 {
		g.pointer = p.ID
		g.held = true
		g.pointerX, g.pointerY = p.X, p.Y
		g.routePending = true
	} else if p.Phase == engine.PointerUp && p.ID == g.pointer {
		g.held = false
		if g.routePending {
			g.pointerX, g.pointerY = p.X, p.Y
		}
	} else if p.Phase == engine.PointerMove && p.ID == g.pointer && g.held && p.Buttons&1 != 0 {
		g.pointerX, g.pointerY = p.X, p.Y
		g.routePending = true
	}
}

func (g *Game) route(x, y float32) {
	ground, ok := g.controller.ScreenToGround(
		controllers.Vec2{X: float64(x), Y: float64(y)},
		controllers.Vec2{X: g.focus.X, Y: g.focus.Y},
		1280,
		720,
	)
	if !ok {
		return
	}
	target := dungeon.Point{X: ground.X, Y: ground.Y}
	// Foreground masonry and the sight mask are visual occluders, not input
	// blockers. Let the navigator find a safe route around a wall even when
	// the clicked ground is currently hidden from the pilgrim.
	// Tapping a nearby light is the touch equivalent of E. Distant taps walk.
	for i, p := range dungeon.Shrines {
		if !g.lit[i] && dungeon.Distance(target, p) < 1 && dungeon.Distance(g.position(), p) < 1.65 {
			g.interact()
			return
		}
	}
	g.requestPath(target)
}

func (g *Game) nearShrine() int {
	if g.hero == nil {
		return -1
	}
	for i, p := range dungeon.Shrines {
		if !g.lit[i] && dungeon.Distance(g.position(), p) < 1.65 && dungeon.Visible(g.position(), p) {
			return i
		}
	}
	return -1
}
func (g *Game) interact() {
	i := g.nearShrine()
	if i < 0 {
		return
	}
	g.lit[i] = true
	p := dungeon.Shrines[i]
	t := engine.NewTransform3D(engine.NewVec3(float32(p.X), float32(p.Y), 1.205))
	g.shrines[i] = g.NewWorldSprite(engine.LevelTexture(g.level, flameTexture), t, engine.WorldSprite{
		Facing: engine.WorldSpriteFacingUpright, Alpha: engine.WorldSpriteAlphaBlend,
		Size: engine.NewVec2(.4, float32(.7/math.Cos(math.Pi/6))), Origin: engine.NewVec2(.5, 1),
	})
	g.shrineLights[i].SetEnabled(true)
	g.lightShrineAudio(i, p)
	g.status = []string{"A light for those we remember.", "A light for those who keep watch.", "A light against the coming night."}[i]
	if g.progress() == 3 {
		g.status = "The vigil is kept. Peace be with you."
	}
	g.refresh()
}
func (g *Game) progress() int {
	n := 0
	for _, lit := range g.lit {
		if lit {
			n++
		}
	}
	return n
}
func (g *Game) toggleHelp() {
	g.helpPinned = true
	g.help = !g.help
	g.refresh()
}

func (g *Game) refresh() {
	if g.hud != nil {
		g.hud.Invalidate()
	}
}
func (g *Game) resetInput() {
	g.routePending = false
	g.controller.Reset()
	g.held, g.fireHeld = false, false
	g.nav.Cancel()
}
func (g *Game) zoom(factor float64) {
	g.height = g.controller.ZoomBy(factor)
	if g.camera != nil {
		s := g.camera.Settings
		s.OrthoHeight = float32(g.height)
		g.camera.SetSettings(s)
	}
}
func (g *Game) restart() {
	if g.hero == nil {
		return
	}
	g.resetInput()
	g.clearFireballs()
	g.resetAudio()
	g.resetPending = true
	// EntityRef.SetPose is a swept movement proposal. A reset is a fresh spawn,
	// which must not sweep through furniture on the route back to the entrance.
	g.watch.Stop()
	g.hero.Destroy()
	g.shadow.Destroy()
	g.hero, g.shadow = nil, nil
	g.halo.SetPose(engine.NewVec3(float32(dungeon.Spawn.X), float32(dungeon.Spawn.Y), 1.63), 0, 0)
	g.walk, g.facing = dungeon.WalkCycle{}, 4
	g.aim = dungeon.FacingDirection(g.facing)
	g.vision.HeroFrame = dungeon.WalkTextureCount + 4
	g.vision.HeroOffset = dungeon.Point{}
	g.updateHeroPose(dungeon.WalkTextureCount+4, dungeon.Spawn)
	if !g.createMovementRoot() {
		return
	}
	g.bindAudioReceiver()
	g.lastPosition = dungeon.Spawn
	g.focus = dungeon.Point{dungeon.Spawn.X + 1.6, dungeon.Spawn.Y + 1.6}
	pose := engine.NewTransform3D(engine.NewVec3(float32(g.focus.X-18), float32(g.focus.Y-18), 14.696938))
	pose.Yaw, pose.Pitch = math.Pi/4, -math.Pi/6
	if err := g.camera.Ref().SetPose(pose); err != nil {
		panic(err)
	}
	for i := range g.lit {
		g.lit[i] = false
		if g.shrines[i] != nil {
			g.shrines[i].Destroy()
			g.shrines[i] = nil
		}
		g.shrineLights[i].SetEnabled(false)
	}
	g.status = "Rekindle the three vigil lights."
	g.refresh()
}
func (g *Game) stop() {
	g.resetInput()
	g.clearFireballs()
	g.stopAudio()
	g.watch.Stop()
	if g.hud != nil {
		g.hud.Close()
	}
}
