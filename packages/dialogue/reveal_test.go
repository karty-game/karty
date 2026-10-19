package dialogue

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestWordRevealKeepsCompleteTokensAndFinalSpacing(t *testing.T) {
	m := New()
	if err := m.Line("prose", "Narrator: A reservation: carriage three, and an ordinary ticket home."); err != nil {
		t.Fatal(err)
	}
	if m.Speaker != "" || !m.Narration() || m.History[0].Speaker != "" {
		t.Fatal("narration leaked a speaker name")
	}
	m.Reveal()
	previous := ""
	for n := 0; n < 1000 && m.Revealing; n++ {
		m.StepReveal()
		visible := m.DisplayWords()
		if !strings.HasPrefix(m.Text, visible) || !strings.HasPrefix(visible, previous) {
			t.Fatal("reveal reordered completed words")
		}
		if m.Revealing && visible != "" && !strings.ContainsAny(visible[len(visible)-1:], " \t\n") {
			t.Fatal("revealed a partial token without its final spacing")
		}
		previous = visible
	}
	if m.DisplayWords() != m.Text || m.AdvanceLabel() != "▾" {
		t.Fatal("completed line or advance indicator incorrect")
	}
	m.Reveal()
	if m.AdvanceLabel() != "»" {
		t.Fatal("reveal indicator incorrect")
	}
	m.Paused = true
	if m.AdvanceLabel() != "…" {
		t.Fatal("pause indicator incorrect")
	}
}

func TestRevealIsUnicodeSafeSkippableAndTransient(t *testing.T) {
	m := New()
	if err := m.Line("letter", "Iris: Élias… お帰り。"); err != nil {
		t.Fatal(err)
	}
	m.Reveal()
	sawPartial := false
	for n := 0; n < 200 && m.Revealing; n++ {
		m.StepReveal()
		if !utf8.ValidString(m.DisplayText()) {
			t.Fatal("split a character")
		}
		sawPartial = sawPartial || m.DisplayText() != "" && m.DisplayText() != m.Text
	}
	if !sawPartial || m.DisplayText() != m.Text || m.Revealing {
		t.Fatal(m)
	}
	m.Reveal()
	m.StepReveal()
	data, err := m.SaveRun()
	if err != nil {
		t.Fatal(err)
	}
	restored := New()
	if err := restored.RestoreRun(data); err != nil {
		t.Fatal(err)
	}
	if restored.Revealing || restored.DisplayText() != m.Text {
		t.Fatal("saved an incomplete line")
	}
	m.FinishReveal()
	if m.Revealing || m.DisplayText() != m.Text {
		t.Fatal("skip did not finish line")
	}
}

func TestCharacterRevealExposesEveryUnicodePrefixWithoutWaitingForWords(t *testing.T) {
	m := New()
	if err := m.Line("characters", "Iris: Café 東京."); err != nil {
		t.Fatal(err)
	}
	m.Reveal()
	previous := ""
	count := 0
	for frame := 0; frame < 200 && m.Revealing; frame++ {
		if !m.StepReveal() {
			continue
		}
		count++
		visible := m.DisplayText()
		if !strings.HasPrefix(visible, previous) || utf8.RuneCountInString(visible) != count || m.RevealCharacters() != uint32(count) {
			t.Fatal("typewriter skipped characters or split UTF-8", previous, visible, count, m.RevealCharacters())
		}
		if count == 1 && visible != "C" {
			t.Fatal("first character waited for a complete word", visible)
		}
		previous = visible
	}
	if count != len([]rune(m.Text)) || m.Revealing || m.DisplayText() != m.Text {
		t.Fatal("character reveal failed to complete", count, m)
	}
	m.Reveal()
	if m.RevealCharacters() != 0 {
		t.Fatal("new reveal retained characters")
	}
	m.StepReveal()
	m.FinishReveal()
	if m.RevealCharacters() != uint32(utf8.RuneCountInString(m.Text)) {
		t.Fatal("skip left part of a line hidden")
	}
}
