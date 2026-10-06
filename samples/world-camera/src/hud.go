package main

import (
	"strconv"

	"example.com/world-camera/.karty/engine"
)

// cameraHUD formats accepted camera settings only when a hook or UI change
// invalidates them. Idle frames perform no projection or formatting work.
type cameraHUD struct {
	cameraInfo string
	infoBuffer [1024]byte
}

func (hud *cameraHUD) Refresh(game *cameraController) bool {
	if game.camera == nil {
		return false
	}
	settings := game.fps
	text := appendCameraPose(hud.infoBuffer[:0], "FPS", settings)
	if game.isometric && game.orbitCamera != nil {
		settings = game.orbitPose
		text = append(text, '\n')
		text = appendCameraPose(text, "ISO", settings)
	}
	text = append(text, "\nView: "...)
	if game.isometric {
		text = append(text, "isometric"...)
	} else {
		text = append(text, "perspective"...)
	}
	text = append(text, "\nChannel: "...)
	text = append(text, game.outputName()...)
	for _, field := range [...]struct {
		name  string
		value float32
	}{{"\nFOVy(rad)=", settings.FOVY}, {" ortho=", settings.OrthoHeight}, {"\nnear=", settings.Near}, {" far=", settings.Far}, {" path=", game.path}} {
		text = append(text, field.name...)
		text = strconv.AppendFloat(text, float64(field.value), 'g', -1, 32)
	}
	text = append(text, "\nviewport="...)
	text = strconv.AppendUint(text, uint64(settings.ViewportWidth), 10)
	text = append(text, 'x')
	text = strconv.AppendUint(text, uint64(settings.ViewportHeight), 10)
	if string(text) != hud.cameraInfo {
		hud.cameraInfo = string(text)
		return true
	}
	return false
}

func appendCameraPose(text []byte, name string, settings engine.CameraSettings) []byte {
	text = append(text, name...)
	for _, field := range [...]struct {
		name  string
		value float32
	}{{"\nx=", settings.Position.X}, {" y=", settings.Position.Y}, {"\nz=", settings.Position.Z}, {"\nyaw(rad)=", settings.Yaw}, {"\npitch(rad)=", settings.Pitch}} {
		text = append(text, field.name...)
		text = strconv.AppendFloat(text, float64(field.value), 'g', -1, 32)
	}
	return text
}

func (hud *cameraHUD) Clear() { hud.cameraInfo = "" }
