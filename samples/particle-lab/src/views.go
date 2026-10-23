package main

import "example.com/particle-lab/.karty/engine"

// A viewpoint keeps its room, camera pose and orbit target together.
type viewpoint struct {
	name         string
	mode         effectMode
	position     engine.Vec3
	yaw, pitch   float32
	target       engine.Vec2
	hideControls bool
}

// These IDs also back the existing keyboard shortcuts and UI buttons.
const (
	combinedFireView = iota
	sparklesView
	acidView
	reactorView
	dischargeView
	explosionView
	collectionView
	separateFireView
	fireComparisonView
	smokeComparisonView
	thinSmokeView
	rollingSmokeView
)

// Close views isolate individual effects; comparison views leave the panel hidden.
var viewpoints = [...]viewpoint{
	combinedFireView: {
		name:     "Brackeys combined fire",
		mode:     fireMode,
		position: engine.NewVec3(-1.25, -1.6, 0.85),
		yaw:      0,
		pitch:    -0.1,
		target:   engine.NewVec2(-1.25, 0),
	},
	sparklesView: {
		name:     "Electric sparkles",
		mode:     collectionMode,
		position: engine.NewVec3(1, -5.5, 2.1),
		yaw:      -0.380506377,
		pitch:    -0.36,
		target:   engine.NewVec2(0, -3),
	},
	acidView: {
		name:     "Acid bubbles",
		mode:     collectionMode,
		position: engine.NewVec3(7, -5.5, 2.1),
		yaw:      -0.380506377,
		pitch:    -0.34,
		target:   engine.NewVec2(6, -3),
	},
	reactorView: {
		name:     "Reactor",
		mode:     collectionMode,
		position: engine.NewVec3(-4.6, 0.8, 3.3),
		yaw:      -0.412410442,
		pitch:    -0.03,
		target:   engine.NewVec2(-6, 4),
	},
	dischargeView: {
		name:     "Electric discharge",
		mode:     collectionMode,
		position: engine.NewVec3(1.4, 0.8, 2.4),
		yaw:      -0.412410442,
		pitch:    -0.31,
		target:   engine.NewVec2(0, 4),
	},
	explosionView: {
		name:     "Explosion · Restart to replay",
		mode:     collectionMode,
		position: engine.NewVec3(7.4, 0.8, 2.4),
		yaw:      -0.412410442,
		pitch:    -0.31,
		target:   engine.NewVec2(6, 4),
	},
	collectionView: {
		name:         "Collection",
		mode:         collectionMode,
		position:     engine.NewVec3(0, -10, 6.5),
		yaw:          0,
		pitch:        -0.42,
		target:       engine.NewVec2(0, 0),
		hideControls: true,
	},
	separateFireView: {
		name:     "Brackeys separate fire",
		mode:     fireMode,
		position: engine.NewVec3(1.25, -1.6, 0.85),
		yaw:      0,
		pitch:    -0.1,
		target:   engine.NewVec2(1.25, 0),
	},
	fireComparisonView: {
		name:         "Brackeys fire comparison",
		mode:         fireMode,
		position:     engine.NewVec3(0, -5, 1.5),
		yaw:          0,
		pitch:        -0.1,
		target:       engine.NewVec2(0, 0),
		hideControls: true,
	},
	smokeComparisonView: {
		name:         "Smoke comparison",
		mode:         smokeMode,
		position:     engine.NewVec3(0, -10, 2.9),
		yaw:          0,
		pitch:        0,
		target:       engine.NewVec2(0, 0),
		hideControls: true,
	},
	thinSmokeView: {
		name:     "Thin smoke",
		mode:     smokeMode,
		position: engine.NewVec3(-2.3, -3.2, 2.5),
		yaw:      -0.2153577,
		pitch:    0,
		target:   engine.NewVec2(-3, 0),
	},
	rollingSmokeView: {
		name:     "Rolling smoke",
		mode:     smokeMode,
		position: engine.NewVec3(4.4, -4.5, 2.85),
		yaw:      -0.30161905,
		pitch:    0,
		target:   engine.NewVec2(3, 0),
	},
}
