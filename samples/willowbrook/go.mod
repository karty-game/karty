module example.com/willowbrook

go 1.27.0

require github.com/karty-game/tiled v0.1.0

replace github.com/karty-game/tiled => ./.karty/packages/tiled

require github.com/karty-game/controllers v0.1.0

replace github.com/karty-game/controllers => ./.karty/packages/controllers
