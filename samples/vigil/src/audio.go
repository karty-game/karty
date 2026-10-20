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
	effects [8]vigilSound
	shrines [3]*engine.SoundEmitter
	steps   float64
}

type vigilSound struct {
	emitter *engine.SoundEmitter
	until   uint64
}

func (g *Game) createAudio() {
	g.bindAudioReceiver()
	g.PlayMusic(assets.MusicChurchAmbience, engine.StreamOptions{Loop: true, CrossfadeMS: 1200})
}

func (g *Game) bindAudioReceiver() {
	// The camera is above and behind the scene. Distance belongs to the
	// accepted physical player root; stereo belongs to the screen's right axis.
	// The painted hero has per-frame registration offsets, so use its shadow.
	if g.shadow != nil && g.camera != nil {
		yaw := float64(g.camera.Settings().Yaw)
		g.SetAudioListener(g.shadow.Ref(), engine.AudioListenerOptions{
			Right: engine.NewVec2(float32(math.Cos(yaw)), -float32(math.Sin(yaw))),
		})
	}
}

func (g *Game) updateAudio(travel float64, p dungeon.Point) {
	for i := range g.audio.effects {
		voice := &g.audio.effects[i]
		if voice.emitter != nil && g.frame >= voice.until {
			voice.emitter.Stop()
			voice.emitter = nil
			// Stop fades for 100 ms. Keep the slot reserved through that fade.
			voice.until = g.frame + 8
		}
	}
	// Only accepted travel counts, so pushing a wall is silent. Two contacts
	// per authored stride preserve cadence while walking or hastening.
	g.audio.steps += travel
	if g.audio.steps >= dungeon.WalkStride/2 {
		g.audio.steps = math.Mod(g.audio.steps, dungeon.WalkStride/2)
		g.playWorldSound(assets.SoundStoneFootstep, p, .04, .18, .5, 6, 28)
	}
}

func (g *Game) playWorldSound(sound engine.SoundID, p dungeon.Point, z, volume, near, far float32, frames uint64) {
	for i := range g.audio.effects {
		voice := &g.audio.effects[i]
		if voice.emitter != nil || g.frame < voice.until {
			continue
		}
		voice.emitter = g.NewSoundEmitter(g.level, sound, engine.NewVec3(float32(p.X), float32(p.Y), z),
			engine.SoundEmitterOptions{Volume: volume, NearDistance: near, FarDistance: far, Occlusion: true})
		voice.until = g.frame + frames
		return
	}
}

func (g *Game) lightShrineAudio(index int, p dungeon.Point) {
	g.playWorldSound(assets.SoundShrineChime, p, 1.25, .42, 1, 14, 124)
	g.audio.shrines[index] = g.NewSoundEmitter(g.level, assets.SoundEmberCrackle,
		engine.NewVec3(float32(p.X), float32(p.Y), 1.25),
		engine.SoundEmitterOptions{Volume: .2, NearDistance: .7, FarDistance: 7, Loop: true, Occlusion: true})
}

func (g *Game) resetAudio() {
	for i := range g.audio.effects {
		voice := &g.audio.effects[i]
		if voice.emitter != nil {
			voice.emitter.Stop()
			voice.emitter = nil
			voice.until = g.frame + 8
		}
	}
	for i, emitter := range g.audio.shrines {
		emitter.Stop()
		g.audio.shrines[i] = nil
	}
	g.audio.steps = 0
}

func (g *Game) stopAudio() {
	g.resetAudio()
	g.StopMusic(250)
	g.ClearAudioListener()
}
