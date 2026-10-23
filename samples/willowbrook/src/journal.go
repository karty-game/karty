package main

import "example.com/willowbrook/internal/village"

func (q *treasureQuest) close(player *villagePlayer, hud *villageHUD) {
	q.journalOpen = false
	q.update(player, hud)
}

// Inspect arrivals first; inspection clears the target before refreshing this journal.
func (q *treasureQuest) update(player *villagePlayer, hud *villageHUD) {
	if player.inspectTarget >= 0 && village.CanInteract(player.position, village.QuestStations[player.inspectTarget]) {
		q.inspect(player.inspectTarget, player, hud)
		return
	}
	text := q.journalText(player.position, player.inspectTarget)
	if text != q.text || q.journalOpen != q.lastOpen {
		q.text = text
		q.lastOpen = q.journalOpen
		hud.refresh()
	}
}

// journalText chooses the next objective, nearby action or open journal page.
func (q *treasureQuest) journalText(position village.Point, inspectTarget int) string {
	text := "Tap sign / chest | WASD: walk | R: reset"
	label := [4]string{"JOURNAL: FIND THE COTTAGE SIGN", "JOURNAL: FIND THE GARDEN SIGN", "JOURNAL: FIND THE HIDDEN CHEST", "TREASURE FOUND! TAP FOR JOURNAL"}[q.stage]
	nearby := q.nearby(position)
	if nearby >= 0 {
		if nearby < 2 {
			text = "E / SPACE / TAP HERE: READ SIGN"
		} else if q.stage == 3 {
			text = "TAP HERE: VIEW REWARD   R: reset"
		} else {
			text = "E / SPACE / TAP HERE: OPEN CHEST"
		}
	} else if inspectTarget >= 0 {
		text = "Walking to sign...  WASD / tap: cancel"
		if inspectTarget == 2 {
			text = "Walking to chest... WASD / tap: cancel"
		}
	}
	if q.journalOpen {
		label = q.message
		text = "E / SPACE / TAP HERE: CLOSE\nTap a sign or chest to inspect it"
	}
	return label + "\n" + text
}

// The journal consumes pointer input before world picking and stays sharp
// independently of game-image resolution.
func (q *treasureQuest) activate(loaded bool, player *villagePlayer, hud *villageHUD) {
	if !loaded {
		return
	}
	player.controller.CancelTarget()
	player.clearRoute()
	if q.journalOpen {
		q.close(player, hud)
	} else if q.nearby(player.position) >= 0 {
		q.inspectNearby(player, hud)
	} else {
		q.journalOpen = true
		q.update(player, hud)
	}
}
