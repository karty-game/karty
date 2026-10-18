package dialogue

import (
	"fmt"
	"math"
)

// StageTransform uses a viewport-relative centre and height. Rotation is radians.
// These transforms can be rendered through ordinary sprites without a camera API.
type StageTransform struct {
	X, Y, Height, Rotation, Alpha float32
}

// StageActor is one named actor's current presentation, including entrance/exit.
type StageActor struct {
	Name string
	StageTransform
}

// StageParticle is a viewport-relative ambient streak or light patch.
type StageParticle struct {
	X, Y, Width, Height, Alpha float32
}

// StageFrame is a bounded, allocation-free snapshot for a renderer. Actor names
// are identities in the script, not player-facing captions.
type StageFrame struct {
	Scene, Framing          string
	ReducedMotion, Ensemble bool
	Tick                    uint64
	Background              StageTransform
	Actors                  [8]StageActor
	ActorCount              int
	Rain                    [36]StageParticle
	Lights                  [2]StageParticle
}

type stageActorState struct {
	name        string
	visibility  float64
	gesture     string
	gestureTick int
}

// Stage animates authored visual-novel direction at 60 updates per second.
// It owns no assets, clock, renderer or save format. A zero-value Stage is valid;
// NewStage registers actors that may enter, leave or join an ensemble.
type Stage struct {
	actors                   [8]stageActorState
	count                    int
	scene, portrait, framing string
	ensemble, reduced        bool
	tick                     uint64
	zoom                     float64
}

// NewStage registers up to eight distinct actors. Empty and duplicate names are
// ignored. Scripts can use any names; Last Train's three passengers are sample data.
func NewStage(names ...string) *Stage {
	s := &Stage{framing: "wide", zoom: 1.035}
	for _, name := range names {
		if name == "" || s.actor(name) != nil || s.count == len(s.actors) {
			continue
		}
		s.actors[s.count].name = name
		s.count++
	}
	return s
}

func (s *Stage) actor(name string) *stageActorState {
	for i := 0; i < s.count; i++ {
		if s.actors[i].name == name {
			return &s.actors[i]
		}
	}
	return nil
}

// SetScene changes the ambient scene. Platform scenes use rain; other scenes
// use carriage sway and passing window lights. A repeated scene does not restart time.
func (s *Stage) SetScene(scene string) { s.scene = scene }

// SetPortrait changes focus. An empty name makes every actor leave. Unknown
// names are rejected so a script typo cannot silently remove the cast.
func (s *Stage) SetPortrait(name string) error {
	if name != "" && s.actor(name) == nil {
		return fmt.Errorf("unknown stage actor %q", name)
	}
	s.portrait = name
	if s.reduced {
		s.snap()
	}
	return nil
}

func (s *Stage) SetFraming(framing string) error {
	switch framing {
	case "wide", "medium", "close":
	default:
		return fmt.Errorf("unknown stage framing %q", framing)
	}
	s.framing = framing
	if s.reduced {
		s.snap()
	}
	return nil
}

func (s *Stage) SetEnsemble(enabled bool) {
	s.ensemble = enabled
	if s.reduced {
		s.snap()
	}
}

// SetReducedMotion immediately settles entrances/framing and removes continuous
// ambient and reaction motion. Toggling it never alters story progress.
func (s *Stage) SetReducedMotion(enabled bool) {
	s.reduced = enabled
	if enabled {
		s.snap()
	}
}

// Gesture starts one short, authored reaction, rather than continuously bouncing
// a speaking portrait. A reduced-motion reader sees the settled expression.
func (s *Stage) Gesture(name, gesture string) error {
	a := s.actor(name)
	if a == nil {
		return fmt.Errorf("unknown stage actor %q", name)
	}
	switch gesture {
	case "nod", "recoil", "reach":
	default:
		return fmt.Errorf("unknown stage gesture %q", gesture)
	}
	if !s.reduced {
		a.gesture, a.gestureTick = gesture, 0
	}
	return nil
}

func (s *Stage) visible(a *stageActorState) bool { return s.ensemble || a.name == s.portrait }
func (s *Stage) targetZoom() float64 {
	switch s.framing {
	case "medium":
		return 1.085
	case "close":
		return 1.16
	default:
		return 1.035
	}
}
func (s *Stage) snap() {
	s.zoom = s.targetZoom()
	for i := 0; i < s.count; i++ {
		a := &s.actors[i]
		a.visibility = 0
		if s.visible(a) {
			a.visibility = 1
		}
		a.gesture = ""
	}
}

// Step advances one fixed host update. Pause these updates while reading a log
// or menu if that game's presentation should freeze there.
func (s *Stage) Step() {
	s.tick++
	if s.reduced {
		return
	}
	if s.zoom == 0 {
		s.zoom = 1.035
	}
	s.zoom += (s.targetZoom() - s.zoom) * .035
	for i := 0; i < s.count; i++ {
		a := &s.actors[i]
		if s.visible(a) {
			a.visibility = math.Min(1, a.visibility+1.0/30)
		} else {
			a.visibility = math.Max(0, a.visibility-1.0/24)
		}
		if a.gesture != "" {
			a.gestureTick++
			if a.gestureTick >= 36 {
				a.gesture = ""
			}
		}
	}
}

// Snapshot computes presentation without advancing it. All coordinates and
// amplitudes are independent of screen pixels; a renderer chooses its viewport.
func (s *Stage) Snapshot() StageFrame {
	f := StageFrame{Scene: s.scene, Framing: s.framing, ReducedMotion: s.reduced, Ensemble: s.ensemble, Tick: s.tick, ActorCount: s.count}
	if f.Framing == "" {
		f.Framing = "wide"
	}
	zoom := s.zoom
	if zoom == 0 {
		zoom = 1.035
	}
	t := float64(s.tick) / 60
	driftX, driftY, sway := 0.0, 0.0, 0.0
	if !s.reduced {
		driftX = .004 * math.Sin(t*.19)
		driftY = .0025 * math.Sin(t*.13)
		if s.scene != "platform" {
			sway = .0009 * math.Sin(t*1.9)
		}
	}
	f.Background = StageTransform{
		X:        float32(.5 + driftX),
		Y:        float32(.5 + driftY),
		Height:   float32(zoom),
		Rotation: float32(sway),
		Alpha:    1,
	}
	for i := 0; i < s.count; i++ {
		a := &s.actors[i]
		x, y, h := .72, .53, .94
		if s.ensemble {
			x = .24 + float64(i)*.52/math.Max(1, float64(s.count-1))
			y = .55
			h = .86
		}
		focus := (zoom - 1.035) * .9
		h *= 1 + focus
		y += focus * .22
		v := a.visibility
		// Smoothstep keeps the entrance and exit from stopping abruptly.
		e := v * v * (3 - 2*v)
		x += (1 - e) * .065
		rot := 0.0
		if !s.reduced && v > 0 {
			y += .0013 * math.Sin(t*1.7+float64(i))
			rot = sway * .55
			if a.gesture != "" {
				beat := math.Sin(math.Pi * float64(a.gestureTick) / 36)
				switch a.gesture {
				case "nod":
					y += .011 * beat
				case "recoil":
					x += .024 * beat
					rot += .013 * beat
				case "reach":
					x -= .023 * beat
					y -= .004 * beat
				}
			}
		}
		f.Actors[i] = StageActor{
			Name:           a.name,
			StageTransform: StageTransform{X: float32(x), Y: float32(y), Height: float32(h), Rotation: float32(rot), Alpha: float32(e)},
		}
	}
	if s.reduced {
		return f
	}
	if s.scene == "platform" {
		for i := range f.Rain {
			seed := float64(i)
			f.Rain[i] = StageParticle{
				X:      float32(mod(seed*.6180339887-t*.11, 1.08) - .04),
				Y:      float32(mod(seed*.371+t*(.65+float64(i%5)*.04), 1.16) - .08),
				Width:  .004,
				Height: .025 + float32(i%4)*.005,
				Alpha:  .15 + float32(i%3)*.025,
			}
		}
	} else {
		// Small warm patches confined to the carriage windows. The long gaps
		// between passing lamps avoid a flashing or strobing effect.
		phase := mod(t, 8)
		if phase < 2.4 {
			alpha := float32(math.Sin(math.Pi*phase/2.4) * .085)
			f.Lights[0] = StageParticle{X: float32(.07 + phase*.075), Y: .31, Width: .025, Height: .22, Alpha: alpha}
			f.Lights[1] = StageParticle{X: float32(.66 + phase*.055), Y: .29, Width: .02, Height: .21, Alpha: alpha * .7}
		}
	}
	return f
}

func mod(value, period float64) float64 { return value - math.Floor(value/period)*period }

// Settle finishes transient entrances and framing, for example after restoring a
// checkpoint. It does not change the reader's reduced-motion preference.
func (s *Stage) Settle() { s.snap() }

// ActiveGesture exposes a transient reaction to renderers with discrete pose or
// transition support. It is empty after the beat finishes or in reduced motion.
func (s *Stage) ActiveGesture(name string) string {
	if a := s.actor(name); a != nil {
		return a.gesture
	}
	return ""
}
