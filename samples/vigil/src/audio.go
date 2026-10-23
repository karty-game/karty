package main

import (
	"math"

	"example.com/vigil/.karty/assets"
	"example.com/vigil/.karty/engine"
	"example.com/vigil/internal/dungeon"
)

// Eight effects and three shrine loops leave room for the host's stop fades
// within its sixteen spatial voices. The church bed uses the streaming lane.
type vigilAudio struct {
	game    *engine.Game
	level   engine.LevelHandle
	frame   uint64
	effects [8]vigilSound
	shrines [3]*engine.SoundEmitter
	steps   float64
}

type vigilSound struct {
	emitter *engine.SoundEmitter
	until   uint64
}

func (a *vigilAudio) create(shadow *engine.Sprite3D, camera *engine.Camera3D) {
	a.bindReceiver(shadow, camera)
	a.game.PlayMusic(assets.MusicChurchAmbience, engine.StreamOptions{Loop: true, CrossfadeMS: 1200})
}

func (a *vigilAudio) bindReceiver(shadow *engine.Sprite3D, camera *engine.Camera3D) {
	// The camera is above and behind the scene. Distance belongs to the
	// accepted physical player root; stereo belongs to the screen's right axis.
	// The painted hero has per-frame registration offsets, so use its shadow.
	if shadow != nil && camera != nil {
		yaw := float64(camera.Settings().Yaw)
		a.game.SetAudioListener(shadow.Ref(), engine.AudioListenerOptions{
			Right: engine.NewVec2(float32(math.Cos(yaw)), -float32(math.Sin(yaw))),
		})
	}
}

func (a *vigilAudio) update(travel float64, p dungeon.Point) {
	for i := range a.effects {
		voice := &a.effects[i]
		if voice.emitter != nil && a.frame >= voice.until {
			voice.emitter.Stop()
			voice.emitter = nil
			// Stop fades for 100 ms. Keep the slot reserved through that fade.
			voice.until = a.frame + 8
		}
	}
	// Only accepted travel counts, so pushing a wall is silent. Two contacts
	// per authored stride preserve cadence while walking or hastening.
	a.steps += travel
	if a.steps >= dungeon.WalkStride/2 {
		a.steps = math.Mod(a.steps, dungeon.WalkStride/2)
		a.play(assets.SoundStoneFootstep, p, .04, .18, .5, 6, 28)
	}
}

// Leave stopped slots reserved through their fade instead of admitting a new voice early.
func (a *vigilAudio) play(sound engine.SoundID, p dungeon.Point, z, volume, near, far float32, frames uint64) {
	for i := range a.effects {
		voice := &a.effects[i]
		if voice.emitter != nil || a.frame < voice.until {
			continue
		}
		voice.emitter = a.game.NewSoundEmitter(a.level, sound, engine.NewVec3(float32(p.X), float32(p.Y), z),
			engine.SoundEmitterOptions{Volume: volume, NearDistance: near, FarDistance: far, Occlusion: true})
		voice.until = a.frame + frames
		return
	}
}

func (a *vigilAudio) lightShrine(index int, p dungeon.Point) {
	a.play(assets.SoundShrineChime, p, 1.25, .42, 1, 14, 124)
	a.shrines[index] = a.game.NewSoundEmitter(a.level, assets.SoundEmberCrackle,
		engine.NewVec3(float32(p.X), float32(p.Y), 1.25),
		engine.SoundEmitterOptions{Volume: .2, NearDistance: .7, FarDistance: 7, Loop: true, Occlusion: true})
}

func (a *vigilAudio) reset() {
	for i := range a.effects {
		voice := &a.effects[i]
		if voice.emitter != nil {
			voice.emitter.Stop()
			voice.emitter = nil
			voice.until = a.frame + 8
		}
	}
	for i, emitter := range a.shrines {
		emitter.Stop()
		a.shrines[i] = nil
	}
	a.steps = 0
}

func (a *vigilAudio) stop() {
	a.reset()
	a.game.StopMusic(250)
	a.game.ClearAudioListener()
}
