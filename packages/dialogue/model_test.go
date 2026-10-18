package dialogue

import (
	"strconv"
	"strings"
	"testing"
)

func TestPresentationRetainsRecentLinesAndChoiceState(t *testing.T) {
	model := New()
	for n := 0; n < 40; n++ {
		if err := model.Line(strconv.Itoa(n), "Iris: Train: northbound."); err != nil {
			t.Fatal(err)
		}
	}
	if model.Speaker != "Iris" || model.Text != "Train: northbound." || len(model.History) != MaxHistory || model.History[0].ID != "16" {
		t.Fatal(model)
	}
	if err := model.SetChoices([]Option{{ID: 7, Text: "Ask", Enabled: true}, {ID: 8, Text: "Wait", Enabled: false}}); err != nil {
		t.Fatal(err)
	}
	if err := model.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := model.Line("too-large", strings.Repeat("x", 4097)); err == nil || len(model.Choices) != 2 {
		t.Fatal("invalid line replaced presentation")
	}
}

func TestCheckpointRestoresPresentationAndRejectsCorruption(t *testing.T) {
	model := New()
	if err := model.Line("letter", "Iris: A letter, with a \"promise\".\nPlease read it."); err != nil {
		t.Fatal(err)
	}
	model.Scene, model.Portrait = "dining", "iris"
	if err := model.SetChoices([]Option{{ID: 9, Text: "Read, then ask", Enabled: true}}); err != nil {
		t.Fatal(err)
	}
	data, err := model.Save()
	if err != nil {
		t.Fatal(err)
	}
	restored := New()
	if err := restored.Restore(data); err != nil {
		t.Fatal(err)
	}
	if restored.Text != model.Text || restored.Portrait != "iris" || len(restored.Choices) != 1 || restored.Choices[0].ID != 9 ||
		len(restored.History) != 1 {
		t.Fatal(restored)
	}
	before := restored.Text
	if err := restored.Restore([]byte("DIALOGUE1,broken\n")); err == nil || restored.Text != before {
		t.Fatal("bad checkpoint replaced presentation")
	}
	for _, corrupt := range []func(*Model){
		func(m *Model) { m.Status = strings.Repeat("x", 4097) },
		func(m *Model) { m.Text = string([]byte{0xff}) },
		func(m *Model) { m.Choices = []Option{{ID: 1}, {ID: 1}} },
	} {
		invalid := *restored
		corrupt(&invalid)
		if _, err := invalid.Save(); err == nil {
			t.Fatal("accepted presentation that cannot safely reach UI")
		}
	}
}

func TestRunRestoreKeepsLifetimePreferences(t *testing.T) {
	original := New()
	original.Text, original.Status = "saved line", "transient status"
	original.Music, original.Ambience = true, true
	for _, save := range []func() ([]byte, error){original.SaveRun, original.Save} {
		data, err := save()
		if err != nil {
			t.Fatal(err)
		}
		current := New()
		current.Music, current.Ambience = false, false
		if err := current.RestoreRun(data); err != nil {
			t.Fatal(err)
		}
		if current.Music || current.Ambience || current.Text != "saved line" || current.Status != "" {
			t.Fatal(current)
		}
		if err := current.RestoreRun([]byte("DIALOGUE-RUN1,broken")); err == nil || current.Text != "saved line" || current.Music {
			t.Fatal("invalid run mutated current state")
		}
	}
}

func TestLegacyNarrationCheckpointRestoresWithoutNameplates(t *testing.T) {
	m := New()
	m.Speaker, m.Text = "Narrator", "In your pocket: one ordinary ticket."
	m.History = []Entry{{ID: "ticket", Speaker: "Narrator", Text: m.Text}}
	data, err := m.SaveRun()
	if err != nil {
		t.Fatal(err)
	}
	restored := New()
	if err := restored.RestoreRun(data); err != nil {
		t.Fatal(err)
	}
	if restored.Speaker != "" || !restored.Narration() || restored.History[0].Speaker != "" || restored.Text != m.Text {
		t.Fatal("legacy narration leaked a name or lost its colon")
	}
}
