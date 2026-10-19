package dialogue

import (
	"strings"
	"unicode/utf8"
)

// Reveal begins a player-skippable text reveal. Saved runs contain the complete
// line; restoring one shows it immediately rather than replaying its animation.
func (model *Model) Reveal() {
	model.VisibleText = ""
	model.Revealing = model.Text != ""
	model.revealAt, model.revealWait = 0, 1
}

// StepReveal advances one 60 Hz update, with a short breath at punctuation.
func (model *Model) StepReveal() bool {
	if !model.Revealing {
		return false
	}
	if model.revealAt >= len(model.Text) {
		model.FinishReveal()
		return true
	}
	model.revealWait--
	if model.revealWait > 0 {
		return false
	}
	r, size := utf8.DecodeRuneInString(model.Text[model.revealAt:])
	model.revealAt += size
	model.VisibleText = model.Text[:model.revealAt]
	model.revealWait = 2
	switch r {
	case '.', '!', '?':
		model.revealWait = 12
	case ',', ';', ':', '—':
		model.revealWait = 6
	}
	if model.revealAt >= len(model.Text) {
		model.Revealing = false
	}
	return true
}
func (model *Model) FinishReveal() { model.VisibleText = model.Text; model.Revealing = false }
func (model *Model) DisplayText() string {
	if model.Revealing {
		return model.VisibleText
	}
	return model.Text
}

// RevealCharacters returns the visible Unicode scalar count for a label whose
// text remains the complete line. Keeping layout text fixed allows the renderer
// to reveal glyphs without moving words or changing text alignment.
func (model *Model) RevealCharacters() uint32 {
	return uint32(utf8.RuneCountInString(model.DisplayText()))
}

// DisplayWords reveals complete words with their trailing whitespace. Wrapping
// then uses each word's final width, so a growing fragment cannot jump lines.
// The character cursor still controls pacing and punctuation pauses.
func (model *Model) DisplayWords() string {
	if !model.Revealing {
		return model.Text
	}
	at := strings.LastIndexAny(model.VisibleText, " \t\n")
	if at < 0 {
		return ""
	}
	return model.VisibleText[:at+1]
}
func (model *Model) AdvanceLabel() string {
	if model.Paused {
		return "…"
	}
	if model.Revealing {
		return "»"
	}
	return "▾"
}
