// Package stage renders the dialogue package's asset-independent stage through
// the released sprite/vector API. All animation stays in cartridge user land.
package stage

import (
	"fmt"

	"example.com/last-train/.karty/engine"
	"example.com/last-train/internal/art"
	"example.com/last-train/internal/presentation"
	"github.com/karty-game/dialogue"
)

const width, height = float32(1280), float32(720)

type sprite struct {
	entity      *engine.Sprite2D
	last        dialogue.StageTransform
	initialized bool
	asset       engine.LevelTextureRef
}

type particle struct {
	entity      *engine.Shape2D
	last        dialogue.StageParticle
	initialized bool
}

// Renderer draws the desktop stage in the project's 1280x720 design space.
// The matching TrainStage UI component covers this with a responsive portrait
// composition on narrow displays: this SDK does not expose viewport dimensions
// to cartridge code. Both paths resolve the same mounted texture owners.
type Renderer struct {
	Motion      *dialogue.Stage
	game        *engine.Game
	Art         *art.Loader
	background  sprite
	actors      [3]sprite
	rain        [36]particle
	expressions [3]string
}

var names = [3]string{"mara", "elias", "iris"}

func New(game *engine.Game, motion *dialogue.Stage, packs *art.Loader) *Renderer {
	if motion == nil {
		motion = dialogue.NewStage(names[:]...)
	}
	r := &Renderer{Motion: motion, game: game, Art: packs}
	for i := range r.rain {
		e := game.NewShape2D(engine.Line(engine.NewVec2(-5, 25)), engine.NewVec2(0, 0))
		e.SetStroke(engine.RGBA(190, 211, 230, 255), .8)
		e.SetLayer(-70)
		e.SetAlpha(0)
		r.rain[i].entity = e
	}
	r.Draw()
	return r
}

// SetExpression swaps an authored expression. Asset IDs are manifest-owned;
// defaults use the previously approved portraits.
func (r *Renderer) SetExpression(name, mood string) error {
	index := -1
	for i, n := range names {
		if n == name {
			index = i
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("unknown expression actor %q", name)
	}
	valid := mood == "neutral" || mood == "default" || mood == ""
	switch name {
	case "mara":
		valid = valid || mood == "worried" || mood == "relieved" || mood == "resolute"
	case "elias":
		valid = valid || mood == "wistful" || mood == "hopeful" || mood == "reading"
	case "iris":
		valid = valid || mood == "guarded" || mood == "relieved" || mood == "offering"
	}
	if !valid {
		return fmt.Errorf("unknown %s expression %q", name, mood)
	}
	if mood == "default" || mood == "neutral" {
		mood = ""
	}
	r.expressions[index] = mood
	return nil
}

func (r *Renderer) Expression(name string) string {
	for i, n := range names {
		if n == name {
			return r.expressions[i]
		}
	}
	return ""
}
func (r *Renderer) Framing() string            { return r.Motion.Snapshot().Framing }
func (r *Renderer) Ensemble() bool             { return r.Motion.Snapshot().Ensemble }
func (r *Renderer) ReducedMotion() bool        { return r.Motion.Snapshot().ReducedMotion }
func (r *Renderer) Gesture(name string) string { return r.Motion.ActiveGesture(name) }

// Step advances the stage once and draws it; Draw projects without advancing,
// useful immediately after restoring a checkpoint or toggling reduced motion.
func (r *Renderer) Step() { r.Motion.Step(); r.Draw() }
func (r *Renderer) Draw() {
	frame := r.Motion.Snapshot()
	scene := frame.Scene
	if scene == "" {
		scene = "platform"
	}
	r.background.apply(r.game, r.Art.Texture(scene), presentation.ArchitecturalBackground(frame.Background), 720, -100)
	for i, name := range names {
		texture := name
		if r.expressions[i] != "" {
			texture += "-" + r.expressions[i]
		}
		for a := 0; a < frame.ActorCount; a++ {
			if frame.Actors[a].Name == name {
				r.actors[i].apply(r.game, r.Art.Texture(texture), frame.Actors[a].StageTransform, 960, -60+int32(i))
				break
			}
		}
	}
	for i, p := range frame.Rain {
		r.rain[i].apply(p)
	}
}

func (s *sprite) apply(game *engine.Game, asset engine.LevelTextureRef, t dialogue.StageTransform, sourceHeight float32, layer int32) {
	if asset.Handle == 0 {
		return
	}
	if s.entity == nil {
		s.entity = game.NewLevelSprite2D(asset, engine.NewVec2(width/2, height/2))
		s.entity.SetLayer(layer)
		s.asset = asset
	}

	if s.asset != asset {
		s.entity.SetLevelTexture(asset)
		s.asset = asset
	}
	if !s.initialized || t.X != s.last.X || t.Y != s.last.Y || t.Height != s.last.Height || t.Rotation != s.last.Rotation {
		scale := t.Height * height / sourceHeight
		s.entity.SetTransform(engine.NewVec2(t.X*width, t.Y*height), engine.NewVec2(scale, scale), t.Rotation)
	}
	if !s.initialized || t.Alpha != s.last.Alpha {
		s.entity.SetAlpha(t.Alpha)
	}
	s.last = t
	s.initialized = true
}
func (p *particle) apply(next dialogue.StageParticle) {
	if p.initialized && next == p.last {
		return
	}
	if next.Alpha > 0 {
		scale := next.Height * height / 25
		p.entity.SetTransform(engine.NewVec2(next.X*width, next.Y*height), engine.NewVec2(scale, scale), 0)
	}
	if !p.initialized || next.Alpha != p.last.Alpha {
		p.entity.SetAlpha(next.Alpha)
	}
	p.last = next
	p.initialized = true
}

// Source supplies the owning pack's static one-image template to the phone UI.
func (r *Renderer) Source(name string) engine.UISource { return r.Art.Source(name) }
func (r *Renderer) Ready(name string) bool             { return r.Art.Ready(art.Owner(name)) }
func (r *Renderer) PortraitReady(name string) bool {
	mood := r.Expression(name)
	if mood != "" {
		name += "-" + mood
	}
	return r.Ready(name)
}
