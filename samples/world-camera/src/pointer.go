package main

import (
	"math"

	"example.com/world-camera/.karty/config"
	"example.com/world-camera/.karty/engine"
)

type cameraPointer struct {
	id                   uint32
	x, y, startX, startY float32
}

const moveStickRadius float32 = 72

type moveStickOverlay struct{ base, thumb *engine.EntityVector2D }

func (game *Game) controlHelp() string {
	if game.isometric {
		return "Drag / WASD pan · Q/E orbit\nPinch / +/- zoom · Center resets view"
	}
	if game.touchMode {
		return "Left joystick walks · Right swipe looks\nUse the buttons to change view"
	}
	return "WASD walk · Mouse look · Shift run\nEsc release · C controls · V view"
}

func (game *Game) onPointer(pointer engine.PointerInput) {
	if game.camera == nil {
		return
	}
	if pointer.Phase == engine.PointerCancel {
		game.stopMotion()
		return
	}
	if pointer.Kind == engine.PointerMouse {
		if pointer.Phase == engine.PointerDown && pointer.Buttons&1 != 0 && game.sceneContains(pointer.X, pointer.Y) && !game.isometric {
			game.SetCursorCaptured(true)
		}
		if pointer.Phase == engine.PointerMove {
			if game.isometric && pointer.Buttons&1 != 0 && game.mouseDragging {
				game.dragPan(pointer.DeltaX, pointer.DeltaY)
			} else if !game.isometric && pointer.Captured {
				game.mouseX += pointer.DeltaX
				game.mouseY += pointer.DeltaY
			}
		}
		if game.isometric && pointer.Phase == engine.PointerDown && pointer.Buttons&1 != 0 && game.sceneContains(pointer.X, pointer.Y) {
			game.mouseDragging = true
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
		if !game.touchMode {
			game.touchMode = true
			game.invalidateControls()
		}
		if game.controlsHidden && pointer.Y < 100 {
			game.stopMotion()
			game.showControls()
			return
		}
		if !game.touchSceneContains(pointer.Y) {
			return
		}
		index := 0
		if !game.isometric && pointer.X >= float32(config.ResolutionWidth)*.5 {
			index = 1
		}
		if game.isometric && game.pointers[0].id != 0 {
			index = 1
		}
		if game.pointers[index].id == 0 {
			game.pointers[index] = cameraPointer{pointer.ID, pointer.X, pointer.Y, pointer.X, pointer.Y}
			if !game.isometric && index == 0 {
				game.drawMoveStick(pointer.X, pointer.Y, 0, 0)
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
			if !game.isometric && index == 0 {
				game.touchMove, game.touchStrafe = 0, 0
				game.hideMoveStick()
			}
			return
		}
		if pointer.Phase != engine.PointerMove {
			return
		}
		dx, dy := pointer.X-contact.x, pointer.Y-contact.y
		if game.isometric {
			other := game.pointers[1-index]
			if other.id == 0 {
				game.dragPan(dx, dy)
			} else {
				before := math.Hypot(float64(contact.x-other.x), float64(contact.y-other.y))
				after := math.Hypot(float64(pointer.X-other.x), float64(pointer.Y-other.y))
				if before > 8 && after > 8 {
					game.Zoom(float32(before / after))
				}
				game.dragPan(dx*.5, dy*.5)
			}
		} else if index == 0 {
			stickX, stickY := pointer.X-contact.startX, pointer.Y-contact.startY
			game.touchStrafe, game.touchMove = moveStickInput(stickX, stickY)
			game.drawMoveStick(contact.startX, contact.startY, stickX, stickY)
		} else {
			game.mouseX += dx * 2
			game.mouseY += dy * 2
		}
		contact.x, contact.y = pointer.X, pointer.Y
		return
	}
}

func (game *Game) touchSceneContains(y float32) bool {
	if game.isometric {
		return game.sceneContains(0, y)
	}
	// Portrait presentation can put the thumb below the authored game rectangle.
	// Reserve the top toolbar, but accept the entire lower touch area.
	return !game.inspecting && (game.controls == nil || y > float32(config.ResolutionHeight)*.45)
}

func moveStickInput(dx, dy float32) (strafe, forward float32) {
	length := float32(math.Hypot(float64(dx), float64(dy)))
	if length <= moveStickRadius*.15 {
		return 0, 0
	}
	strength := min(1, (length/moveStickRadius-.15)/.85)
	return dx / length * strength, -dy / length * strength
}

func (game *Game) drawMoveStick(x, y, dx, dy float32) {
	// Vector entities share the authored scene bounds. Keep the indicator inside
	// that image even when a portrait thumb starts in the surrounding letterbox.
	x = max(moveStickRadius+8, min(float32(config.ResolutionWidth)-moveStickRadius-8, x))
	y = max(moveStickRadius+8, min(float32(config.ResolutionHeight)-moveStickRadius-8, y))
	basePosition := engine.NewVec2(x, y)
	if game.moveStick.base == nil {
		base := game.NewVector2D(engine.Circle(moveStickRadius), basePosition)
		base.SetFill(engine.RGBA(20, 30, 34, 140))
		base.SetStroke(engine.RGBA(217, 183, 113, 200), 2)
		base.SetLayer(20)
		thumb := game.NewVector2D(engine.Circle(23), basePosition)
		thumb.SetFill(engine.RGBA(217, 183, 113, 220))
		thumb.SetLayer(21)
		game.moveStick = moveStickOverlay{base, thumb}
	}
	if length := float32(math.Hypot(float64(dx), float64(dy))); length > moveStickRadius {
		dx *= moveStickRadius / length
		dy *= moveStickRadius / length
	}
	thumbPosition := engine.NewVec2(x+dx, y+dy)
	for _, item := range []struct {
		entity   *engine.EntityVector2D
		position engine.Vec2
	}{{game.moveStick.base, basePosition}, {game.moveStick.thumb, thumbPosition}} {
		if item.entity.Position != item.position {
			item.entity.Set(item.position, engine.NewVec2(1, 1), 0)
		}
		if item.entity.Alpha != 1 {
			item.entity.SetAlpha(1)
		}
	}
}

func (game *Game) hideMoveStick() {
	if game.moveStick.base != nil && game.moveStick.base.Alpha != 0 {
		game.moveStick.base.SetAlpha(0)
		game.moveStick.thumb.SetAlpha(0)
	}
}

func (game *Game) sceneContains(_, y float32) bool {
	if game.controls == nil {
		return true
	}
	return !game.inspecting && y > 300 && y < float32(config.ResolutionHeight)-200
}

func (game *cameraController) dragPan(dx, dy float32) {
	// Screen vertical maps to the ground plane through the fixed isometric pitch.
	scale := game.orbitSettings().OrthoHeight / float32(config.ResolutionHeight)
	game.panX -= dx * scale
	game.panY += dy * scale / float32(math.Sin(.6154797))
}
