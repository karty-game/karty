// Package dialogue runs compiled Yarn stories and provides visual-novel presentation state.
package dialogue

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const MaxHistory = 24

type Entry struct {
	ID      string
	Speaker string
	Text    string
}
type Option struct {
	ID      int
	Text    string
	Enabled bool
}

// Model belongs to the game. UI components render it and invoke game callbacks.
// Full lines are the default; Reveal enables optional transient text pacing.
type Model struct {
	VisibleText          string
	Revealing            bool
	Paused               bool
	revealAt, revealWait int
	Speaker              string
	Text                 string
	Scene                string
	Portrait             string
	Location             string
	Status               string
	Ending               string
	Choices              []Option
	History              []Entry
	ShowHistory          bool
	Done                 bool
	Music                bool
	Ambience             bool
}

func New() *Model {
	return &Model{Scene: "platform", Portrait: "", Location: "NORTHBOUND · 23:47", Music: true, Ambience: true}
}
func (model *Model) Line(id, text string) error {
	if !validText(text, 4096) || !validText(id, 256) {
		return fmt.Errorf("dialogue line exceeds 4096 bytes")
	}
	speaker, body, found := strings.Cut(text, ":")
	if !found {
		speaker = ""
		body = text
	}
	if !validText(strings.TrimSpace(speaker), 128) {
		return fmt.Errorf("dialogue speaker exceeds 128 bytes")
	}
	model.Revealing, model.Paused = false, false
	model.VisibleText = ""
	model.Speaker = strings.TrimSpace(speaker)
	// Narration has no nameplate, including legacy authored/saved lines.
	if model.Speaker == "Narrator" {
		model.Speaker = ""
	}
	model.Text = strings.TrimSpace(body)
	model.Choices = nil
	model.History = append(model.History, Entry{id, model.Speaker, model.Text})
	if len(model.History) > MaxHistory {
		copy(model.History, model.History[len(model.History)-MaxHistory:])
		model.History = model.History[:MaxHistory]
	}
	return nil
}

// Narration distinguishes unvoiced prose from a character's spoken line.
func (model *Model) Narration() bool { return model.Speaker == "" || model.Speaker == "Narrator" }

func (model *Model) SetChoices(options []Option) error {
	if err := validateChoices(options); err != nil {
		return err
	}
	model.Choices = append(model.Choices[:0], options...)
	return nil
}

func (model *Model) Validate() error {
	if !validText(model.VisibleText, 4096) || !validText(model.Text, 4096) || !validText(model.Speaker, 128) ||
		!validText(model.Scene, 128) ||
		!validText(model.Portrait, 128) ||
		!validText(model.Location, 256) ||
		!validText(model.Status, 4096) ||
		!validText(model.Ending, 256) ||
		len(model.History) > MaxHistory {
		return fmt.Errorf("invalid dialogue presentation bounds")
	}
	for _, entry := range model.History {
		if !validText(entry.ID, 256) || !validText(entry.Text, 4096) || !validText(entry.Speaker, 128) {
			return fmt.Errorf("invalid history entry")
		}
	}
	return validateChoices(model.Choices)
}

func validText(text string, limit int) bool { return len(text) <= limit && utf8.ValidString(text) }

func validateChoices(options []Option) error {
	if len(options) > 32 {
		return fmt.Errorf("dialogue choice count exceeds 32")
	}
	seen := map[int]bool{}
	for _, option := range options {
		if option.ID < 0 || option.ID > 65535 || seen[option.ID] || !validText(option.Text, 4096) {
			return fmt.Errorf("invalid dialogue choice")
		}
		seen[option.ID] = true
	}
	return nil
}
