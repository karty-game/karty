package main

import (
	"math"

	"example.com/particle-lab/.karty/config"
	"example.com/particle-lab/.karty/engine"
)

type cameraPointer struct {
	id   uint32
	x, y float32
}

// The UI follows actual window dimensions. World input is mapped into the
// authored square; this central region excludes the desktop side dock and
// portrait header/bottom dock. Controls also provide rotation and zoom without
// a gesture, including when a screen leaves little central space.
func (camera *effectCamera) sceneContains(x, y float32) bool {
	return x > float32(config.ResolutionWidth)*.2 && x < float32(config.ResolutionWidth)*.68 &&
		y > float32(config.ResolutionHeight)*.17 && y < float32(config.ResolutionHeight)*.68
}

// pointer turns one-finger drags into orbits and two-finger distance changes into zoom.
// Releases still clear contacts while a room change blocks camera movement.
func (camera *effectCamera) pointer(host *engine.Game, pointer engine.PointerInput, view int, releasing bool) {
	if camera.entity == nil {
		return
	}
	if pointer.Phase == engine.PointerCancel {
		camera.releasePointer(host)
		return
	}
	if pointer.Kind == engine.PointerMouse {
		if pointer.Phase == engine.PointerDown && pointer.Buttons&1 != 0 && camera.sceneContains(pointer.X, pointer.Y) {
			camera.mouseDragging = true
		}
		if pointer.Phase == engine.PointerMove && camera.mouseDragging && pointer.Buttons&1 != 0 && !releasing {
			camera.rotate(view, -pointer.DeltaX*.004)
		}
		if pointer.Phase == engine.PointerUp {
			camera.mouseDragging = false
		}
		return
	}
	if pointer.Kind != engine.PointerTouch {
		return
	}
	if pointer.Phase == engine.PointerDown {
		if !camera.sceneContains(pointer.X, pointer.Y) {
			return
		}
		for index := range camera.pointers {
			if camera.pointers[index].id == 0 {
				camera.pointers[index] = cameraPointer{pointer.ID, pointer.X, pointer.Y}
				break
			}
		}
		return
	}
	for index := range camera.pointers {
		contact := &camera.pointers[index]
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
		other := camera.pointers[1-index]
		if other.id == 0 {
			if !releasing {
				camera.rotate(view, -(pointer.X-contact.x)*.004)
			}
		} else {
			before := math.Hypot(float64(contact.x-other.x), float64(contact.y-other.y))
			after := math.Hypot(float64(pointer.X-other.x), float64(pointer.Y-other.y))
			if before > 8 && after > 8 && !releasing {
				camera.zoom(float32(before / after))
			}
		}
		contact.x, contact.y = pointer.X, pointer.Y
		return
	}
}

func (camera *effectCamera) releasePointer(host *engine.Game) {
	host.SetCursorCaptured(false)
	camera.mouseDragging = false
	camera.pointers = [2]cameraPointer{}
}

func (camera *effectCamera) pressed(key engine.Key, down bool) bool {
	if int(key) >= len(camera.keys) {
		return false
	}
	held := camera.keys[key]
	camera.keys[key] = down
	return down && !held
}
