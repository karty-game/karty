package main

import (
	"math"

	"example.com/hangar/.karty/assets"
	"example.com/hangar/.karty/engine"
)

// Prefer nearby moving crates and count fading voices against the sound budget.
func (audio *hangarAudio) updateCrateSounds(host *engine.Game, camera *engine.Camera3D, seconds float64, frame uint64) {
	desired := audio.crateSoundVolumes(seconds, frame)
	selected := audio.closestMovingCrates(camera, desired)
	// Stopped voices still count as resident until their fade has finished.
	active, resident := 0, 0
	for index, emitter := range audio.crates {
		if emitter != nil {
			active++
			resident++
		}
		if frame < audio.retiringUntil[index] {
			resident++
		}
	}
	for index, emitter := range audio.crates {
		if emitter == nil {
			continue
		}
		volume := desired[index]
		removed := audio.sources[index] == nil
		displaced := volume > 0 && !selected[index]
		idle := volume == 0 && frame-audio.lastMoved[index] >= crateSoundIdleFrames
		if removed || displaced || idle {
			audio.retireCrateSound(index, frame)
			active--
			// A stopped voice still uses its slot until its fade finishes.
			continue
		}
		audio.setCrateSoundVolume(index, volume)
	}
	// Reserve one slot so an idle voice can fade while a moving crate starts.
	if active >= maxMovingCrateSounds && resident < maxResidentCrateSounds &&
		audio.retireIdleCrateForMotion(camera, selected, desired, frame) {
		active--
	}
	for active < maxMovingCrateSounds && resident < maxResidentCrateSounds {
		index := audio.nextCrateSound(camera, selected, frame)
		if index < 0 {
			break
		}
		// Attach before playback so buffering uses the real source and volume.
		emitter := host.NewAttachedSoundEmitter(audio.level, assets.SoundCrateScrape, audio.sources[index].Ref(),
			engine.Vec3{}, crateSoundOptions(desired[index]))
		if emitter == nil {
			break
		}
		audio.crates[index] = emitter
		audio.volume[index] = desired[index]
		active++
		resident++
	}
}

func (audio *hangarAudio) crateSoundVolumes(seconds float64, frame uint64) [len(freightStacks)]float32 {
	var desired [len(freightStacks)]float32
	for index, crate := range audio.sources {
		if crate == nil {
			continue
		}
		position := crate.Transform().Position
		previous := audio.previous[index]
		// Track silent crates too, so waking a voice cannot create a speed spike.
		audio.previous[index] = position
		speed := math.Hypot(float64(position.X-previous.X), float64(position.Y-previous.Y)) / seconds
		desired[index] = float32(max(0, min(.8, (speed-.04)*.5)))
		if desired[index] > 0 {
			audio.lastMoved[index] = frame
		}
	}
	return desired
}

func (audio *hangarAudio) nextCrateSound(camera *engine.Camera3D, selected [len(freightStacks)]bool, frame uint64) int {
	index := -1
	distance := math.Inf(1)
	for candidate, wanted := range selected {
		if !wanted || audio.crates[candidate] != nil || frame < audio.retiringUntil[candidate] {
			continue
		}
		square := audio.crateSoundDistance(camera, candidate)
		if square < distance {
			index = candidate
			distance = square
		}
	}
	return index
}

func (audio *hangarAudio) retireCrateSound(index int, frame uint64) {
	audio.crates[index].Stop()
	audio.crates[index] = nil
	audio.volume[index] = 0
	audio.retiringUntil[index] = frame + crateSoundRetireFrames
}

// Evict at most one silent voice when a newly moving crate is waiting for a slot.
func (audio *hangarAudio) retireIdleCrateForMotion(
	camera *engine.Camera3D,
	selected [len(freightStacks)]bool,
	volume [len(freightStacks)]float32,
	frame uint64,
) bool {
	waiting := false
	for index, wanted := range selected {
		if wanted && audio.crates[index] == nil && frame >= audio.retiringUntil[index] {
			waiting = true
			break
		}
	}
	if !waiting {
		return false
	}
	retired := -1
	farthest := float64(-1)
	for index, emitter := range audio.crates {
		if emitter == nil || volume[index] != 0 {
			continue
		}
		distance := audio.crateSoundDistance(camera, index)
		if distance > farthest {
			retired = index
			farthest = distance
		}
	}
	if retired < 0 {
		return false
	}
	audio.retireCrateSound(retired, frame)
	return true
}

func crateSoundOptions(volume float32) engine.SoundEmitterOptions {
	return engine.SoundEmitterOptions{Volume: volume, NearDistance: .5, FarDistance: 9, Loop: true, Occlusion: true}
}

func (audio *hangarAudio) setCrateSoundVolume(index int, volume float32) {
	// Fades use wall time. Retry active voices during catch-up updates in case
	// a fading voice held their slot; existing playback does not restart.
	if volume == 0 && audio.volume[index] == 0 {
		return
	}
	audio.volume[index] = volume
	audio.crates[index].SetOptions(crateSoundOptions(volume))
}

// Fixed arrays avoid per-update sorting allocations. Select the closest moving
// crates when a cascade exceeds the live voice budget; ties keep source order.
func (audio *hangarAudio) closestMovingCrates(camera *engine.Camera3D, volume [len(freightStacks)]float32) [len(freightStacks)]bool {
	var selected [len(freightStacks)]bool
	for range maxMovingCrateSounds {
		closest := -1
		distance := math.Inf(1)
		for index, crate := range audio.sources {
			if crate == nil || volume[index] == 0 || selected[index] {
				continue
			}
			square := audio.crateSoundDistance(camera, index)
			if square < distance {
				closest = index
				distance = square
			}
		}
		if closest < 0 {
			break
		}
		selected[closest] = true
	}
	return selected
}

func (audio *hangarAudio) crateSoundDistance(camera *engine.Camera3D, index int) float64 {
	var receiver engine.Vec3
	if camera != nil {
		receiver = camera.Settings().Position
	}
	p := audio.sources[index].Transform().Position
	dx, dy, dz := float64(p.X-receiver.X), float64(p.Y-receiver.Y), float64(p.Z-receiver.Z)
	return dx*dx + dy*dy + dz*dz
}
