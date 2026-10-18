package persistence

import "testing"

func TestProfileKeepsPreferencesAndDistinctEndings(t *testing.T) {
	p := DefaultProfile()
	p.Music = false
	for _, id := range []string{"truth", "truth", "kindness", "timetable"} {
		p.Unlock(id)
	}
	restored, err := ReadProfile(p.Save())
	if err != nil || restored.Music || !restored.Ambience || restored.Count() != 3 {
		t.Fatal(restored, err)
	}
	if restored.Unlock("unknown") || restored.Unlock("truth") {
		t.Fatal("unknown or repeated ending changed unlocks")
	}
	for _, data := range [][]byte{nil, []byte("LTP2"), []byte("LTP1\x04\x00"), []byte("LTP1\x00\x08")} {
		if _, err := ReadProfile(data); err == nil {
			t.Fatal("accepted invalid sample profile")
		}
	}
}
