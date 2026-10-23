package main

// visit records the requested station even if its room still needs to load.
func (game *Game) visit(index int) {
	if index < 0 || index >= len(viewpoints) {
		return
	}
	view := viewpoints[index]
	game.room.view = index
	game.controls.hidden = view.hideControls
	if view.mode != game.room.mode {
		game.room.mode = view.mode
		game.restart()
		return
	}
	if game.room.level == 0 || game.room.release != 0 {
		return
	}
	game.camera.releasePointer(&game.Game)
	game.camera.visit(&game.Game, game.room.level, index)

	game.controls.setStatus(view.name)
}

func (game *Game) rotate(amount float32) {
	if game.room.release == 0 {
		game.camera.rotate(game.room.view, amount)
	}
}

func (game *Game) zoom(factor float32) {
	if game.room.release == 0 {
		game.camera.zoom(factor)
	}
}

func (game *Game) selectMode(mode int) {
	switch mode {
	case 0:
		game.selectView(combinedFireView)
	case 1:
		game.selectView(thinSmokeView)
	case 2:
		game.selectView(collectionView)
	}
}

// selectView ignores tab/fire selections during loading and defers panel changes until ready.
func (game *Game) selectView(view int) {
	if game.room.request != 0 || game.room.release != 0 {
		return
	}
	mode := viewpoints[view].mode
	if game.room.mode == mode {
		game.visit(view)
		return
	}
	game.room.mode, game.room.view = mode, view
	game.restart()
}

func (game *Game) selectFire(index int) {
	switch index {
	case 0:
		game.selectView(combinedFireView)
	case 1:
		game.selectView(separateFireView)
	case 2:
		game.selectView(fireComparisonView)
	}
}

func (game *Game) selectSmoke(index int) {
	switch index {
	case 0:
		game.visit(thinSmokeView)
	case 1:
		game.visit(rollingSmokeView)
	case 2:
		game.visit(smokeComparisonView)
	}
}
