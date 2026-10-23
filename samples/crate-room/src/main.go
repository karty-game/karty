package main

import (
	"example.com/crate-room/.karty/assets"
	"example.com/crate-room/.karty/config"
	"example.com/crate-room/.karty/engine"
)

const width, height = float32(config.ResolutionWidth), float32(config.ResolutionHeight)

// A centered control stays visible when the full-frame view crops its sides.
const shootX, shootY = width / 2, height * .8125

type Game struct {
	engine.Game
	level  engine.LevelHandle
	player roomPlayer
	crates crateStack
	hud    roomHUD
}

//go:wasmexport karty_register
func main() {
	game := &Game{}
	engine.Run(game.Hooks())
}

func (game *Game) Hooks() engine.Hooks {
	return engine.Hooks{
		OnStart:       game.start,
		OnUpdate:      game.update,
		OnKeyDown:     func(key engine.Key) { game.player.input.key(key, true) },
		OnKeyUp:       func(key engine.Key) { game.player.input.key(key, false) },
		OnPointer:     func(pointer engine.PointerInput) { game.player.input.pointer(&game.Game, pointer) },
		OnInputReset:  game.player.input.reset,
		OnLevelFailed: func(event engine.LevelFailed) { game.hud.setStatus("Level failed: " + event.Diagnostic) },
	}
}

func (game *Game) start() {
	game.hud.show(&game.Game, func() { game.player.input.pendingBlast = true }, func() { game.player.input.pendingReset = true })
	game.LoadLevel(assets.LevelCrateRoom, game.ready)
}

// ready builds physics objects only after their authored world is mounted.
func (game *Game) ready(event engine.LevelReady) {
	game.level = event.Handle
	if err := game.World(game.level).
		Configure(engine.WorldSettings{Mode: engine.WorldSpatial, Gravity: engine.Vec3{Z: -9.81}}); err != nil {
		panic(err)
	}
	game.player.create(&game.Game, game.level)
	game.crates.create(&game.Game, game.level)
	game.hud.refresh(game.player.gun.shots, game.player.gun.blasts)
}

// update handles reset before movement or queued shots can affect the new round.
func (game *Game) update(engine.Frame) {
	if game.player.camera == nil {
		return
	}
	if game.player.input.pendingReset {
		game.player.input.pendingReset = false
		game.reset()
		return
	}
	game.player.gun.update(&game.player.input, game.fire)
}

// fire keeps the world impulse and displayed shot counters in the same action.
func (game *Game) fire(blast bool) {
	game.player.gun.fire(&game.Game, game.level, game.player.camera, blast)
	game.hud.refresh(game.player.gun.shots, game.player.gun.blasts)
}

// reset restores the original stacks without reloading the level or resetting the shot cooldown.
func (game *Game) reset() {
	game.player.input.reset()
	game.player.gun.shots, game.player.gun.blasts = 0, 0
	game.player.reset()
	game.crates.reset(&game.Game, game.level)
	game.hud.refresh(game.player.gun.shots, game.player.gun.blasts)
}
