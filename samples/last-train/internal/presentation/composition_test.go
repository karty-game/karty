package presentation

import "testing"

func TestCompositionRestoresAuthoredStateAndRejectsBadCues(t *testing.T) {
	c := DefaultComposition()
	c.Frame, c.Ensemble = "close", true
	if err := c.SetExpression("iris", "relieved"); err != nil {
		t.Fatal(err)
	}
	data, err := c.Save()
	if err != nil {
		t.Fatal(err)
	}
	r, err := ReadComposition(data)
	if err != nil || r != c {
		t.Fatal(r, err)
	}
	before := c
	if err := c.SetExpression("elias", "guarded"); err == nil || c != before {
		t.Fatal("invalid expression changed composition")
	}
	for _, input := range []string{`{"frame":"extreme"}`, `{"Mara":"unknown"}`, `{`} {
		if _, err := ReadComposition([]byte(input)); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
	if r, err := ReadComposition(nil); err != nil || r != DefaultComposition() {
		t.Fatal(r, err)
	}
	for _, pose := range [][2]string{{"mara", "resolute"}, {"elias", "reading"}, {"iris", "offering"}} {
		if err := c.SetExpression(pose[0], pose[1]); err != nil {
			t.Fatal(err)
		}
	}
	data, err = c.Save()
	if err != nil {
		t.Fatal(err)
	}
	if restored, err := ReadComposition(data); err != nil || restored != c {
		t.Fatal("alternate poses did not survive checkpoint", restored, err)
	}
}

func TestCompositionBinaryRejectsTruncationAndInvalidEnums(t *testing.T) {
	data, err := DefaultComposition().Save()
	if err != nil || len(data) != 9 {
		t.Fatal(data, err)
	}
	for i := 0; i < len(data); i++ {
		if i == 0 {
			continue
		}
		if _, err := ReadComposition(data[:i]); err == nil {
			t.Fatal("accepted truncation", i)
		}
	}
	for _, index := range []int{0, 4, 5, 6, 7, 8} {
		bad := append([]byte{}, data...)
		bad[index] = 255
		if _, err := ReadComposition(bad); err == nil {
			t.Fatal("accepted invalid enum", index)
		}
	}
	if _, err := ReadComposition(append(data, 0)); err == nil {
		t.Fatal("accepted trailing data")
	}
}
