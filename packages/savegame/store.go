// Package savegame separates lifetime profile data from independently loadable
// run slots. All payloads are arbitrary bytes; games choose their own formats.
package savegame

import "errors"

var ErrInvalidSlot = errors.New("invalid save slot")

// Backend is implemented by Karty's generated Game. Payloads and backend errors
// pass through unchanged; this package adds no envelope or schema requirement.
type Backend interface {
	ReadSave(string) ([]byte, error)
	WriteSave(string, []byte) error
	DeleteSave(string) error
}

type Store struct{ Backend Backend }

func (s Store) ReadProfile() ([]byte, error)   { return s.Backend.ReadSave("profile") }
func (s Store) WriteProfile(data []byte) error { return s.Backend.WriteSave("profile", data) }

// DeleteProfile resets lifetime data only when the game explicitly requests it.
func (s Store) DeleteProfile() error { return s.Backend.DeleteSave("profile") }

func (s Store) ReadRun(slot string) ([]byte, error) {
	key, err := runKey(slot)
	if err != nil {
		return nil, err
	}
	return s.Backend.ReadSave(key)
}
func (s Store) WriteRun(slot string, data []byte) error {
	key, err := runKey(slot)
	if err != nil {
		return err
	}
	return s.Backend.WriteSave(key, data)
}

// DeleteRun never deletes the profile. Missing slots follow backend semantics.
func (s Store) DeleteRun(slot string) error {
	key, err := runKey(slot)
	if err != nil {
		return err
	}
	return s.Backend.DeleteSave(key)
}
func runKey(slot string) (string, error) {
	if len(slot) == 0 || len(slot) > 60 {
		return "", ErrInvalidSlot
	}
	for _, c := range []byte(slot) {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return "", ErrInvalidSlot
		}
	}
	return "run_" + slot, nil
}
