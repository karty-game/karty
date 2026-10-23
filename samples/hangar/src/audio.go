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
	level engine.LevelHandle
	// The scene owns these solids; audio follows their accepted poses.
	sources       *[len(freightStacks)]*engine.Solid3D
	pending       bool
	ambient       [ambientSoundCount]*engine.SoundEmitter
	crates        [len(freightStacks)]*engine.SoundEmitter
	previous      [len(freightStacks)]engine.Vec3
	volume        [len(freightStacks)]float32
	lastMoved     [len(freightStacks)]uint64
	retiringUntil [len(freightStacks)]uint64
	frame         uint64
}

// Mount ambience now; crate voices are admitted only after accepted movement.
func (audio *hangarAudio) create(host *engine.Game, level engine.LevelHandle, sources *[len(freightStacks)]*engine.Solid3D) {
	audio.level, audio.sources = level, sources
	audio.ambient[0] = host.NewSoundEmitter(audio.level, assets.SoundReactorHum,
		engine.NewVec3(8, 21.7, 2.6), engine.SoundEmitterOptions{
			Volume: .8, NearDistance: 3, FarDistance: 28, Loop: true, Occlusion: true,
		})
	// The upper console bank uses the same recording and tuning as the lower
	// terminals. Its source sits in front of the authored console colliders.
	for index, position := range [...]engine.Vec3{
		engine.NewVec3(-13.7, 19, 3.2), engine.NewVec3(-13.7, 25, 3.2), engine.NewVec3(34.4, 11.85, 4.9),
	} {
		audio.ambient[index+1] = host.NewSoundEmitter(audio.level, assets.SoundComputerIdle, position,
			engine.SoundEmitterOptions{Volume: .7, NearDistance: 1, FarDistance: 9, Loop: true, Occlusion: true})
	}
	// Above the barrel rim and the acid surface, respectively, so the source
	// does not start inside the solid drum or below the authored liquid surface.
	audio.ambient[4] = host.NewSoundEmitter(audio.level, assets.SoundFireAmbience, engine.NewVec3(14.8, -1.55, 1.12),
		engine.SoundEmitterOptions{Volume: .25, NearDistance: 1, FarDistance: 8, Loop: true, Occlusion: true})
	// The south basin and both trench branches each have a quiet source above
	// the liquid's highest wave. Smaller gain and range keep overlap gentle.
	for index, position := range [...]engine.Vec3{
		engine.NewVec3(31, 18, -.95), engine.NewVec3(26.4, 29, -.95), engine.NewVec3(37, 29, -.95),
	} {
		audio.ambient[index+5] = host.NewSoundEmitter(audio.level, assets.SoundAcidAmbience, position,
			engine.SoundEmitterOptions{Volume: .16, NearDistance: 1.5, FarDistance: 10, Loop: true, Occlusion: true})
	}
	for index, crate := range audio.sources {
		if crate != nil {
			audio.previous[index] = crate.Transform().Position
		}
	}
}

// Keep panning current, but measure crate motion only when the frame advances.
func (audio *hangarAudio) update(host *engine.Game, camera *engine.Camera3D, frame uint64) {
	if camera != nil {
		yaw := float64(camera.Settings().Yaw)
		host.SetAudioListener(camera.Ref(), engine.AudioListenerOptions{
			Right: engine.NewVec2(float32(math.Cos(yaw)), -float32(math.Sin(yaw))),
		})
	}
	if audio.frame == 0 {
		audio.frame = frame
		return
	}
	if frame <= audio.frame {
		return
	}
	seconds := float64(frame-audio.frame) / 60
	audio.frame = frame
	if audio.sources == nil {
		return
	}
	audio.updateCrateSounds(host, camera, seconds, frame)
}

func (audio *hangarAudio) stop(host *engine.Game) {
	for _, emitter := range audio.ambient {
		if emitter != nil {
			emitter.Stop()
		}
	}
	for _, emitter := range audio.crates {
		if emitter != nil {
			emitter.Stop()
		}
	}
	*audio = hangarAudio{}
	host.ClearAudioListener()
}
