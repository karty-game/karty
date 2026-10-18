package dialogue

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"
)

func TestBinaryStringTablePreservesTextAndRejectsMalformedRecords(t *testing.T) {
	lines := []Line{
		{ID: "line:1", Text: "Mara: \"Ticket, please.\"\nÀ bientôt!", Tags: []string{"pause:4", "portrait:mara"}},
		{ID: "line:2", Text: ""},
	}
	data, err := EncodeStrings(lines)
	if err != nil {
		t.Fatal(err)
	}
	got, err := loadStrings(data)
	if err != nil || !reflect.DeepEqual(got["line:1"], lines[0]) {
		t.Fatal(got, err)
	}
	again, err := EncodeStrings(lines)
	if err != nil || !bytes.Equal(data, again) {
		t.Fatal("nondeterministic string table", err)
	}
	for i := 0; i < len(data); i++ {
		if _, err := loadStrings(data[:i]); err == nil {
			t.Fatalf("accepted truncation %d", i)
		}
	}
	for _, bad := range [][]byte{append(append([]byte{}, data...), 0), []byte("id,text,tags\nline:1,old,\n")} {
		if _, err := loadStrings(bad); err == nil {
			t.Fatal("accepted trailing or legacy input")
		}
	}
	bad := append([]byte{}, data...)
	binary.LittleEndian.PutUint32(bad[8:], ^uint32(0))
	if _, err := loadStrings(bad); err == nil {
		t.Fatal("accepted hostile row count")
	}
	for _, badLines := range [][]Line{{lines[1], lines[0]}, {lines[0], lines[0]}, {{ID: "bad", Text: string([]byte{255})}}} {
		if _, err := EncodeStrings(badLines); err == nil {
			t.Fatal("accepted invalid string table")
		}
	}
}
func TestBinaryRunTruncationNeverChangesLivePresentation(t *testing.T) {
	model := New()
	model.Text = "saved"
	model.History = []Entry{{ID: "one", Text: "comma, quote\" and newline\n"}}
	data, err := model.SaveRun()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(data); i++ {
		live := New()
		live.Text = "live"
		if err := live.RestoreRun(data[:i]); err == nil || live.Text != "live" {
			t.Fatalf("partial record %d changed live state", i)
		}
	}
	if err := model.Restore(data); err == nil {
		t.Fatal("run record accepted as full presentation")
	}
}
func TestBinaryStoryRejectsEveryTruncationAndTrailingByte(t *testing.T) {
	story := NewStory(fixture(t))
	if err := story.Start("Start"); err != nil {
		t.Fatal(err)
	}
	data, err := story.Save()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(data); i++ {
		if err := story.Restore(data[:i]); err == nil {
			t.Fatalf("accepted partial story %d", i)
		}
	}
	if err := story.Restore(append(append([]byte{}, data...), 0)); err == nil {
		t.Fatal("accepted trailing byte")
	}
	after, err := story.Save()
	if err != nil || !bytes.Equal(after, data) {
		t.Fatal("rejected state changed the story", err)
	}
}
func FuzzBinaryDialogueRecords(f *testing.F) {
	model := New()
	data, _ := model.SaveRun()
	f.Add(data)
	f.Add([]byte("YSS1,old"))
	f.Add([]byte("KST1"))
	f.Fuzz(func(t *testing.T, data []byte) {
		m := New()
		_ = m.RestoreRun(data)
		_, _ = loadStrings(data)
		_, _ = unmarshalSnapshot(data)
	})
}
