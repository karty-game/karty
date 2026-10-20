package main

import (
	"math"

	"example.com/hangar/.karty/assets"
	"example.com/hangar/.karty/engine"
)

const (
	maxPersistentSounds         = 16
	ambientSoundCount           = 8
	reservedRetiringCrateSounds = 1
	maxResidentCrateSounds      = maxPersistentSounds - ambientSoundCount
	maxMovingCrateSounds        = maxResidentCrateSounds - reservedRetiringCrateSounds
	crateSoundIdleFrames        = 30
	// Host stops fade for 100 ms. Eight ordinary 60 Hz updates leave margin
	// before admitting a replacement for a retired voice.
	crateSoundRetireFrames = 8
)

// All playback belongs to the mounted level; no emitter state enters saves.
// Eight ambience voices plus at most eight live/fading crate voices fit the host's
// sixteen-emitter budget. Idle crates consume no voice until horizontal motion.
type hangarAudio struct {
	ambient       [ambientSoundCount]*engine.SoundEmitter
	crates        [len(freightStacks)]*engine.SoundEmitter
	previous      [len(freightStacks)]engine.Vec3
	volume        [len(freightStacks)]float32
	lastMoved     [len(freightStacks)]uint64
	retiringUntil [len(freightStacks)]uint64
	frame         uint64
}

func (game *Game) createAudio() {
	game.audio.ambient[0] = game.NewSoundEmitter(game.level, assets.SoundReactorHum,
		engine.NewVec3(8, 21.7, 2.6), engine.SoundEmitterOptions{
			Volume: .8, NearDistance: 3, FarDistance: 28, Loop: true, Occlusion: true,
		})
	// The upper console bank uses the same recording and tuning as the lower
	// terminals. Its source sits in front of the authored console colliders.
	for index, position := range [...]engine.Vec3{
		engine.NewVec3(-13.7, 19, 3.2), engine.NewVec3(-13.7, 25, 3.2), engine.NewVec3(34.4, 11.85, 4.9),
	} {
		game.audio.ambient[index+1] = game.NewSoundEmitter(game.level, assets.SoundComputerIdle, position,
			engine.SoundEmitterOptions{Volume: .7, NearDistance: 1, FarDistance: 9, Loop: true, Occlusion: true})
	}
	// Above the barrel rim and the acid surface, respectively, so the source
	// does not start inside the solid drum or below the authored liquid surface.
	game.audio.ambient[4] = game.NewSoundEmitter(game.level, assets.SoundFireAmbience, engine.NewVec3(14.8, -1.55, 1.12),
		engine.SoundEmitterOptions{Volume: .25, NearDistance: 1, FarDistance: 8, Loop: true, Occlusion: true})
	// The south basin and both trench branches each have a quiet source above
	// the liquid's highest wave. Smaller gain and range keep overlap gentle.
	for index, position := range [...]engine.Vec3{
		engine.NewVec3(31, 18, -.95), engine.NewVec3(26.4, 29, -.95), engine.NewVec3(37, 29, -.95),
	} {
		game.audio.ambient[index+5] = game.NewSoundEmitter(game.level, assets.SoundAcidAmbience, position,
			engine.SoundEmitterOptions{Volume: .16, NearDistance: 1.5, FarDistance: 10, Loop: true, Occlusion: true})
	}
	for index, crate := range game.crates {
		if crate != nil {
			game.audio.previous[index] = crate.Transform().Position
		}
	}
}

func (game *Game) updateAudio(frame uint64) {
	if game.camera != nil {
		yaw := float64(game.camera.Settings().Yaw)
		game.SetAudioListener(game.camera.Ref(), engine.AudioListenerOptions{
			Right: engine.NewVec2(float32(math.Cos(yaw)), -float32(math.Sin(yaw))),
		})
	}
	if game.audio.frame == 0 {
		game.audio.frame = frame
		return
	}
	if frame <= game.audio.frame {
		return
	}
	seconds := float64(frame-game.audio.frame) / 60
	game.audio.frame = frame
	var desired [len(freightStacks)]float32
	for index, crate := range game.crates {
		if crate == nil {
			continue
		}
		position := crate.Transform().Position
		previous := game.audio.previous[index]
		// Track accepted positions even before a voice exists, so waking an idle
		// crate never measures accumulated displacement as a sudden speed spike.
		game.audio.previous[index] = position
		speed := math.Hypot(float64(position.X-previous.X), float64(position.Y-previous.Y)) / seconds
		desired[index] = float32(max(0, min(.8, (speed-.04)*.5)))
		if desired[index] > 0 {
			game.audio.lastMoved[index] = frame
		}
	}
	selected := game.closestMovingCrates(desired)
	active, resident := 0, 0
	for index, emitter := range game.audio.crates {
		if emitter != nil {
			active++
			resident++
		}
		if frame < game.audio.retiringUntil[index] {
			resident++
		}
	}
	for index, emitter := range game.audio.crates {
		if emitter == nil {
			continue
		}
		volume := desired[index]
		if game.crates[index] == nil || volume > 0 && !selected[index] ||
			volume == 0 && frame-game.audio.lastMoved[index] >= crateSoundIdleFrames {
			game.retireCrateSound(index, frame)
			active--
			// The retiring player still occupies the resident budget until its fade
			// finishes. It cannot be replaced as though Stop immediately freed it.
			continue
		}
		game.setCrateSoundVolume(index, volume)
	}
	// Idle debounce must not delay a different moving crate. The reserved
	// resident slot lets one silent warm player fade while its replacement
	// starts immediately with the correct source and movement gain.
	if active >= maxMovingCrateSounds && resident < maxResidentCrateSounds && game.retireIdleCrateForMotion(selected, desired, frame) {
		active--
	}
	for active < maxMovingCrateSounds && resident < maxResidentCrateSounds {
		index := -1
		distance := math.Inf(1)
		for candidate, wanted := range selected {
			if !wanted || game.audio.crates[candidate] != nil || frame < game.audio.retiringUntil[candidate] {
				continue
			}
			square := game.crateSoundDistance(candidate)
			if square < distance {
				index = candidate
				distance = square
			}
		}
		if index < 0 {
			break
		}
		// Give the host the real source and initial movement gain together;
		// it must never prebuffer PCM using an unattached origin-point source.
		emitter := game.NewAttachedSoundEmitter(game.level, assets.SoundCrateScrape, game.crates[index].Ref(),
			engine.Vec3{}, crateSoundOptions(desired[index]))
		if emitter == nil {
			break
		}
		game.audio.crates[index] = emitter
		game.audio.volume[index] = desired[index]
		active++
		resident++
	}
}

func (game *Game) retireCrateSound(index int, frame uint64) {
	game.audio.crates[index].Stop()
	game.audio.crates[index] = nil
	game.audio.volume[index] = 0
	game.audio.retiringUntil[index] = frame + crateSoundRetireFrames
}

func (game *Game) retireIdleCrateForMotion(selected [len(freightStacks)]bool, volume [len(freightStacks)]float32, frame uint64) bool {
	waiting := false
	for index, wanted := range selected {
		if wanted && game.audio.crates[index] == nil && frame >= game.audio.retiringUntil[index] {
			waiting = true
			break
		}
	}
	if !waiting {
		return false
	}
	retired := -1
	farthest := float64(-1)
	for index, emitter := range game.audio.crates {
		if emitter == nil || volume[index] != 0 {
			continue
		}
		distance := game.crateSoundDistance(index)
		if distance > farthest {
			retired = index
			farthest = distance
		}
	}
	if retired < 0 {
		return false
	}
	game.retireCrateSound(retired, frame)
	return true
}

func crateSoundOptions(volume float32) engine.SoundEmitterOptions {
	return engine.SoundEmitterOptions{Volume: volume, NearDistance: .5, FarDistance: 9, Loop: true, Occlusion: true}
}

func (game *Game) setCrateSoundVolume(index int, volume float32) {
	// Stop fades use host wall time. Catch-up updates can exhaust an admission
	// slot before the nominal frame grace has elapsed in real time. Repeating
	// the active upsert retries that admission without rewinding an existing
	// player, even when movement gain is unchanged. Idle silence needs one update.
	if volume == 0 && game.audio.volume[index] == 0 {
		return
	}
	game.audio.volume[index] = volume
	game.audio.crates[index].SetOptions(crateSoundOptions(volume))
}

// Fixed arrays avoid per-update sorting allocations. Select the closest moving
// crates when a cascade exceeds the live voice budget; ties keep source order.
func (game *Game) closestMovingCrates(volume [len(freightStacks)]float32) [len(freightStacks)]bool {
	var selected [len(freightStacks)]bool
	for range maxMovingCrateSounds {
		closest := -1
		distance := math.Inf(1)
		for index, crate := range game.crates {
			if crate == nil || volume[index] == 0 || selected[index] {
				continue
			}
			square := game.crateSoundDistance(index)
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

func (game *Game) crateSoundDistance(index int) float64 {
	var receiver engine.Vec3
	if game.camera != nil {
		receiver = game.camera.Settings().Position
	}
	p := game.crates[index].Transform().Position
	dx, dy, dz := float64(p.X-receiver.X), float64(p.Y-receiver.Y), float64(p.Z-receiver.Z)
	return dx*dx + dy*dy + dz*dz
}

func (game *Game) stopAudio() {
	for _, emitter := range game.audio.ambient {
		if emitter != nil {
			emitter.Stop()
		}
	}
	for _, emitter := range game.audio.crates {
		if emitter != nil {
			emitter.Stop()
		}
	}
	game.audio = hangarAudio{}
	game.ClearAudioListener()
}
