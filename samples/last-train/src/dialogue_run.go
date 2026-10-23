package main

import (
	"fmt"
	"strings"

	"example.com/last-train/.karty/processed"
	"github.com/karty-game/dialogue"
)

type dialogueRun struct {
	program *dialogue.Program
	story   *dialogue.Story
	model   *dialogue.Model
}

func (run *dialogueRun) loadProgram() error {
	compiled, ok := processed.Read("dialogue.compile", "story.yarnc")
	if !ok {
		return fmt.Errorf("compiled dialogue is missing")
	}
	table, ok := processed.Read("dialogue.compile", "strings.bin")
	if !ok {
		return fmt.Errorf("dialogue strings are missing")
	}
	var err error
	run.program, err = dialogue.Load(compiled, table)
	return err
}

func (run *dialogueRun) start() error {
	run.story = dialogue.NewStory(run.program)
	return run.story.Start("Start")
}

func (run *dialogueRun) presentSpeaker(line string) string {
	name, body, found := strings.Cut(line, ":")
	if !found {
		return line
	}
	aliases := map[string]string{"Mara": "Conductor", "Elias": "Older passenger", "Iris": "Traveller"}
	alias, character := aliases[name]
	if !character {
		return line
	}
	known, ok := run.story.Variable("$knows_" + strings.ToLower(name))
	if ok && known.Bool {
		return line
	}
	return alias + ":" + body
}

// Keep disabled responses visible whenever the group has an available choice.
func dialogueOptions(choices []dialogue.Choice) (options []dialogue.Option, available bool) {
	options = make([]dialogue.Option, 0, len(choices))
	for _, choice := range choices {
		options = append(options, dialogue.Option{
			ID: choice.ID, Text: choice.Line.Text, Enabled: choice.Enabled,
		})
		available = available || choice.Enabled
	}
	return options, available
}
