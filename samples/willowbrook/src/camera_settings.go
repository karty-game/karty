package main

import (
	"strconv"

	"example.com/willowbrook/.karty/engine"
	"example.com/willowbrook/.karty/ui"
	"github.com/karty-game/controllers"
)

// The sample owns these controls; the portable composer has no UI dependency.
type cameraLab struct {
	options, defaults    controllers.CameraFramingOptions
	open, locked, guides bool
	outlines             [2]*engine.Shape2D
}

func (g *Game) cameraSettingsProps() ui.CameraSettingsProps {
	return ui.CameraSettingsProps{
		Open:   func() bool { return g.cameraLab.open },
		Locked: func() bool { return g.cameraLab.locked },
		Guides: func() bool { return g.cameraLab.guides },
		Value:  g.cameraSettingValue, Label: g.cameraSettingLabel,
		Change: g.changeCameraSetting, Toggle: g.toggleCameraSettings,
		Compare: func() { g.cameraLab.locked = !g.cameraLab.locked; g.applyCameraSettings() },
		Reset: func() {
			g.cameraLab.options = g.cameraLab.defaults
			g.cameraLab.locked = false
			g.applyCameraSettings()
		},
		SetGuides: func(on bool) { g.cameraLab.guides = on; g.drawCameraGuides() },
	}
}

func (g *Game) toggleCameraSettings() {
	g.cameraLab.open = !g.cameraLab.open
	g.hud.Invalidate()
}

func (g *Game) cameraSettingValue(field int) int32 {
	o := g.cameraLab.options
	values := [7]float64{o.Damping.X * 100, o.Damping.Y * 100, o.DeadZone.X * 100,
		o.DeadZone.Y * 100, o.SoftZone.X * 100, o.SoftZone.Y * 100, o.LookaheadTime * 200}
	return int32(values[field] + .5)
}

func (g *Game) cameraSettingLabel(field int) string {
	names := [7]string{"X", "Y", "X", "Y", "X", "Y", "Lookahead"}
	value := int(g.cameraSettingValue(field))
	unit := "%"
	if field >= 2 && field <= 5 {
		o := g.cameraLab.options
		values := [4]float64{o.DeadZone.X, o.DeadZone.Y, o.SoftZone.X, o.SoftZone.Y}
		return names[field] + ": " + strconv.FormatFloat(values[field-2]*100, 'f', 1, 64) + "%"
	}
	if field < 2 {
		value *= 10
		unit = " ms"
	}
	if field == 6 {
		value *= 5
		unit = " ms"
	}
	return names[field] + ": " + strconv.Itoa(value) + unit
}

func (g *Game) changeCameraSetting(field int, value int32) {
	if g.cameraLab.locked {
		return
	}
	v := float64(max(0, min(100, value))) / 100
	o := &g.cameraLab.options
	switch field {
	case 0:
		o.Damping.X = v
	case 1:
		o.Damping.Y = v
	case 2:
		o.DeadZone.X = v
		o.SoftZone.X = max(v, o.SoftZone.X)
	case 3:
		o.DeadZone.Y = v
		o.SoftZone.Y = max(v, o.SoftZone.Y)
	case 4:
		o.SoftZone.X = v
		o.DeadZone.X = min(v, o.DeadZone.X)
	case 5:
		o.SoftZone.Y = v
		o.DeadZone.Y = min(v, o.DeadZone.Y)
	case 6:
		o.LookaheadTime = v / 2
	default:
		return
	}
	g.applyCameraSettings()
}

func (g *Game) applyCameraSettings() {
	o := g.cameraLab.options
	if g.cameraLab.locked {
		o.Damping, o.DeadZone, o.SoftZone = controllers.Vec2{}, controllers.Vec2{}, controllers.Vec2{}
		o.LookaheadTime = 0
	}
	if err := g.framing.Configure(o); err != nil {
		panic(err)
	}
	g.drawCameraGuides()
	g.hud.Invalidate()
}

func (g *Game) drawCameraGuides() {
	o := g.framing.Options()
	for i, zone := range [2]controllers.Vec2{o.DeadZone, o.SoftZone} {
		shape := g.cameraLab.outlines[i]
		if !g.cameraLab.guides || zone.X == 0 || zone.Y == 0 {
			if shape != nil {
				shape.SetStroke(engine.ColorTransparent, 0)
			}
			continue
		}
		w, h := float32(zone.X*1280), float32(zone.Y*720)
		position := engine.NewVec2(float32(o.ScreenPosition.X*1280)-w/2, float32(o.ScreenPosition.Y*720)-h/2)
		if shape == nil {
			shape = g.NewShape2D(engine.Rectangle(w, h), position)
			shape.SetFill(engine.ColorTransparent)
			shape.SetLayer(1000000050)
			g.cameraLab.outlines[i] = shape
		} else {
			shape.SetShape(engine.Rectangle(w, h))
			shape.SetTransform(position, engine.NewVec2(1, 1), 0)
		}
		color := engine.RGBA(90, 235, 155, 230)
		if i == 1 {
			color = engine.RGBA(245, 185, 65, 230)
		}
		shape.SetStroke(color, 2)
	}
}
