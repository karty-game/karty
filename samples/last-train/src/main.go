package main

import (
	"fmt"
	"strconv"
	"strings"

	"example.com/last-train/.karty/assets"
	"example.com/last-train/.karty/engine"
	"example.com/last-train/.karty/processed"
	"example.com/last-train/.karty/ui"
	"example.com/last-train/internal/art"
	"example.com/last-train/internal/persistence"
	"example.com/last-train/internal/presentation"
	"example.com/last-train/internal/stage"
	"github.com/karty-game/dialogue"
	"github.com/karty-game/savegame"
)

type game struct {
	engine.Game
	program      *dialogue.Program
	story        *dialogue.Story
	model        *dialogue.Model
	screen       *engine.UI
	saves        savegame.Store
	profile      persistence.Profile
	profileError error
	controls     persistence.Controls
	stage        presentation.State
	motion       *dialogue.Stage
	visual       *stage.Renderer
	composition  presentation.Composition
	audioWarmup  uint16
	art          *art.Loader
	pendingArt   *pendingArt
}

func (g *game) start() {
	// Let the opening's first draws prepare its images and fonts before audio
	// starts. Otherwise those initial uploads can starve WebAudio on slow hosts.
	g.audioWarmup = 120
	g.model = dialogue.New()
	g.saves = savegame.Store{Backend: &g.Game}
	g.profile = persistence.DefaultProfile()
	g.controls.Slot = "1"
	g.motion = dialogue.NewStage("mara", "elias", "iris")
	g.art = art.New(&g.Game)
	g.art.Request("opening")
	g.visual = stage.New(&g.Game, g.motion, g.art)
	if data, err := g.saves.ReadProfile(); err == nil {
		profile, decodeErr := persistence.ReadProfile(data)
		if decodeErr == nil {
			g.profile = profile
		} else {
			g.profileError = decodeErr
		}
	} else if err != engine.SaveNotFound {
		g.profileError = err
	}
	g.syncProfile()
	g.screen = ui.GameScreen(
		ui.TrainScreenProps{
			Model:    g.model,
			Controls: &g.controls,
			Stage:    &g.stage,
			Visual:   g.visual,
			Motion:   g.toggleMotion,
			Begin:    g.begin,
			Slot:     g.nextSlot,
			Advance:  g.advance,
			Choose:   g.choose,
			Save:     g.save,
			Load:     g.load,
			Log:      g.log,
			Music:    g.music,
			Ambience: g.ambience,
			Restart:  g.restart,
		},
	)
	compiled, ok := processed.Read("dialogue.compile", "story.yarnc")
	if !ok {
		g.fail(fmt.Errorf("compiled dialogue is missing"))
		return
	}
	table, ok := processed.Read("dialogue.compile", "strings.bin")
	if !ok {
		g.fail(fmt.Errorf("dialogue strings are missing"))
		return
	}
	var err error
	g.program, err = dialogue.Load(compiled, table)
	if err != nil {
		g.fail(err)
		return
	}
	g.restart()
	if g.profileError != nil {
		g.model.Status = "Could not read profile: " + g.profileError.Error()
		g.invalidate()
	}
}

func (g *game) restart() {
	if g.program == nil {
		return
	}
	g.pendingArt = nil
	g.art.CancelConclusion()
	*g.model = *dialogue.New()
	g.syncProfile()
	g.story = dialogue.NewStory(g.program)
	if err := g.story.Start("Start"); err != nil {
		g.fail(err)
		return
	}
	g.composition = presentation.DefaultComposition()
	g.restoreDirection()
	g.stage.Opening()
	g.applyAudio()
	g.invalidate()
}

func (g *game) begin() {
	if !g.stage.Title {
		return
	}
	if g.retryArt() {
		return
	}
	g.waitForArt([]string{"opening"}, func() {
		g.stage.Begin()
		g.model.Status = ""
		g.advance()
	})
}
func (g *game) update(_ engine.Frame) {
	waiting := g.updateArt()
	if waiting {
		return
	}
	if g.audioWarmup > 0 {
		g.audioWarmup--
		if g.audioWarmup == 0 {
			g.applyAudio()
		}
	}
	if g.model.ShowHistory {
		return
	}
	g.visual.Step()
	if g.stage.Title {
		return
	}
	if g.stage.Busy {
		scene, resume := g.stage.Step()
		if scene != "" {
			_ = g.setScene(scene)
		}
		g.model.Paused = g.stage.Busy
		g.invalidate()
		if resume {
			g.advance()
		}
		return
	}
	if g.model.StepReveal() {
		g.invalidate()
	}
}
func (g *game) advance() {
	if g.retryArt() {
		return
	}
	if g.stage.Title || g.stage.Busy || g.story == nil || g.model.Done || g.model.ShowHistory || len(g.model.Choices) > 0 {
		return
	}
	if g.model.Revealing {
		g.model.FinishReveal()
		g.invalidate()
		return
	}
	for count := 0; count < 32; count++ {
		event, err := g.story.Continue()
		if err != nil {
			g.fail(err)
			return
		}
		switch event.Kind {
		case "line":
			if err := g.model.Line(event.Line.ID, g.presentSpeaker(event.Line.Text)); err != nil {
				g.fail(err)
				return
			}
			g.model.Reveal()
			g.invalidate()
			return
		case "choices":
			options := make([]dialogue.Option, 0, len(event.Choices))
			available := false
			for _, choice := range event.Choices {
				options = append(options, dialogue.Option{ID: choice.ID, Text: choice.Line.Text, Enabled: choice.Enabled})
				available = available || choice.Enabled
			}
			if !available {
				if err := g.story.Choose(-1); err != nil {
					g.fail(err)
					return
				}
				continue
			}
			g.waitForArt([]string{"opening", "investigation"}, func() {
				if err := g.model.SetChoices(options); err != nil {
					g.fail(err)
				}
				g.invalidate()
			})
			return
		case "command":
			packs := g.commandPacks(event.Command)
			if !g.art.Ready(packs...) {
				command := event.Command
				g.waitForArt(packs, func() {
					if err := g.command(command); err != nil {
						g.fail(err)
						return
					}
					if !g.stage.Busy {
						g.advance()
					}
				})
				return
			}
			if err := g.command(event.Command); err != nil {
				g.fail(err)
				return
			}
			if g.stage.Busy {
				g.model.Paused = true
				g.invalidate()
				return
			}
		case "complete":
			g.model.Done = true
			g.model.Status = "End of the line · Restart to try another journey."
			if err := g.persistProfile(); err != nil {
				g.model.Status += " Profile not saved: " + err.Error()
			}
			g.invalidate()
			return
		}
	}
	g.fail(fmt.Errorf("too many dialogue commands without a line"))
}

func (g *game) presentSpeaker(line string) string {
	name, body, found := strings.Cut(line, ":")
	if !found {
		return line
	}
	aliases := map[string]string{"Mara": "Conductor", "Elias": "Older passenger", "Iris": "Traveller"}
	alias, character := aliases[name]
	if !character {
		return line
	}
	known, ok := g.story.Variable("$knows_" + strings.ToLower(name))
	if ok && known.Bool {
		return line
	}
	return alias + ":" + body
}

func (g *game) choose(id int) {
	if g.pendingArt != nil || g.stage.Title || g.stage.Busy || g.story == nil || g.model.ShowHistory {
		return
	}
	if err := g.story.Choose(id); err != nil {
		g.fail(err)
		return
	}
	g.model.Choices = nil
	g.advance()
}

func (g *game) command(text string) error {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return fmt.Errorf("empty story command")
	}
	switch parts[0] {
	case "scene":
		if len(parts) != 2 {
			return fmt.Errorf("scene command needs a location")
		}
		return g.setScene(parts[1])
	case "travel":
		if len(parts) != 2 {
			return fmt.Errorf("travel command needs a location")
		}
		var chapter, subtitle string
		switch parts[1] {
		case "carriage":
			chapter, subtitle = "I · THE LAST NORTHBOUND", "A warm carriage. An unexpected request."
		case "dining":
			chapter, subtitle = "II · THE LAST LIT LAMP", "Twelve minutes to decide what matters."
		case "platform":
			chapter, subtitle = "NORTH QUAY", "Some promises outlive the journey."
		default:
			return fmt.Errorf("unknown destination %s", parts[1])
		}
		g.stage.Travel(parts[1], chapter, subtitle)
	case "pause":
		if len(parts) != 2 {
			return fmt.Errorf("pause needs a duration in updates")
		}
		ticks, err := strconv.Atoi(parts[1])
		if err != nil || ticks < 1 || ticks > 120 {
			return fmt.Errorf("invalid pause duration")
		}
		g.stage.Pause(ticks)
	case "portrait":
		if len(parts) != 2 {
			return fmt.Errorf("portrait command needs a character")
		}
		switch parts[1] {
		case "mara", "elias", "iris":
			g.model.Portrait = parts[1]
		case "none":
			g.model.Portrait = ""
		default:
			return fmt.Errorf("unknown portrait %s", parts[1])
		}
		if err := g.motion.SetPortrait(g.model.Portrait); err != nil {
			return err
		}
	case "frame":
		if len(parts) != 2 {
			return fmt.Errorf("frame needs a composition")
		}
		if err := g.motion.SetFraming(parts[1]); err != nil {
			return err
		}
		g.composition.Frame = parts[1]
	case "ensemble":
		if len(parts) != 2 || parts[1] != "on" && parts[1] != "off" {
			return fmt.Errorf("ensemble needs on or off")
		}
		g.composition.Ensemble = parts[1] == "on"
		g.motion.SetEnsemble(g.composition.Ensemble)
	case "expression":
		if len(parts) != 3 {
			return fmt.Errorf("expression needs a character and mood")
		}
		if err := g.composition.SetExpression(parts[1], parts[2]); err != nil {
			return err
		}
		return g.visual.SetExpression(parts[1], parts[2])
	case "gesture":
		if len(parts) != 3 {
			return fmt.Errorf("gesture needs a character and reaction")
		}
		return g.motion.Gesture(parts[1], parts[2])
	case "ending":
		if len(parts) != 2 {
			return fmt.Errorf("ending command needs an ID")
		}
		g.model.Ending = parts[1]
		if g.profile.Unlock(parts[1]) {
			g.syncProfile()
			if err := g.persistProfile(); err != nil {
				g.model.Status = "Could not save profile: " + err.Error()
			}
		}
	case "punch":
		if g.model.Ambience {
			g.PlaySound(assets.SoundTicketPunch)
		}
	case "whistle":
		if g.model.Ambience {
			g.PlaySound(assets.SoundStationWhistle)
		}
	default:
		return fmt.Errorf("unhandled story command %q", text)
	}
	return nil
}

func (g *game) setScene(scene string) error {
	switch scene {
	case "carriage":
		g.model.Location = "NORTHBOUND · THE PASSENGER CARRIAGE"
	case "dining":
		g.model.Location = "NORTHBOUND · THE LAST LIT LAMP"
	case "platform":
		g.model.Location = "NORTH QUAY · BEFORE THE FINAL STOP"
		if !g.stage.Busy {
			g.model.Location = "WESTBRIDGE · 23:47"
		}
	default:
		return fmt.Errorf("unknown scene %s", scene)
	}
	g.model.Scene = scene
	g.model.Portrait = ""
	g.composition.Frame, g.composition.Ensemble = "wide", false
	g.motion.SetScene(scene)
	_ = g.motion.SetPortrait("")
	_ = g.motion.SetFraming("wide")
	g.motion.SetEnsemble(false)
	g.motion.Settle()
	g.visual.Draw()
	g.applyAudio()
	return nil
}

func (g *game) save() {
	if g.pendingArt != nil || g.stage.Title || g.stage.Busy || g.story == nil {
		return
	}
	state, err := g.story.Save()
	if err != nil {
		g.fail(err)
		return
	}
	view, err := g.model.SaveRun()
	if err == nil {
		direction, directionErr := g.composition.Save()
		err = directionErr
		if err == nil {
			data, encodeErr := (persistence.Checkpoint{Story: state, View: view, Direction: direction}).Save()
			err = encodeErr
			if err == nil {
				err = g.saves.WriteRun(g.controls.Slot, data)
			}
		}
	}
	if err != nil {
		g.model.Status = "Could not save: " + err.Error()
	} else {
		g.model.Status = "Checkpoint saved · Slot " + g.controls.Slot + " · You can close and reopen the game."
	}
	g.invalidate()
}

func (g *game) load() {
	if g.stage.Busy || g.program == nil {
		return
	}
	data, err := g.saves.ReadRun(g.controls.Slot)
	// The original sample used one checkpoint. Retain read compatibility in slot 1.
	if err == engine.SaveNotFound && g.controls.Slot == "1" {
		data, err = g.ReadSave("checkpoint")
	}
	if err == engine.SaveNotFound {
		g.model.Status = "No checkpoint in slot " + g.controls.Slot + ". Choose Save to keep this moment."
		g.invalidate()
		return
	}
	if err != nil {
		g.model.Status = "Could not load: " + err.Error()
		g.invalidate()
		return
	}
	checkpoint, err := persistence.ReadCheckpoint(data)
	if err != nil {
		g.model.Status = "The checkpoint is incomplete or incompatible."
		g.invalidate()
		return
	}
	composition, err := presentation.ReadComposition(checkpoint.Direction)
	if err != nil {
		g.model.Status = "The checkpoint direction is invalid."
		g.invalidate()
		return
	}
	view := dialogue.New()
	view.Music, view.Ambience = g.profile.Music, g.profile.Ambience
	if err := view.RestoreRun(checkpoint.View); err != nil {
		g.model.Status = "The checkpoint presentation is invalid."
		g.invalidate()
		return
	}
	restored := dialogue.NewStory(g.program)
	if err = restored.Restore(checkpoint.Story); err != nil {
		g.model.Status = "Could not restore: " + err.Error()
		g.invalidate()
		return
	}
	if view.Scene == "carriage" {
		g.art.Request("conclusion")
	}
	g.waitForArt(checkpointPacks(view, composition), func() {
		g.stage.Begin()
		g.story = restored
		*g.model = *view
		g.composition = composition
		g.restoreDirection()
		g.model.ShowHistory = false
		g.model.Status = "Checkpoint restored · Slot " + g.controls.Slot + "."
		if g.profile.Unlock(g.model.Ending) {
			g.syncProfile()
			if err := g.persistProfile(); err != nil {
				g.model.Status += " Profile not saved: " + err.Error()
			}
		}
		g.applyAudio()
		g.invalidate()
	})
}

func (g *game) applyAudio() {
	if g.audioWarmup > 0 {
		return
	}
	if g.model.Music {
		g.PlayMusic(assets.MusicLastCarriage, engine.StreamOptions{Loop: true, CrossfadeMS: 400})
	} else {
		g.StopMusic(250)
	}
	if g.model.Ambience && g.model.Scene != "platform" {
		g.PlayEnvironment(engine.EnvironmentLane0, assets.EnvironmentNightTrain, engine.StreamOptions{Loop: true, CrossfadeMS: 400})
	} else {
		g.StopEnvironment(engine.EnvironmentLane0, 250)
	}
}
func (g *game) music() {
	g.profile.Music = !g.profile.Music
	g.syncProfile()
	g.applyAudio()
	if g.model.Music {
		g.model.Status = "Music on"
	} else {
		g.model.Status = "Music off"
	}
	if err := g.persistProfile(); err != nil {
		g.model.Status += " · Could not save profile: " + err.Error()
	}
	g.invalidate()
}
func (g *game) ambience() {
	g.profile.Ambience = !g.profile.Ambience
	g.syncProfile()
	g.applyAudio()
	if g.model.Ambience {
		g.model.Status = "Train ambience on"
	} else {
		g.model.Status = "Train ambience off"
	}
	if err := g.persistProfile(); err != nil {
		g.model.Status += " · Could not save profile: " + err.Error()
	}
	g.invalidate()
}

// Profile writes never contain story state; run loads never restore preferences.
func (g *game) syncProfile() {
	g.model.Music, g.model.Ambience = g.profile.Music, g.profile.Ambience
	g.controls.Endings = strconv.Itoa(g.profile.Count())
	g.controls.Motion = "Motion: Full"
	if g.profile.ReducedMotion {
		g.controls.Motion = "Motion: Reduced"
	}
	if g.motion != nil {
		g.motion.SetReducedMotion(g.profile.ReducedMotion)
	}
}
func (g *game) restoreDirection() {
	g.motion.SetScene(g.model.Scene)
	_ = g.motion.SetPortrait(g.model.Portrait)
	_ = g.motion.SetFraming(g.composition.Frame)
	g.motion.SetEnsemble(g.composition.Ensemble)
	g.motion.SetReducedMotion(g.profile.ReducedMotion)
	_ = g.visual.SetExpression("mara", g.composition.Mara)
	_ = g.visual.SetExpression("elias", g.composition.Elias)
	_ = g.visual.SetExpression("iris", g.composition.Iris)
	g.motion.Settle()
	g.visual.Draw()
}
func (g *game) toggleMotion() {
	g.profile.ReducedMotion = !g.profile.ReducedMotion
	g.syncProfile()
	g.visual.Draw()
	if err := g.persistProfile(); err != nil {
		g.model.Status = "Could not save motion preference: " + err.Error()
	}
	g.invalidate()
}
func (g *game) persistProfile() error {
	// Preserve an unreadable profile for recovery instead of replacing unknown data.
	if g.profileError != nil {
		return g.profileError
	}
	return g.saves.WriteProfile(g.profile.Save())
}
func (g *game) nextSlot() {
	switch g.controls.Slot {
	case "1":
		g.controls.Slot = "2"
	case "2":
		g.controls.Slot = "3"
	default:
		g.controls.Slot = "1"
	}
	g.model.Status = "Slot " + g.controls.Slot + " selected · Save or Load this journey."
	g.invalidate()
}
func (g *game) log() {
	if g.stage.Title || g.stage.Busy {
		return
	}
	g.model.ShowHistory = !g.model.ShowHistory
	g.invalidate()
}
func (g *game) invalidate() {
	if g.screen != nil {
		g.screen.Invalidate()
	}
}
func (g *game) fail(err error) { g.model.Status = err.Error(); g.invalidate() }

//go:wasmexport karty_register
func main() {
	g := &game{}
	engine.Run(engine.Hooks{OnStart: g.start, OnUpdate: g.update, OnLevelReady: g.levelReady, OnLevelFailed: g.levelFailed, OnStop: func() {
		g.StopMusic(0)
		g.StopEnvironment(engine.EnvironmentLane0, 0)
		if g.screen != nil {
			g.screen.Close()
		}
		g.art.Close()
	}})
}
