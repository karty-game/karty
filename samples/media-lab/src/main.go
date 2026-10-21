package main

import (
	"example.com/media-lab/.karty/assets"
	"example.com/media-lab/.karty/engine"
	"example.com/media-lab/.karty/ui"
)

type Game struct {
	engine.Game

	hud            *engine.UI
	status         string
	musicEnabled   bool
	burstRemaining uint8
}

//go:wasmexport karty_register
func main() { game := &Game{}; engine.Run(game.Hooks()) }

func (game *Game) Initialize() {
	background := game.NewSprite2D(assets.TextureGalleryBackground, engine.NewVec2(480, 270))
	background.SetLayer(-20)

	checker := game.NewSprite2D(assets.TextureGalleryChecker, engine.NewVec2(132, 170))
	checker.SetLayer(-5)
	badge := game.NewSprite2D(assets.TextureGalleryBadge, engine.NewVec2(780, 170))
	badge.SetLayer(-5)

	shade := game.NewShape2D(engine.Rectangle(960, 540), engine.NewVec2(0, 0))
	shade.SetFill(engine.RGBA(7, 13, 30, 112))
	shade.SetLayer(-10)

	game.status = "Play a sound, start a burst, or replay the video."
	game.hud = ui.MediaHud(ui.MediaHUDProps{
		Status:       func() string { return game.status },
		MusicEnabled: func() bool { return game.musicEnabled },
		PlayVideo:    game.playVideo, StopVideo: game.stopVideo,
		OneShot: game.oneShot, Burst: game.startBurst, ToggleMusic: game.toggleMusic,
	})
}

func (game *Game) Hooks() engine.Hooks {
	return engine.Hooks{OnStart: game.Initialize, OnUpdate: game.advanceBurst,
		OnStop: game.Shutdown, OnPointerReleased: game.onPointerReleased}
}

func (game *Game) onPointerReleased(event engine.PointerEvent) {
	if event.Button != engine.PointerPrimary {
		return
	}
	// Keep the image itself interactive; the labelled controls are KartUI
	// buttons and never fall through into these game-coordinate hit regions.
	if event.Y >= 120 && event.Y < 340 && event.X >= 300 && event.X <= 660 {
		if event.X < 480 {
			game.playVideo()
		} else {
			game.stopVideo()
		}
	}
}

func (game *Game) playVideo() {
	game.PlayVideo(assets.VideoDemoPattern, 300, 130, 360, 180)
	game.setStatus("Playing MPEG-1 video with MP2 sound")
}

func (game *Game) stopVideo() {
	game.StopVideo()
	game.setStatus("Video stopped; press Play video to replay")
}

func (game *Game) oneShot() {
	game.PlaySound(assets.SoundEffectsClick)
	game.setStatus("Played one short 8-bit source through QOA")
}

func (game *Game) startBurst() {
	game.burstRemaining = 36
	game.setStatus("Burst started: overlapping the same QOA sample")
}

func (game *Game) setStatus(status string) {
	game.status = status
	game.hud.Invalidate()
}

func (game *Game) advanceBurst(engine.Frame) {
	// Two starts per frame deliberately exceed the host's 32-voice limit before
	// this relatively long effect finishes. The host must stay responsive.
	for starts := 0; starts < 2 && game.burstRemaining > 0; starts++ {
		game.PlaySound(assets.SoundEffectsMachineGun)
		game.burstRemaining--
	}
}

func (game *Game) toggleMusic() {
	game.musicEnabled = !game.musicEnabled
	if game.musicEnabled {
		game.PlayMusic(assets.MusicLabLoop, engine.StreamOptions{Loop: true, CrossfadeMS: 500})
		game.setStatus("BGM started with a host-owned streaming QOA loop")
		return
	}

	game.StopMusic(500)
	game.setStatus("BGM loop stopped")
}

func (game *Game) Shutdown() {}
