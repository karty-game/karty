package savegame

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

var missing = errors.New("missing")
var quota = errors.New("quota")

type memory struct {
	values map[string][]byte
	fail   error
	calls  int
}

func (m *memory) ReadSave(key string) ([]byte, error) {
	m.calls++
	if data, ok := m.values[key]; ok {
		return append([]byte(nil), data...), nil
	}
	return nil, missing
}
func (m *memory) WriteSave(key string, data []byte) error {
	m.calls++
	if m.fail != nil {
		return m.fail
	}
	m.values[key] = append([]byte(nil), data...)
	return nil
}
func (m *memory) DeleteSave(key string) error { m.calls++; delete(m.values, key); return nil }

func TestIndependentLifetimesAndArbitraryPayloads(t *testing.T) {
	backend := &memory{values: map[string][]byte{}}
	store := Store{Backend: backend}
	// No encoding, schema, version, UTF-8 or nonempty-payload requirement.
	profile := []byte{0, 255, 1, 3, 0}
	if err := store.WriteProfile(profile); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteRun("1", []byte("arbitrary game data")); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteRun("2", nil); err != nil {
		t.Fatal(err)
	}
	first, err := store.ReadRun("1")
	if err != nil || string(first) != "arbitrary game data" {
		t.Fatal(first, err)
	}
	second, err := store.ReadRun("2")
	if err != nil || len(second) != 0 {
		t.Fatal(second, err)
	}
	if err := store.DeleteRun("1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadRun("1"); err != missing {
		t.Fatal(err)
	}
	p, err := store.ReadProfile()
	if err != nil || !bytes.Equal(p, profile) {
		t.Fatal(p, err)
	}
	backend.fail = quota
	if err := store.WriteProfile([]byte("new")); err != quota || !bytes.Equal(profile, backend.values["profile"]) {
		t.Fatal("failed write replaced profile", err)
	}
	if err := store.WriteRun("2", []byte("new")); err != quota || len(backend.values["run_2"]) != 0 {
		t.Fatal("failed write replaced run", err)
	}
	if err := store.DeleteProfile(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadProfile(); err != missing {
		t.Fatal(err)
	}
	if _, err := store.ReadRun("2"); err != nil {
		t.Fatal("profile deletion removed run", err)
	}
}

func TestSlotsCannotAliasProfileOrOtherKeys(t *testing.T) {
	backend := &memory{values: map[string][]byte{}}
	store := Store{Backend: backend}
	for _, slot := range []string{"", "../profile", "é", strings.Repeat("a", 61)} {
		if _, err := store.ReadRun(slot); err != ErrInvalidSlot {
			t.Fatal(slot, err)
		}
		if err := store.WriteRun(slot, nil); err != ErrInvalidSlot {
			t.Fatal(slot, err)
		}
		if err := store.DeleteRun(slot); err != ErrInvalidSlot {
			t.Fatal(slot, err)
		}
	}
	if backend.calls != 0 {
		t.Fatal("invalid request reached backend")
	}
	if err := store.WriteRun("profile", []byte("run")); err != nil {
		t.Fatal(err)
	}
	if _, ok := backend.values["profile"]; ok {
		t.Fatal("run aliased profile")
	}
	if err := store.WriteRun(strings.Repeat("a", 60), nil); err != nil {
		t.Fatal(err)
	}
	// Bounds and backend availability remain backend policy, not schema checks.
	backend.fail = quota
	if err := store.WriteProfile(make([]byte, 48*1024+1)); err != quota {
		t.Fatal(err)
	}
}
