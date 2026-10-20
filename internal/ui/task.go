// Package ui provides the CLI's presentation and interactive task primitives.
package ui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type taskFinishedMsg struct {
	err error
}

type staticError string

func (err staticError) Error() string {
	return string(err)
}

const errUnexpectedTaskModel staticError = "task UI returned an unexpected model"

type taskModel struct {
	label string
	err   error
	frame int
}

type taskTickMsg struct{}

type taskProgressMsg string

const (
	taskFrameCount     = 4
	taskFrameDash      = 2
	taskBarWidth       = 24
	taskIndicatorWidth = 5
	taskBarSteps       = taskBarWidth - taskIndicatorWidth + 1
)

// RunTask renders a Bubble Tea task screen while work runs asynchronously.
func RunTask(ctx context.Context, label string, task func(context.Context) error) error {
	return RunTaskWithProgress(ctx, label, func(taskContext context.Context, _ func(string)) error {
		return task(taskContext)
	})
}

// RunTaskWithProgress lets work report the current activity to the task screen.
func RunTaskWithProgress(ctx context.Context, label string, task func(context.Context, func(string)) error) error {
	taskContext, cancel := context.WithCancel(ctx)
	defer cancel()

	model := taskModel{label: label}
	program := tea.NewProgram(
		model,
		tea.WithContext(ctx),
		tea.WithInput(nil),
		tea.WithOutput(os.Stderr),
	)

	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)

		progress := func(label string) { program.Send(taskProgressMsg(label)) }
		program.Send(taskFinishedMsg{err: task(taskContext, progress)})
	}()

	finalModel, err := program.Run()
	// The UI may stop before the task does, including when its context is
	// canceled or terminal initialization fails. Let the worker finish cleanup
	// before the command can return and its caller can exit the process.
	cancel()
	<-workerDone

	if err != nil {
		return fmt.Errorf("run task UI: %w", err)
	}

	result, ok := finalModel.(taskModel)
	if !ok {
		return errUnexpectedTaskModel
	}

	return result.err
}

func (model taskModel) Init() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return taskTickMsg{}
	})
}

func (model taskModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	if label, ok := message.(taskProgressMsg); ok {
		model.label = string(label)

		return model, nil
	}

	if message, ok := message.(taskFinishedMsg); ok {
		model.err = message.err

		return model, tea.Quit
	}

	if _, ok := message.(taskTickMsg); ok {
		model.frame = (model.frame + 1) % taskBarSteps

		return model, tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
			return taskTickMsg{}
		})
	}

	return model, nil
}

func (model taskModel) View() string {
	if model.err != nil {
		return model.label + ": failed"
	}

	return taskFrame(model.frame%taskFrameCount) + " " + model.label + " [" + taskBar(model.frame) + "]"
}

func taskBar(position int) string {
	position %= taskBarSteps

	return strings.Repeat("-", position) + "====>" + strings.Repeat("-", taskBarWidth-position-taskIndicatorWidth)
}

func taskFrame(index int) string {
	switch index {
	case 0:
		return "|"
	case 1:
		return "/"
	case taskFrameDash:
		return "-"
	default:
		return "\\"
	}
}
