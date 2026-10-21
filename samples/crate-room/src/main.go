package main

import (
	"math"
	"strconv"

	"example.com/crate-room/.karty/config"
	"example.com/crate-room/.karty/engine"
	"example.com/crate-room/.karty/ui"
	"github.com/karty-game/controllers"
)

const width, height = float32(config.ResolutionWidth), float32(config.ResolutionHeight)
const crateCount = 80

// A centered control stays visible when the full-frame view crops its sides.
const shootX, shootY = width / 2, height * .8125

type Game struct {
	engine.Game
	camera                                             *engine.Camera3D
	cameraBody                                         *engine.Body
	crateBodies                                        [crateCount]*engine.Body
	crates                                             [crateCount]*engine.Solid3D
	level                                              engine.LevelHandle
	keys                                               [256]bool
	moveID, lookID, shootID                            uint32
	lastLookX, lastLookY                               float32
	controller                                         *controllers.FirstPerson
	mouseHeld, pendingShot, pendingBlast, pendingReset bool
	cooldown                                           uint8
	shots, blasts                                      uint64
	status                                             string
	panel                                              *engine.UI
	moveThumb                                          *engine.Shape2D
}

//go:wasmexport karty_register
func main() {
	game := &Game{}
	engine.Run(
		engine.Hooks{
			OnStart:       game.start,
			OnUpdate:      game.update,
			OnKeyDown:     func(key engine.Key) { game.key(key, true) },
			OnKeyUp:       func(key engine.Key) { game.key(key, false) },
			OnPointer:     game.pointer,
			OnInputReset:  game.resetInput,
			OnLevelReady:  game.ready,
			OnLevelFailed: func(event engine.LevelFailed) { game.setStatus("Level failed: " + event.Diagnostic) },
		},
	)
}
func (game *Game) start() {
	options := controllers.DefaultFirstPersonOptions()
	options.Width, options.Height = width, height
	options.Speed, options.RunMultiplier = 4.5, 7.0/4.5
	options.StickRadius, options.Deadzone = 65, 0
	options.KeyYawSpeed, options.KeyPitchSpeed = 2.1, 1.5
	options.MouseSensitivity, options.MinPitch, options.MaxPitch = .006, -1.2, 1.2
	var err error
	game.controller, err = controllers.NewFirstPerson(options)
	if err != nil {
		panic(err)
	}

	game.status = "Loading crate room..."
	game.panel = ui.Hud(ui.HudProps{
		Status: func() string { return game.status },
		Blast:  func() { game.pendingBlast = true },
		Reset:  func() { game.pendingReset = true },
	})
	// Gesture feedback stays in game coordinates; all copy is presentation UI.
	// SHOOT is a passive UI label so raw down/up events preserve held fire while
	// other fingers independently move and look. BLAST and RESET are UI buttons.
	game.pad(110, height-104)
	game.pad(width-110, height-104)
	game.moveThumb = game.NewShape2D(engine.Circle(25), engine.NewVec2(110, height-104))
	game.moveThumb.SetFill(engine.RGBA(233, 191, 113, 210))
	game.moveThumb.SetLayer(31)
	game.button(shootX-105, shootY-37.5, 210, 75)
	crossX := game.NewShape2D(engine.Line(engine.NewVec2(18, 0)), engine.NewVec2(width/2-9, height/2))
	crossX.SetStroke(engine.RGBA(255, 235, 184, 235), 2)
	crossX.SetLayer(35)
	crossY := game.NewShape2D(engine.Line(engine.NewVec2(0, 18)), engine.NewVec2(width/2, height/2-9))
	crossY.SetStroke(engine.RGBA(255, 235, 184, 235), 2)
	crossY.SetLayer(35)
	game.RequestLevel("levels.crate-room")
}
func (game *Game) pad(x, y float32) {
	base := game.NewShape2D(engine.Circle(86), engine.NewVec2(x, y))
	base.SetFill(engine.RGBA(12, 24, 35, 165))
	base.SetStroke(engine.RGBA(225, 184, 110, 190), 2)
	base.SetLayer(30)
}
func (game *Game) button(x, y, w, h float32) {
	base := game.NewShape2D(engine.Rectangle(w, h), engine.NewVec2(x, y))
	base.SetFill(engine.RGBA(19, 32, 43, 210))
	base.SetStroke(engine.RGBA(220, 180, 109, 220), 2)
	base.SetLayer(30)
}
func (game *Game) ready(event engine.LevelReady) {
	game.level = event.Handle
	if err := game.World(game.level).
		Configure(engine.WorldSettings{Mode: engine.WorldSpatial, Gravity: engine.Vec3{Z: -9.81}}); err != nil {
		panic(err)
	}
	game.camera = game.NewCamera3D(
		game.level,
		engine.Camera3DSettings{
			Projection:     engine.CameraProjectionPerspective,
			Output:         engine.CameraOutputFull,
			Position:       engine.NewVec3(.2, -10, 1.67),
			VerticalFOV:    config.CameraFOVY,
			OrthoHeight:    config.CameraOrthoHeight,
			NearClip:       config.CameraNear,
			FarClip:        config.CameraFar,
			ViewportWidth:  config.ResolutionWidth,
			ViewportHeight: config.ResolutionHeight,
		},
	)
	var err error
	game.cameraBody, err = game.World(game.level).AttachBody(game.camera.Ref(), playerBox())
	if err != nil {
		panic(err)
	}

	for i := range game.crates {
		game.crates[i] = game.NewSolid3D(game.level, crateTransform(i), engine.SolidRenderer{Prefab: "crate"})
		game.crateBodies[i], err = game.World(game.level).AttachBody(game.crates[i].Ref(), crateBox())
		if err != nil {
			panic(err)
		}

	}
	game.hud()
}
func crateTransform(i int) engine.Transform3D {
	return engine.NewTransform3D(engine.NewVec3(float32((i/4)%5-2)*3.5, -1+float32(i/20)*3.5, .51+float32(i%4)*1.015))
}
func crateBox() engine.BodySettings {
	return engine.BodySettings{
		Kind:          engine.BodyDynamic,
		Collider:      engine.BoxCollider{HalfExtents: engine.NewVec3(.5, .5, .5)},
		AllowRotation: true,
		Forces:        engine.BodyForceSettings{GravityScale: 1, Buoyancy: true},
		Mass:          16,
		Friction:      .5,
		Restitution:   .08,
	}
}
func playerBox() engine.BodySettings {
	return engine.BodySettings{
		Kind:          engine.BodyDynamic,
		Collider:      engine.BoxCollider{HalfExtents: engine.NewVec3(.3, .3, .9), Offset: engine.NewVec3(0, 0, -.75)},
		AllowRotation: false,
		Forces:        engine.BodyForceSettings{GravityScale: 1, Buoyancy: true},
		Mass:          80,
	}
}
func (game *Game) key(key engine.Key, down bool) {
	if int(key) < len(game.keys) {
		game.keys[key] = down
	}
	switch key {
	case engine.KeyW:
		game.controller.Key(controllers.Forward, down)
	case engine.KeyS:
		game.controller.Key(controllers.Backward, down)
	case engine.KeyA:
		game.controller.Key(controllers.Left, down)
	case engine.KeyD:
		game.controller.Key(controllers.Right, down)
	case engine.KeyLeft:
		game.controller.Key(controllers.LookLeft, down)
	case engine.KeyRight:
		game.controller.Key(controllers.LookRight, down)
	case engine.KeyArrowUp:
		game.controller.Key(controllers.LookUp, down)
	case engine.KeyArrowDown:
		game.controller.Key(controllers.LookDown, down)
	case engine.KeyShiftLeft:
		game.controller.Key(controllers.RunLeft, down)
	case engine.KeyShiftRight:
		game.controller.Key(controllers.RunRight, down)
	}
	if !down {
		return
	}
	switch key {
	case engine.KeySpace:
		game.pendingShot = true
	case engine.KeyB:
		game.pendingBlast = true
	case engine.KeyR:
		game.pendingReset = true
	case engine.KeyEscape:
		game.SetCursorCaptured(false)
		game.resetInput()
	}
}
func (game *Game) pointer(p engine.PointerInput) {
	if p.Kind == engine.PointerMouse {
		if p.Phase == engine.PointerMove && p.Captured {
			game.controller.Mouse(p.DeltaX, p.DeltaY)
		}
		if p.Phase == engine.PointerDown {
			if shootContains(p.X, p.Y) {
				game.mouseHeld, game.pendingShot = true, true
				return
			}
			game.SetCursorCaptured(true)
			if p.Buttons&2 != 0 {
				game.pendingBlast = true
			} else {
				game.mouseHeld = true
				game.pendingShot = true
			}
		}
		if p.Phase == engine.PointerUp || p.Phase == engine.PointerCancel {
			game.mouseHeld = false
		}
		return
	}
	if p.Kind != engine.PointerTouch {
		return
	}
	switch p.Phase {
	case engine.PointerDown:
		if shootContains(p.X, p.Y) {
			game.shootID = p.ID
			game.pendingShot = true
			return
		}
		if p.X < width/2 && game.moveID == 0 {
			game.moveID = p.ID
			game.controller.BeginTouch(0, p.ID, 110, height-104)
			game.move(p.X, p.Y)
		} else if p.X >= width/2 && game.lookID == 0 {
			game.lookID = p.ID
			game.lastLookX, game.lastLookY = p.X, p.Y
		}
	case engine.PointerMove:
		if p.ID == game.moveID {
			game.move(p.X, p.Y)
		}
		if p.ID == game.lookID {
			game.controller.Mouse(p.X-game.lastLookX, p.Y-game.lastLookY)
			game.lastLookX, game.lastLookY = p.X, p.Y
		}
	case engine.PointerUp, engine.PointerCancel:
		if p.ID == game.moveID {
			game.moveID = 0
			game.controller.EndTouch(p.ID)
			game.moveThumb.SetTransform(engine.NewVec2(110, height-104), engine.NewVec2(1, 1), 0)
		}
		if p.ID == game.lookID {
			game.lookID = 0
		}
		if p.ID == game.shootID {
			game.shootID = 0
		}
	}
}
func shootContains(x, y float32) bool {
	return x > shootX-125 && x < shootX+125 && y > shootY-57.5 && y < shootY+57.5
}
func (game *Game) move(x, y float32) {
	game.controller.MoveTouch(game.moveID, x, y)
	stick := game.controller.Sticks[0]
	game.moveThumb.SetTransform(engine.NewVec2(110+stick.DX, height-104+stick.DY), engine.NewVec2(1, 1), 0)
}
func (game *Game) resetInput() {
	game.keys = [256]bool{}
	game.moveID, game.lookID, game.shootID = 0, 0, 0
	game.controller.Reset()
	game.mouseHeld = false
	game.pendingShot, game.pendingBlast = false, false
	if game.moveThumb != nil {
		game.moveThumb.SetTransform(engine.NewVec2(110, height-104), engine.NewVec2(1, 1), 0)
	}
}
func (game *Game) update(engine.Frame) {
	if game.camera == nil {
		return
	}
	if game.pendingReset {
		game.pendingReset = false
		game.reset()
		return
	}
	settings := game.camera.Settings()
	motion := game.controller.Step(settings.Yaw, settings.Pitch)
	if motion.Yaw != settings.Yaw || motion.Pitch != settings.Pitch {
		settings.Yaw, settings.Pitch = motion.Yaw, motion.Pitch
		game.camera.SetSettings(settings)
	}
	if err := game.cameraBody.SetHorizontalVelocity(engine.NewVec2(motion.X*60, motion.Y*60)); err != nil {
		panic(err)
	}
	if game.cooldown > 0 {
		game.cooldown--
	}
	if (game.pendingShot || game.mouseHeld || game.shootID != 0 || game.keys[engine.KeySpace]) && game.cooldown == 0 {
		game.fire(false)
		game.cooldown = 8
		game.pendingShot = false
	}
	if game.pendingBlast {
		game.pendingBlast = false
		game.fire(true)
	}
}
func (game *Game) fire(blast bool) {
	settings := game.camera.Settings()
	sy, cy := math.Sincos(float64(settings.Yaw))
	sp, cp := math.Sincos(float64(settings.Pitch))
	ray := engine.RayImpulse{
		Origin:    settings.Position,
		Direction: engine.NewVec3(float32(sy*cp), float32(cy*cp), float32(sp)),
		Distance:  60,
		Impulse:   95,
		Exclude:   game.camera.Ref(),
	}
	if blast {
		ray.Impulse = 260
		ray.ExplosionRadius = 5
		game.blasts++
	} else {
		game.shots++
	}
	if err := game.World(game.level).ApplyRayImpulse(ray); err != nil {
		panic(err)
	}
	game.hud()
}
func (game *Game) reset() {
	game.resetInput()
	game.shots, game.blasts = 0, 0
	game.pendingShot, game.pendingBlast = false, false
	if err := game.cameraBody.Detach(); err != nil {
		panic(err)
	}
	settings := game.camera.Settings()
	settings.Position = engine.NewVec3(.2, -10, 1.67)
	settings.Yaw, settings.Pitch = 0, 0
	game.camera.SetSettings(settings)
	var err error
	game.cameraBody, err = game.World(game.level).AttachBody(game.camera.Ref(), playerBox())
	if err != nil {
		panic(err)
	}
	for i, crate := range game.crates {
		if err := game.crateBodies[i].Detach(); err != nil {
			panic(err)
		}
		if err := crate.Ref().SetPose(crateTransform(i)); err != nil {
			panic(err)
		}
		game.crateBodies[i], err = game.World(game.level).AttachBody(crate.Ref(), crateBox())
		if err != nil {
			panic(err)
		}
	}
	game.hud()
}
func (game *Game) hud() {
	game.setStatus(
		"CRATE ROOM · 80 boxes · Shots " + strconv.FormatUint(game.shots, 10) + " · Blasts " + strconv.FormatUint(game.blasts, 10),
	)
}

func (game *Game) setStatus(value string) {
	game.status = value
	if game.panel != nil {
		game.panel.Invalidate()
	}
}
