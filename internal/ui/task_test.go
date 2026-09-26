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
