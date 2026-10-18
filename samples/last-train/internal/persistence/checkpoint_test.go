package persistence

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestCheckpointPreservesAllAuthoredStateAndBounds(t *testing.T) {
	c := Checkpoint{[]byte("story"), []byte("view"), []byte("direction")}
	data, err := c.Save()
	if err != nil {
		t.Fatal(err)
	}
	r, err := ReadCheckpoint(data)
	if err != nil || !bytes.Equal(c.Story, r.Story) || !bytes.Equal(c.View, r.View) || !bytes.Equal(c.Direction, r.Direction) {
		t.Fatal(r, err)
	}
	for n := range len(data) {
		if _, err := ReadCheckpoint(data[:n]); err == nil {
			t.Fatalf("accepted truncated envelope at %d", n)
		}
	}
	malformed := append([]byte(nil), data...)
	binary.LittleEndian.PutUint32(malformed[4:], ^uint32(0))
	if _, err := ReadCheckpoint(malformed); err == nil {
		t.Fatal("accepted overflowing size")
	}
	if _, err := (Checkpoint{Story: make([]byte, 64*1024), View: []byte("v")}).Save(); err == nil {
		t.Fatal("accepted oversized checkpoint")
	}
}

func TestCheckpointFitsEngineStorageIncludingHeader(t *testing.T) {
	checkpoint := Checkpoint{Story: make([]byte, 32*1024), View: make([]byte, 16*1024-16)}
	data, err := checkpoint.Save()
	if err != nil || len(data) != 48*1024 {
		t.Fatal("exact engine boundary rejected", len(data), err)
	}
	checkpoint.View = append(checkpoint.View, 0)
	if _, err := checkpoint.Save(); err == nil {
		t.Fatal("accepted payload one byte above the engine limit")
	}
	checkpoint.View = make([]byte, 20*1024)
	if _, err := checkpoint.Save(); err == nil {
		t.Fatal("accepted the oversized checkpoint found in review")
	}
}

func TestLegacyCheckpointAndLifetimeMotionPreference(t *testing.T) {
	legacy := []byte("LTN1\x01\x00\x00\x00\x01\x00\x00\x00sv")
	c, err := ReadCheckpoint(legacy)
	if err != nil || string(c.Story) != "s" || string(c.View) != "v" || len(c.Direction) != 0 {
		t.Fatal(c, err)
	}
	p, err := ReadProfile([]byte("LTP1\x03\x05"))
	if err != nil || p.ReducedMotion || !p.Music || !p.Ambience || p.Count() != 2 {
		t.Fatal(p, err)
	}
	p.ReducedMotion = true
	r, err := ReadProfile(p.Save())
	if err != nil || r != p {
		t.Fatal(r, err)
	}
}
