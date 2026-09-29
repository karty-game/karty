package main

import (
	"example.com/media-lab/.karty/assets"
	"example.com/media-lab/.karty/engine"
)

type Game struct {
	engine.Game

	status         *engine.EntityText2D
	musicLabel     *engine.EntityText2D
	musicEnabled   bool
	burstRemaining uint8
}

//go:wasmexport karty_register
func main() { engine.Run(&Game{}) }

func (game *Game) Initialize() {
	background := game.NewSprite2D(assets.TextureGalleryBackground, engine.NewVec2D(480, 270))
	background.SetLayer(-20)

	checker := game.NewSprite2D(assets.TextureGalleryChecker, engine.NewVec2D(132, 170))
	checker.SetLayer(-5)
	badge := game.NewSprite2D(assets.TextureGalleryBadge, engine.NewVec2D(780, 170))
	badge.SetLayer(-5)

	shade := game.NewVector2D(engine.Rectangle(960, 540), engine.NewVec2D(0, 0))
	shade.SetFill(engine.RGBA(7, 13, 30, 112))
	shade.SetLayer(-10)

	title := game.NewText2D("Karty media lab", engine.NewVec2D(330, 42))
	title.SetSize(34)
	title.SetLayer(10)
	formats := game.NewText2D("Images: QOI  |  Sound: QOA  |  Video: MPEG-1", engine.NewVec2D(246, 96))
	formats.SetSize(18)
	formats.SetColor(engine.RGBA(210, 226, 255, 255))
	formats.SetLayer(10)

	videoControls := game.NewText2D("VIDEO: left = play/replay | right = stop", engine.NewVec2D(310, 318))
	videoControls.SetSize(15)
	videoControls.SetLayer(12)
	game.status = game.NewText2D("Video: click center-left to play, center-right to stop", engine.NewVec2D(244, 350))
	game.status.SetSize(18)
	game.status.SetLayer(10)

	game.addButton(20, engine.RGBA(39, 100, 150, 235), "ONE SHOT", "8-bit mono WAV")
	game.addButton(333, engine.RGBA(150, 62, 65, 235), "36-START BURST", "24-bit mono WAV")
	game.addButton(646, engine.RGBA(75, 94, 156, 235), "BGM REPLAY: OFF", "16-bit stereo WAV / 4 sec")
	game.musicLabel = game.NewText2D("BGM REPLAY: OFF", engine.NewVec2D(680, 425))
	game.musicLabel.SetSize(22)
	game.musicLabel.SetLayer(12)
}

func (game *Game) addButton(x float32, color engine.Color, title, subtitle string) {
	panel := game.NewVector2D(engine.Rectangle(294, 112), engine.NewVec2D(x, 408))
	panel.SetFill(color)
	panel.SetStroke(engine.RGBA(205, 225, 255, 255), 2)
	panel.SetLayer(8)

	if title != "BGM REPLAY: OFF" {
		label := game.NewText2D(title, engine.NewVec2D(x+34, 425))
		label.SetSize(22)
		label.SetLayer(12)
	}
	detail := game.NewText2D(subtitle, engine.NewVec2D(x+34, 468))
	detail.SetSize(15)
	detail.SetColor(engine.RGBA(220, 232, 255, 255))
	detail.SetLayer(12)
}

func (game *Game) Update(frame engine.Frame) {
	for _, event := range frame.Events {
		if event.Type == engine.EventPointerUp && event.Y >= 120 && event.Y < 340 && event.X >= 300 && event.X <= 660 {
			if event.X < 480 {
				game.PlayVideo(assets.VideoDemoPattern, 300, 130, 360, 180)
				game.status.SetText("Playing MPEG-1 + MP2; click right of center to stop")
			} else {
				game.StopVideo()
				game.status.SetText("Video stopped; click left of center to replay")
			}
			continue
		}
		if event.Type != engine.EventPointerUp || event.Y < 400 {
			continue
		}

		switch {
		case event.X < 320:
			game.PlaySound(assets.SoundEffectsClick)
			game.status.SetText("Played one short 8-bit source through QOA")
		case event.X < 640:
			game.burstRemaining = 36
			game.status.SetText("Burst started: overlapping the same QOA sample")
		default:
			game.toggleMusic()
		}
	}

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
		game.musicLabel.SetText("BGM REPLAY: ON")
		game.status.SetText("BGM started with a host-owned streaming QOA loop")
		return
	}

	game.StopMusic(500)
	game.musicLabel.SetText("BGM REPLAY: OFF")
	game.status.SetText("BGM loop stopped")
}

func (game *Game) Shutdown() {}
