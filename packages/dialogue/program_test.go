package dialogue

import (
	"os"
	"strings"
	"testing"
)

func fixture(t *testing.T) *Program {
	t.Helper()
	compiled, err := os.ReadFile("testdata/story.yarnc")
	if err != nil {
		t.Fatal(err)
	}
	table, err := os.ReadFile("testdata/strings.bin")
	if err != nil {
		t.Fatal(err)
	}
	program, err := Load(compiled, table)
	if err != nil {
		t.Fatal(err)
	}
	return program
}

func TestOfficialCompilerBranchesCommandsDetoursAndRestore(t *testing.T) {
	for _, chooseClue := range []bool{true, false} {
		t.Run(map[bool]string{true: "clue", false: "no-clue"}[chooseClue], func(t *testing.T) {
			story := NewStory(fixture(t))
			if err := story.Start("Start"); err != nil {
				t.Fatal(err)
			}
			event, err := story.Continue()
			if err != nil || event.Kind != "line" || !strings.Contains(event.Line.Text, "ticket") {
				t.Fatal(event, err)
			}
			event, err = story.Continue()
			if err != nil || event.Kind != "choices" || len(event.Choices) != 2 {
				t.Fatal(event, err)
			}
			if _, err := story.Continue(); err == nil {
				t.Fatal("continued while awaiting choice")
			}
			checkpoint, err := story.Save()
			if err != nil {
				t.Fatal(err)
			}
			restored := NewStory(fixture(t))
			if err := restored.Restore(checkpoint); err != nil {
				t.Fatal(err)
			}
			story = restored
			choice := 1
			if chooseClue {
				choice = 0
			}
			if err := story.Choose(choice); err != nil {
				t.Fatal(err)
			}
			var text, commands []string
			for n := 0; n < 20; n++ {
				event, err = story.Continue()
				if err != nil {
					t.Fatal(err)
				}
				switch event.Kind {
				case "line":
					text = append(text, event.Line.Text)
				case "command":
					commands = append(commands, event.Command)
				case "choices":
					if len(event.Choices) != 2 || event.Choices[0].Enabled != chooseClue {
						t.Fatal("conditional choice", event)
					}
					if !chooseClue && story.Choose(0) == nil {
						t.Fatal("selected unavailable option")
					}
					id := 1
					if chooseClue {
						id = 0
					}
					if err := story.Choose(id); err != nil {
						t.Fatal(err)
					}
				case "complete":
					joined := strings.Join(text, " ")
					if chooseClue && (!strings.Contains(joined, "Trust: 1") || len(commands) != 2 || commands[1] != "ending truth") {
						t.Fatal(joined, commands)
					}
					if !chooseClue && (strings.Contains(joined, "Trust:") || !strings.Contains(joined, "without an answer")) {
						t.Fatal(joined)
					}
					return
				}
			}
			t.Fatal("story did not complete")
		})
	}
}

func TestMalformedProgramAndRunawayLoopFail(t *testing.T) {
	if _, err := Load([]byte{0x12, 0xff}, nil); err == nil {
		t.Fatal("accepted truncated protobuf")
	}
	program := fixture(t)
	program.nodes["Loop"] = node{instructions: []instruction{{op: 1, destination: 0}}}
	story := NewStory(program)
	if err := story.Start("Loop"); err != nil {
		t.Fatal(err)
	}
	if _, err := story.Continue(); err == nil || !strings.Contains(err.Error(), "budget") {
		t.Fatal(err)
	}
	story = NewStory(fixture(t))
	if err := story.Restore([]byte(`{"version":1,"program":"different"}`)); err == nil {
		t.Fatal("accepted incompatible snapshot")
	}
}

func TestRestoreRejectsMissingVariablesAndForeignVisits(t *testing.T) {
	story := NewStory(fixture(t))
	if err := story.Start("Start"); err != nil {
		t.Fatal(err)
	}
	before, err := story.Save()
	if err != nil {
		t.Fatal(err)
	}
	for _, corrupt := range []func(*snapshot){
		func(saved *snapshot) {
			for key := range saved.State.Variables {
				delete(saved.State.Variables, key)
				break
			}
		},
		func(saved *snapshot) { saved.State.Visits["missing-node"] = 1 },
	} {
		saved, err := unmarshalSnapshot(before)
		if err != nil {
			t.Fatal(err)
		}
		corrupt(&saved)
		bad, err := marshalSnapshot(saved)
		if err != nil {
			t.Fatal(err)
		}
		if err := story.Restore(bad); err == nil {
			t.Fatal("accepted invalid checkpoint")
		}
		after, err := story.Save()
		if err != nil || string(after) != string(before) {
			t.Fatal("failed restore changed running story", err)
		}
	}
}
