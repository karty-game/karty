module example.com/last-train

go 1.27.0

require (
	github.com/karty-game/dialogue v0.1.0
	github.com/karty-game/savegame v0.1.0
)

replace github.com/karty-game/dialogue => ./.karty/packages/dialogue

replace github.com/karty-game/savegame => ./.karty/packages/savegame
