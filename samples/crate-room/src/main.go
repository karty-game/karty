package main

import (
	"math"
	"strconv"

	"example.com/crate-room/.karty/config"
	"example.com/crate-room/.karty/engine"
)

const width, height = float32(config.ResolutionWidth), float32(config.ResolutionHeight)
const crateCount = 80

type Game struct {
	engine.Game
	camera                                             *engine.EntityCamera3D
	crates                                             [crateCount]*engine.EntityWorldSolid
	level                                              engine.LevelHandle
	keys                                               [256]bool
	moveID, lookID, shootID                            uint32
	moveX, moveY, lastLookX, lastLookY                 float32
	lookX, lookY                                       float32
	mouseHeld, pendingShot, pendingBlast, pendingReset bool
	cooldown                                           uint8
	shots, blasts                                      uint64
	status                                             *engine.EntityText2D
	moveThumb                                          *engine.EntityVector2D
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
			OnLevelFailed: func(event engine.LevelFailed) { game.status.SetText("Level failed: " + event.Diagnostic) },
		},
	)
}
func (game *Game) start() {
	game.status = game.NewText2D("Loading crate room...", engine.NewVec2D(22, 22))
	game.status.SetSize(19)
	game.status.SetColor(engine.RGBA(237, 221, 177, 255))
	game.status.SetLayer(30)
	help := game.NewText2D("WASD / drag MOVE · Mouse / drag LOOK · Shoot · B blast · R reset", engine.NewVec2D(22, 52))
	help.SetSize(14)
	help.SetLayer(35)
	// Controls are cartridge entities and consume the public multi-touch input.
	game.pad(110, height-104, "MOVE")
	game.pad(width-110, height-104, "LOOK")
	game.moveThumb = game.NewVector2D(engine.Circle(25), engine.NewVec2D(110, height-104))
	game.moveThumb.SetFill(engine.RGBA(233, 191, 113, 210))
	game.moveThumb.SetLayer(31)
	game.button(width-252, height-220, 210, 75, "SHOOT")
	game.button(width-252, height-310, 210, 75, "BLAST")
	game.button(width-170, 20, 150, 55, "RESET")
	crossX := game.NewVector2D(engine.Line(engine.NewVec2D(18, 0)), engine.NewVec2D(width/2-9, height/2))
	crossX.SetStroke(engine.RGBA(255, 235, 184, 235), 2)
	crossX.SetLayer(35)
	crossY := game.NewVector2D(engine.Line(engine.NewVec2D(0, 18)), engine.NewVec2D(width/2, height/2-9))
	crossY.SetStroke(engine.RGBA(255, 235, 184, 235), 2)
	crossY.SetLayer(35)
	game.RequestLevel("levels.crate-room")
}
func (game *Game) pad(x, y float32, label string) {
	base := game.NewVector2D(engine.Circle(86), engine.NewVec2D(x, y))
	base.SetFill(engine.RGBA(12, 24, 35, 165))
	base.SetStroke(engine.RGBA(225, 184, 110, 190), 2)
	base.SetLayer(30)
	text := game.NewText2D(label, engine.NewVec2D(x-30, y-10))
	text.SetSize(18)
	text.SetLayer(32)
}
func (game *Game) button(x, y, w, h float32, label string) {
	base := game.NewVector2D(engine.Rectangle(w, h), engine.NewVec2D(x, y))
	base.SetFill(engine.RGBA(19, 32, 43, 210))
	base.SetStroke(engine.RGBA(220, 180, 109, 220), 2)
	base.SetLayer(30)
	text := game.NewText2D(label, engine.NewVec2D(x+20, y+20))
	text.SetSize(22)
	text.SetLayer(31)
}
func (game *Game) ready(event engine.LevelReady) {
	game.level = event.Handle
	game.camera = game.NewCamera3D(
		game.level,
		engine.CameraSettings{
			Projection:     engine.CameraProjectionPerspective,
			Output:         engine.CameraOutputFull,
			Position:       engine.NewVec3D(.2, -10, 1.67),
			FOVY:           config.CameraFOVY,
			OrthoHeight:    config.CameraOrthoHeight,
			Near:           config.CameraNear,
			Far:            config.CameraFar,
			ViewportWidth:  config.ResolutionWidth,
			ViewportHeight: config.ResolutionHeight,
		},
	)
	game.SetPhysicsBox(game.camera, playerBox())
	game.WatchTransform(game.camera, nil)
	for i := range game.crates {
		game.crates[i] = game.NewWorldSolid(game.level, crateTransform(i), engine.SolidRenderer{Prefab: "crate"})
		game.SetPhysicsBox(game.crates[i], crateBox())
		game.WatchTransform(game.crates[i], nil)
	}
	game.hud()
}
func crateTransform(i int) engine.Transform3D {
	return engine.NewTransform3D(engine.NewVec3D(float32((i/4)%5-2)*3.5, -1+float32(i/20)*3.5, .51+float32(i%4)*1.015))
}
func crateBox() engine.PhysicsBox {
	return engine.PhysicsBox{HalfExtents: engine.NewVec3D(.5, .5, .5), Mass: 16, Friction: .5, Restitution: .08}
}
func playerBox() engine.PhysicsBox {
	return engine.PhysicsBox{HalfExtents: engine.NewVec3D(.3, .3, .9), Mass: 80, Offset: engine.NewVec3D(0, 0, -.75), LockRotation: true}
}
func (game *Game) key(key engine.Key, down bool) {
	if int(key) < len(game.keys) {
		game.keys[key] = down
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
			game.lookX += p.DeltaX
			game.lookY += p.DeltaY
		}
		if p.Phase == engine.PointerDown {
			if game.action(p.X, p.Y) {
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
		if p.X > width-272 && p.X < width-22 && p.Y > height-240 && p.Y < height-125 {
			game.shootID = p.ID
			game.pendingShot = true
			return
		}
		if game.action(p.X, p.Y) {
			return
		}
		if p.X < width/2 && game.moveID == 0 {
			game.moveID = p.ID
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
			game.lookX += p.X - game.lastLookX
			game.lookY += p.Y - game.lastLookY
			game.lastLookX, game.lastLookY = p.X, p.Y
		}
	case engine.PointerUp, engine.PointerCancel:
		if p.ID == game.moveID {
			game.moveID = 0
			game.moveX, game.moveY = 0, 0
			game.moveThumb.Set(engine.NewVec2D(110, height-104), engine.NewVec2D(1, 1), 0)
		}
		if p.ID == game.lookID {
			game.lookID = 0
		}
		if p.ID == game.shootID {
			game.shootID = 0
		}
	}
}
func (game *Game) action(x, y float32) bool {
	switch {
	case x > width-185 && y < 90:
		game.pendingReset = true
		return true
	case x > width-272 && x < width-22 && y > height-330 && y < height-245:
		game.pendingBlast = true
		return true
	case x > width-272 && x < width-22 && y > height-240 && y < height-125:
		game.pendingShot = true
		return true
	}
	return false
}
func (game *Game) move(x, y float32) {
	dx, dy := x-110, y-(height-104)
	length := float32(math.Hypot(float64(dx), float64(dy)))
	if length > 65 {
		dx *= 65 / length
		dy *= 65 / length
	}
	game.moveX, game.moveY = dx/65, -dy/65
	game.moveThumb.Set(engine.NewVec2D(110+dx, height-104+dy), engine.NewVec2D(1, 1), 0)
}
func (game *Game) resetInput() {
	game.keys = [256]bool{}
	game.moveID, game.lookID, game.shootID = 0, 0, 0
	game.moveX, game.moveY, game.lookX, game.lookY = 0, 0, 0, 0
	game.mouseHeld = false
	game.pendingShot, game.pendingBlast = false, false
	if game.moveThumb != nil {
		game.moveThumb.Set(engine.NewVec2D(110, height-104), engine.NewVec2D(1, 1), 0)
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
	settings := game.camera.Settings
	right, forward := game.moveX, game.moveY
	if game.keys[engine.KeyW] {
		forward++
	}
	if game.keys[engine.KeyS] {
		forward--
	}
	if game.keys[engine.KeyD] {
		right++
	}
	if game.keys[engine.KeyA] {
		right--
	}
	yaw, pitch := settings.Yaw+game.lookX*.006, settings.Pitch-game.lookY*.006
	game.lookX, game.lookY = 0, 0
	if game.keys[engine.KeyLeft] {
		yaw -= .035
	}
	if game.keys[engine.KeyRight] {
		yaw += .035
	}
	if game.keys[engine.KeyArrowUp] {
		pitch += .025
	}
	if game.keys[engine.KeyArrowDown] {
		pitch -= .025
	}
	pitch = max(-1.2, min(1.2, pitch))
	if yaw != settings.Yaw || pitch != settings.Pitch {
		settings.Yaw, settings.Pitch = yaw, pitch
		game.camera.SetSettings(settings)
	}
	magnitude := float32(math.Hypot(float64(right), float64(forward)))
	if magnitude > 1 {
		right /= magnitude
		forward /= magnitude
	}
	speed := float32(4.5)
	if game.keys[engine.KeyShiftLeft] || game.keys[engine.KeyShiftRight] {
		speed = 7
	}
	sine, cosine := math.Sincos(float64(yaw))
	game.MovePhysicsBody(
		game.camera,
		engine.NewVec2D((float32(cosine)*right+float32(sine)*forward)*speed, (-float32(sine)*right+float32(cosine)*forward)*speed),
	)
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
	settings := game.camera.Settings
	sy, cy := math.Sincos(float64(settings.Yaw))
	sp, cp := math.Sincos(float64(settings.Pitch))
	ray := engine.PhysicsRay{
		Origin:    settings.Position,
		Direction: engine.NewVec3D(float32(sy*cp), float32(cy*cp), float32(sp)),
		Distance:  60,
		Impulse:   95,
		Exclude:   game.camera.ID,
	}
	if blast {
		ray.Impulse = 260
		ray.ExplosionRadius = 5
		game.blasts++
	} else {
		game.shots++
	}
	game.ApplyPhysicsRay(game.level, ray)
	game.hud()
}
func (game *Game) reset() {
	game.resetInput()
	game.shots, game.blasts = 0, 0
	game.pendingShot, game.pendingBlast = false, false
	settings := game.camera.Settings
	settings.Position = engine.NewVec3D(.2, -10, 1.67)
	settings.Yaw, settings.Pitch = 0, 0
	game.camera.SetSettings(settings)
	game.SetPhysicsBox(game.camera, playerBox())
	for i, crate := range game.crates {
		crate.SetTransform(crateTransform(i))
		game.SetPhysicsBox(crate, crateBox())
	}
	game.hud()
}
func (game *Game) hud() {
	game.status.SetText(
		"CRATE ROOM · 80 boxes · Shots " + strconv.FormatUint(game.shots, 10) + " · Blasts " + strconv.FormatUint(game.blasts, 10),
	)
}
