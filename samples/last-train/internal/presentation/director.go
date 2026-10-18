// Package presentation owns the sample's transient pacing, never save payloads.
package presentation

// State is projected into the UI. Cover is a cinematic curtain; Busy also
// includes its exit, during which story input must remain blocked.
type State struct {
	Title, Cover, Busy, Transition bool
	Chapter, Subtitle              string
	ticks                          int
	scene                          string
}

// Timings are fixed 60 Hz cartridge updates. The host curtain takes 360 ms;
// 24 updates leave margin before replacing the scene underneath it.
const coverTicks = 24
const cardTicks = 120
const uncoverTicks = 24

func (s *State) Opening() { *s = State{Title: true} }
func (s *State) Begin()   { *s = State{} }
func (s *State) Travel(scene, chapter, subtitle string) {
	*s = State{
		Cover:      true,
		Busy:       true,
		Transition: true,
		Chapter:    chapter,
		Subtitle:   subtitle,
		scene:      scene,
		ticks:      coverTicks + cardTicks + uncoverTicks,
	}
}
func (s *State) Pause(ticks int) { s.Busy = true; s.ticks = ticks; s.scene = "" }

// Step returns a scene only once the curtain covers it, then resumes the story
// only after the curtain has exited. Pauses resume without changing the scene.
func (s *State) Step() (scene string, resume bool) {
	if s.Title || !s.Busy {
		return "", false
	}
	s.ticks--
	if s.scene != "" && s.ticks == cardTicks+uncoverTicks {
		scene = s.scene
	}
	if s.scene != "" && s.ticks == uncoverTicks {
		s.Cover = false
	}
	if s.ticks <= 0 {
		s.Busy = false
		s.Cover = false
		s.Transition = false
		return scene, true
	}
	return scene, false
}
