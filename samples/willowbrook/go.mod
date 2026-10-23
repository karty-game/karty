module example.com/willowbrook

go 1.27.0

require github.com/karty-game/tiled v0.0.1

replace github.com/karty-game/tiled => ./.karty/packages/tiled

require github.com/karty-game/controllers-topdown v0.0.1

replace github.com/karty-game/controllers-topdown => ./.karty/packages/controllers-topdown
