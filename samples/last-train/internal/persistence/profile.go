// Package persistence owns Last Train's payload format, not the engine's.
package persistence

import "fmt"

// Profile survives starting or loading a run. Slot selection is transient UI state.
type Profile struct {
	Music, Ambience, ReducedMotion bool
	Unlocks                        byte
}
type Controls struct{ Slot, Endings, Motion string }

func DefaultProfile() Profile { return Profile{Music: true, Ambience: true} }
func (p Profile) Save() []byte {
	flags := byte(0)
	if p.Music {
		flags |= 1
	}
	if p.Ambience {
		flags |= 2
	}
	if p.ReducedMotion {
		flags |= 4
	}
	return []byte{'L', 'T', 'P', '2', flags, p.Unlocks}
}
func ReadProfile(data []byte) (Profile, error) {
	if len(data) != 6 || (string(data[:4]) != "LTP1" && string(data[:4]) != "LTP2") || data[4]&^7 != 0 || data[5]&^7 != 0 ||
		(string(data[:4]) == "LTP1" && data[4]&4 != 0) {
		return Profile{}, fmt.Errorf("unrecognized Last Train profile")
	}
	return Profile{Music: data[4]&1 != 0, Ambience: data[4]&2 != 0, ReducedMotion: data[4]&4 != 0, Unlocks: data[5]}, nil
}
func (p *Profile) Unlock(ending string) bool {
	before := p.Unlocks
	switch ending {
	case "truth":
		p.Unlocks |= 1
	case "kindness":
		p.Unlocks |= 2
	case "timetable":
		p.Unlocks |= 4
	}
	return p.Unlocks != before
}
func (p Profile) Count() int {
	count := 0
	for bits := p.Unlocks; bits != 0; bits >>= 1 {
		count += int(bits & 1)
	}
	return count
}
