package main

import (
	"example.com/effect-lab/.karty/config"
	"example.com/effect-lab/.karty/engine"
	"example.com/effect-lab/.karty/ui"
	"github.com/karty-game/controllers"
)

type Game struct {
	engine.Game
	camera     *engine.Camera3D
	controller *controllers.FirstPerson
	level      engine.LevelHandle
	request    engine.RequestID
	status     string
	panel      *engine.UI
	keys       [256]bool
	view       int
}

//go:wasmexport karty_register
func main() {
	g := &Game{}
	engine.Run(engine.Hooks{
		OnStart: g.start, OnUpdate: g.update, OnStop: g.stop,
		OnKeyDown: func(k engine.Key) { g.key(k, true) },
		OnKeyUp:   func(k engine.Key) { g.key(k, false) },
		OnPointer: g.pointer, OnInputReset: g.resetInput,
		OnLevelReady: g.ready,
		OnLevelFailed: func(e engine.LevelFailed) {
			if e.RequestID == g.request {
				g.request = 0
				g.setStatus("Load failed: " + e.Diagnostic)
			}
		},
	})
}

func (g *Game) start() {
	o := controllers.DefaultFirstPersonOptions()
	o.Width, o.Height, o.Speed = float32(config.ResolutionWidth), float32(config.ResolutionHeight), 3.5
	var err error
	g.controller, err = controllers.NewFirstPerson(o)
	if err != nil {
		panic(err)
	}
	g.status = "Loading effect lab…"
	g.panel = ui.Hud(ui.HudProps{Status: func() string { return g.status }})
	for _, offset := range [...]engine.Vec2{engine.NewVec2(-6, 0), engine.NewVec2(0, -6)} {
		shape := engine.Line(engine.NewVec2(12, 0))
		if offset.Y != 0 {
			shape = engine.Line(engine.NewVec2(0, 12))
		}
		cross := g.NewShape2D(shape, engine.NewVec2(640+offset.X, 360+offset.Y))
		cross.SetStroke(engine.RGBA(235, 244, 255, 180), 1)
		cross.SetLayer(22)
	}
	g.request = g.RequestLevel("levels.effect-lab")
}

func (g *Game) setStatus(value string) {
	g.status = value
	if g.panel != nil {
		g.panel.Invalidate()
	}
}

func (g *Game) ready(e engine.LevelReady) {
	if e.RequestID != g.request || g.request == 0 {
		g.ReleaseLevel(e.Handle)
		return
	}
	g.request, g.level = 0, e.Handle
	g.camera = g.NewCamera3D(e.Handle, engine.Camera3DSettings{
		Projection: engine.CameraProjectionPerspective, Output: engine.CameraOutputFull,
		Position:    engine.NewVec3(0, -9, 1.65),
		VerticalFOV: config.CameraFOVY, NearClip: config.CameraNear, FarClip: config.CameraFar,
		OrthoHeight:   config.CameraOrthoHeight,
		ViewportWidth: config.ResolutionWidth, ViewportHeight: config.ResolutionHeight,
	})
	g.camera.SetLayer(-10)
	g.visit(0)
}

func (g *Game) visit(index int) {
	if g.camera == nil {
		return
	}
	g.releasePointer()
	poses := [...]struct {
		position   engine.Vec3
		yaw, pitch float32
		name       string
	}{
		{engine.NewVec3(0, -9, 1.65), 0, -.025, "EFFECT LAB / overview"},
		{engine.NewVec3(0, 4.5, 1.65), 0, 0, "EFFECT LAB / grayscale and color ramps"},
		{engine.NewVec3(-6.8, -6.2, 1.65), .4, -.05, "EFFECT LAB / materials and colored light"},
	}
	g.view = index
	p := poses[index]
	s := g.camera.Settings()
	s.Position, s.Yaw, s.Pitch = p.position, p.yaw, p.pitch
	// A viewpoint is a teleport. Recreate the camera so collision sweeps do not
	// turn a jump across the display objects into an obstructed walking move.
	g.camera.Destroy()
	g.camera = g.NewCamera3D(g.level, s)
	g.camera.SetLayer(-10)
	g.setStatus(p.name)
}

func (g *Game) key(key engine.Key, down bool) {
	if int(key) >= len(g.keys) {
		return
	}
	held := g.keys[key]
	g.keys[key] = down
	controls := map[engine.Key]controllers.Control{
		engine.KeyW: controllers.Forward, engine.KeyS: controllers.Backward,
		engine.KeyA: controllers.Left, engine.KeyD: controllers.Right,
		engine.KeyLeft: controllers.LookLeft, engine.KeyRight: controllers.LookRight,
		engine.KeyArrowUp: controllers.LookUp, engine.KeyArrowDown: controllers.LookDown,
		engine.KeyShiftLeft: controllers.RunLeft, engine.KeyShiftRight: controllers.RunRight,
	}
	if control, ok := controls[key]; ok {
		g.controller.Key(control, down)
	}
	if !down || held {
		return
	}
	switch key {
	case engine.KeyEscape:
		g.releasePointer()
	case engine.KeyR:
		g.visit(g.view)
	case engine.KeyDigit1:
		g.visit(0)
	case engine.KeyDigit2:
		g.visit(1)
	case engine.KeyDigit3:
		g.visit(2)
	}
}

func (g *Game) pointer(p engine.PointerInput) {
	if g.camera == nil || p.Kind != engine.PointerMouse && p.Kind != engine.PointerTouch {
		return
	}
	var phase controllers.PointerPhase
	switch p.Phase {
	case engine.PointerDown:
		phase = controllers.PointerDown
	case engine.PointerMove:
		phase = controllers.PointerMove
	case engine.PointerUp:
		phase = controllers.PointerUp
	case engine.PointerCancel:
		phase = controllers.PointerCancel
	default:
		return
	}
	result := g.controller.Pointer(controllers.Pointer{
		Phase: phase, Touch: p.Kind == engine.PointerTouch, ID: p.ID, X: p.X, Y: p.Y,
		DeltaX: p.DeltaX, DeltaY: p.DeltaY, Primary: p.Buttons&1 != 0, Captured: p.Captured,
	}, p.Y > 125)
	if result.Capture {
		g.SetCursorCaptured(true)
	}
	if result.Release {
		g.releasePointer()
	}
}

func (g *Game) update(engine.Frame) {
	if g.camera == nil {
		return
	}
	s := g.camera.Settings()
	m := g.controller.Step(s.Yaw, s.Pitch)
	if m.X == 0 && m.Y == 0 && m.Yaw == s.Yaw && m.Pitch == s.Pitch {
		return
	}
	s.Position.X += m.X
	s.Position.Y += m.Y
	s.Yaw = m.Yaw
	s.Pitch = m.Pitch
	g.camera.SetSettings(s)
}

func (g *Game) resetInput() {
	g.keys = [256]bool{}
	if g.controller != nil {
		g.controller.Reset()
	}
}
func (g *Game) releasePointer() { g.SetCursorCaptured(false); g.resetInput() }
func (g *Game) stop() {
	if g.panel != nil {
		g.panel.Close()
		g.panel = nil
	}
	g.releasePointer()
	if g.request != 0 {
		g.CancelLevelRequest(g.request)
	}
	if g.camera != nil {
		g.camera.Destroy()
	}
	if g.level != 0 {
		g.ReleaseLevel(g.level)
	}
}
