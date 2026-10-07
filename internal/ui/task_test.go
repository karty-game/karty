package ui

import (
	"errors"
	"strings"
	"testing"
)

func TestTaskModelViewIncludesProgressBar(t *testing.T) {
	t.Parallel()

	view := (taskModel{label: "Building client"}).View()
	if !strings.Contains(view, "Building client [") {
		t.Fatalf("task view = %q, want label and progress bar", view)
	}
}

func TestTaskModelReturnsTaskError(t *testing.T) {
	t.Parallel()

	want := staticError("task failed")

	model, command := (taskModel{}).Update(taskFinishedMsg{err: want})
	if command == nil {
		t.Fatal("task completion did not quit the program")
	}

	result, ok := model.(taskModel)
	if !ok {
		t.Fatal("task completion returned an unexpected model")
	}

	if !errors.Is(result.err, want) {
		t.Fatalf("task error = %v, want %v", result.err, want)
	}
}

func TestTaskProgressRetainsAnimationAndFailureActivity(t *testing.T) {
	t.Parallel()

	model, command := (taskModel{label: "Building game", frame: 3}).Update(taskProgressMsg("Compiling Go game"))

	result, valid := model.(taskModel)
	if !valid || command != nil || result.frame != 3 || !strings.Contains(result.View(), "Compiling Go game [") {
		t.Fatalf("progress lost the activity or animation: %+v", model)
	}

	model, command = result.Update(taskFinishedMsg{err: staticError("compiler failed")})

	result, valid = model.(taskModel)
	if !valid || command == nil || result.View() != "Compiling Go game: failed" {
		t.Fatalf("failure lost its build activity: %+v", model)
	}
}
