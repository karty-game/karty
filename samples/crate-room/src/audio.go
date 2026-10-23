package main

import (
	"math"

	"example.com/crate-room/.karty/assets"
	"example.com/crate-room/.karty/engine"
)

// Four short impact voices and four blast voices stay below the 16-emitter
// budget. Slots include clip duration and the host's 100 ms retirement fade.
type roomVoice struct {
	emitter   *engine.SoundEmitter
	remaining uint16
}

type roomAudio struct {
	impacts        [4]roomVoice
	blasts         [4]roomVoice
	previous       [crateCount]engine.Vec3
	velocity       [crateCount]engine.Vec3
	cooldown       [crateCount]uint8
	quiet, spacing uint8
}

func (audio *roomAudio) reset(crates *crateStack) {
	audio.stop()
	audio.quiet, audio.spacing = 45, 0
	audio.velocity = [crateCount]engine.Vec3{}
	audio.cooldown = [crateCount]uint8{}
	for i, body := range crates.bodies {
		audio.previous[i] = body.Position()
	}
}

func updateVoices(voices *[4]roomVoice) {
	for i := range voices {
		voice := &voices[i]
		if voice.remaining == 0 {
			continue
		}
		voice.remaining--
		if voice.remaining == 8 {
			voice.emitter.Stop()
		}
		if voice.remaining == 0 {
			voice.emitter = nil
		}
	}
}

func playRoomVoice(
	host *engine.Game,
	level engine.LevelHandle,
	voices *[4]roomVoice,
	sound engine.SoundID,
	position engine.Vec3,
	volume float32,
	ticks uint16,
) bool {
	for i := range voices {
		voice := &voices[i]
		if voice.remaining != 0 {
			continue
		}
		emitter := host.NewSoundEmitter(level, sound, position, engine.SoundEmitterOptions{
			Volume: volume, NearDistance: 1.5, FarDistance: 22, Occlusion: false,
		})
		if emitter == nil {
			return false
		}
		*voice = roomVoice{emitter: emitter, remaining: ticks + 8}
		return true
	}
	return false
}

func (audio *roomAudio) update(host *engine.Game, level engine.LevelHandle, camera *engine.Camera3D, crates *crateStack) {
	yaw := float64(camera.Settings().Yaw)
	host.SetAudioListener(camera.Ref(), engine.AudioListenerOptions{
		Right: engine.NewVec2(float32(math.Cos(yaw)), -float32(math.Sin(yaw))),
	})
	updateVoices(&audio.impacts)
	updateVoices(&audio.blasts)
	if audio.quiet > 0 {
		audio.quiet--
	}
	if audio.spacing > 0 {
		audio.spacing--
	}
	// Pick the strongest audible impact this tick, so a falling stack cannot
	// submit 80 thuds. Track every body even while muted or cooling down.
	best, gain, bestVolume := -1, float32(0), float32(0)
	for i, body := range crates.bodies {
		position := body.Position()
		before := audio.previous[i]
		velocity := engine.NewVec3((position.X-before.X)*60, (position.Y-before.Y)*60, (position.Z-before.Z)*60)
		loss := impactSpeed(audio.velocity[i], velocity)
		audio.previous[i], audio.velocity[i] = position, velocity
		if audio.cooldown[i] > 0 {
			audio.cooldown[i]--
			continue
		}
		if audio.quiet != 0 || audio.spacing != 0 || loss < 1.25 {
			continue
		}
		distance := distance3(position, camera.Settings().Position)
		volume := min(.5, (loss-1)*.075)
		priority := volume * max(0, 1-distance/22)
		if priority > gain {
			best, gain, bestVolume = i, priority, volume
		}
	}
	if best >= 0 && playRoomVoice(host, level, &audio.impacts, assets.SoundCrateHit, audio.previous[best], bestVolume, 16) {
		audio.cooldown[best], audio.spacing = 24, 8
	}
}

// Normal gravity and launches do not count as impacts. Contact is inferred
// from lost speed along the incoming direction; the SDK has no contact events.
func impactSpeed(previous, current engine.Vec3) float32 {
	previous.Z -= 9.81 / 60
	speed := float32(math.Sqrt(float64(previous.X*previous.X + previous.Y*previous.Y + previous.Z*previous.Z)))
	if speed < 1.25 {
		return 0
	}
	return ((previous.X-current.X)*previous.X + (previous.Y-current.Y)*previous.Y + (previous.Z-current.Z)*previous.Z) / speed
}

func distance3(a, b engine.Vec3) float32 {
	x, y, z := a.X-b.X, a.Y-b.Y, a.Z-b.Z
	return float32(math.Sqrt(float64(x*x + y*y + z*z)))
}

func (audio *roomAudio) stop() {
	for _, voices := range []*[4]roomVoice{&audio.impacts, &audio.blasts} {
		for i := range voices {
			if voices[i].emitter != nil {
				voices[i].emitter.Stop()
				voices[i].remaining = 8
			}
		}
	}
}
