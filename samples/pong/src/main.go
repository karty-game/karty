package main

import "example.com/pong/.karty/engine"

// Game represents the main game structure, containing it's general state and entities.
type Game struct {
	engine.Game

	player                    *engine.EntitySprite2D
	entities                  []*engine.EntitySprite2D
	label                     *engine.EntityText2D
	marker                    *engine.EntityVector2D
	left, right               bool
	leftRequest, rightRequest engine.LevelRequestID
	leftHandle, rightHandle   engine.LevelHandle
	levelStage                uint8
}

// Entry point of the game application
//
//go:wasmexport karty_register
func main() {
	engine.Run(&Game{})
}

// Initialize is called when the game is first started to set up the initial state.
func (game *Game) Initialize() {
	game.player = game.NewSprite2D(engine.TextureSpritesPlayer, engine.NewVec2D(48, 48))
	panel := game.NewVector2D(engine.Rectangle(360, 56), engine.NewVec2D(16, 216))
	panel.SetFill(engine.RGBA(25, 45, 80, 255))
	panel.SetLayer(-1)
	game.label = game.NewText2D("Arrow keys move; click to place", engine.NewVec2D(28, 232))
	game.label.SetSize(20)
	game.marker = game.NewVector2D(engine.Circle(12), engine.NewVec2D(304, 80))
	game.marker.SetFill(engine.RGBA(255, 170, 60, 180))
	game.marker.SetStroke(engine.ColorWhite, 2)
	axis := game.NewVector2D(engine.Line(engine.NewVec2D(72, 24)), engine.NewVec2D(256, 120))
	axis.SetStroke(engine.RGBA(100, 220, 255, 255), 3)

	for i := 0; i < 10; i++ {
		game.entities = append(game.entities, game.NewSprite2D(engine.TextureSpritesPlayer, engine.NewVec2D(float32(i*16), 64+float32(i*16))))
	}
	game.leftRequest = game.RequestLevel("levels.left")
}

// Update is called once per frame to update the game state based on input and elapsed time.
func (game *Game) Update(frame engine.Frame) {
	for _, event := range frame.Events {
		switch event.Type {
		case engine.EventKeyDown, engine.EventKeyUp:
			down := event.Type == engine.EventKeyDown
			if event.Key == engine.KeyLeft {
				game.left = down
			}
			if event.Key == engine.KeyRight {
				game.right = down
			}
		case engine.EventPointerUp:
			game.player.Translate(event.X-game.player.Position.X, event.Y-game.player.Position.Y)
			game.label.SetText("Player moved to pointer")
			game.marker.Translate(event.X-game.marker.Position.X, event.Y-game.marker.Position.Y)
		case engine.EventLevelReady:
			game.levelReady(event)
		case engine.EventLevelData:
			game.label.SetText("Level data: " + string(event.Bytes))
		case engine.EventLevelReleased:
			if game.levelStage == 2 && event.Handle == uint32(game.leftHandle) {
				game.leftRequest = game.RequestLevel("levels.left")
				game.levelStage = 3
			}
		case engine.EventLevelFailed:
			game.label.SetText("Level failed: " + event.Diagnostic)
		}
	}
	if game.left {
		game.player.Translate(-2, 0)
	}
	if game.right {
		game.player.Translate(2, 0)
	}

	// Text and vector entities move through the same Transform2d as sprites.
	// Reverse every two seconds at the host's fixed 60 updates per second.
	direction := float32(1)
	if frame.Number%240 >= 120 {
		direction = -1
	}
	game.label.Translate(0.25*direction, 0)
	game.marker.Translate(0.75*direction, 0.25*direction)

	for i, entity := range game.entities {
		if ((frame.Number+uint64(i)*10)/500)%2 == 0 {
			entity.Translate(1, 0)
		} else {
			entity.Translate(-1, 0)
		}
	}
}

func (game *Game) levelReady(event engine.InputEvent) {
	switch {
	case game.levelStage == 0 && event.RequestID == uint32(game.leftRequest):
		game.leftHandle = engine.LevelHandle(event.Handle)
		game.ReadLevelData(game.leftHandle, "arena", 0, 1024)
		// Prepare B while A remains mounted and usable.
		game.rightRequest = game.RequestLevel("levels.right")
		game.levelStage = 1
	case game.levelStage == 1 && event.RequestID == uint32(game.rightRequest):
		game.rightHandle = engine.LevelHandle(event.Handle)
		game.ReadLevelData(game.rightHandle, "arena", 0, 1024)
		game.ReleaseLevel(game.leftHandle)
		game.levelStage = 2
	case game.levelStage == 3 && event.RequestID == uint32(game.leftRequest):
		game.leftHandle = engine.LevelHandle(event.Handle)
		game.ReadLevelData(game.leftHandle, "arena", 0, 1024)
		game.label.SetText("Level A remounted; client kept running")
		game.levelStage = 4
	}
}

// Shutdown is called when the game is about to exit, allowing for cleanup of resources.
func (game *Game) Shutdown() {}
