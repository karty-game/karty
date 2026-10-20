package main

import (
	"math"

	"example.com/particle-lab/.karty/config"
	"example.com/particle-lab/.karty/engine"
	"example.com/particle-lab/.karty/ui"
)

type cameraPointer struct {
	id   uint32
	x, y float32
}

func (game *Game) setStatus(status string) {
	game.status = status
	if game.panel != nil {
		game.panel.Invalidate()
	}
}

func (game *Game) showControls() {
	game.panel = ui.Controls(ui.ControlsProps{
		Ready:      func() bool { return game.camera != nil && game.request == 0 && game.release == 0 },
		Status:     func() string { return game.status },
		Hidden:     func() bool { return game.hidden },
		Show:       func() { game.setControlsHidden(false) },
		Hide:       func() { game.setControlsHidden(true) },
		Mode:       game.controlMode,
		SelectMode: game.selectMode,
		SelectFire: game.selectFire,
		SelectSmoke: func(index int) {
			if index == 2 {
				game.visit(9)
			} else if index >= 0 && index < 2 {
				game.visit(10 + index)
			}
		},
		Visit:   game.visit,
		Rotate:  game.rotate,
		Zoom:    game.zoom,
		Center:  func() { game.visit(game.view) },
		Restart: game.restart,
	})
}

func (game *Game) setControlsHidden(hidden bool) {
	game.hidden = hidden
	if game.panel != nil {
		game.panel.Invalidate()
	}
}

func (game *Game) controlMode() int {
	if game.mode == smokeMode {
		return 1
	}
	if game.mode == collectionMode {
		return 2
	}
	return 0
}

func (game *Game) selectMode(mode int) {
	switch mode {
	case 0:
		game.selectView(fireMode, 0)
	case 1:
		game.selectView(smokeMode, 10)
	case 2:
		game.selectView(collectionMode, 6)
	}
}

func (game *Game) selectView(mode, view int) {
	if game.request != 0 || game.release != 0 {
		return
	}
	if game.mode == mode {
		game.visit(view)
		return
	}
	game.mode, game.view = mode, view
	game.restart()
}

func (game *Game) selectFire(index int) {
	switch index {
	case 0:
		game.selectView(fireMode, 0)
	case 1:
		game.selectView(fireMode, 7)
	case 2:
		game.selectView(fireMode, 8)
	}
}

// The UI follows actual window dimensions. World input is mapped into the
// authored square; this central region excludes the desktop side dock and
// portrait header/bottom dock. Controls also provide rotation and zoom without
// a gesture, including when a screen leaves little central space.
func (game *Game) sceneContains(x, y float32) bool {
	return x > float32(config.ResolutionWidth)*.2 && x < float32(config.ResolutionWidth)*.68 &&
		y > float32(config.ResolutionHeight)*.17 && y < float32(config.ResolutionHeight)*.68
}

func (game *Game) pointer(pointer engine.PointerInput) {
	if game.camera == nil {
		return
	}
	if pointer.Phase == engine.PointerCancel {
		game.releasePointer()
		return
	}
	if pointer.Kind == engine.PointerMouse {
		if pointer.Phase == engine.PointerDown && pointer.Buttons&1 != 0 && game.sceneContains(pointer.X, pointer.Y) {
			game.mouseDragging = true
		}
		if pointer.Phase == engine.PointerMove && game.mouseDragging && pointer.Buttons&1 != 0 {
			game.rotate(-pointer.DeltaX * .004)
		}
		if pointer.Phase == engine.PointerUp {
			game.mouseDragging = false
		}
		return
	}
	if pointer.Kind != engine.PointerTouch {
		return
	}
	if pointer.Phase == engine.PointerDown {
		if !game.sceneContains(pointer.X, pointer.Y) {
			return
		}
		for index := range game.pointers {
			if game.pointers[index].id == 0 {
				game.pointers[index] = cameraPointer{pointer.ID, pointer.X, pointer.Y}
				break
			}
		}
		return
	}
	for index := range game.pointers {
		contact := &game.pointers[index]
		if contact.id != pointer.ID {
			continue
		}
		if pointer.Phase == engine.PointerUp {
			*contact = cameraPointer{}
			return
		}
		if pointer.Phase != engine.PointerMove {
			return
		}
		other := game.pointers[1-index]
		if other.id == 0 {
			game.rotate(-(pointer.X - contact.x) * .004)
		} else {
			before := math.Hypot(float64(contact.x-other.x), float64(contact.y-other.y))
			after := math.Hypot(float64(pointer.X-other.x), float64(pointer.Y-other.y))
			if before > 8 && after > 8 {
				game.zoom(float32(before / after))
			}
		}
		contact.x, contact.y = pointer.X, pointer.Y
		return
	}
}

func (game *Game) target() (x, y float32) {
	switch game.view {
	case 0:
		return -1.25, 0
	case 7:
		return 1.25, 0
	case 1:
		return 0, -3
	case 2:
		return 6, -3
	case 3:
		return -6, 4
	case 4:
		return 0, 4
	case 5:
		return 6, 4
	case 10:
		return -3, 0
	case 11:
		return 3, 0
	default:
		return 0, 0
	}
}

func (game *Game) rotate(amount float32) {
	if game.camera == nil || game.release != 0 {
		return
	}
	x, y := game.target()
	settings := game.camera.Settings()
	radius := math.Hypot(float64(settings.Position.X-x), float64(settings.Position.Y-y))
	settings.Yaw += amount
	settings.Position.X = x - float32(math.Sin(float64(settings.Yaw))*radius)
	settings.Position.Y = y - float32(math.Cos(float64(settings.Yaw))*radius)
	game.camera.SetSettings(settings)
}

func (game *Game) zoom(factor float32) {
	if game.camera == nil || game.release != 0 || factor <= 0 {
		return
	}
	settings := game.camera.Settings()
	settings.VerticalFOV = min(max(settings.VerticalFOV*factor, .38), 1.45)
	game.camera.SetSettings(settings)
}
