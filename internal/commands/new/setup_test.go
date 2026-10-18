package newcommand

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/urfave/cli/v3"
)

func TestWizardChoicesAndEOF(t *testing.T) {
	t.Parallel()

	for _, example := range []struct{ answer, template string }{{"1", "2d"}, {"2", "fps"}, {"3", "isometric"}, {"4", "ui"}, {"5", "blank"}, {"none", "blank"}} {
		var output bytes.Buffer

		command := Command()
		command.Writer = &output
		input := bufio.NewReader(strings.NewReader("demo\ninvalid\n" + example.answer + "\nyes\nno\nyes\n"))

		got, err := askSetup(context.Background(), command, input, options{gitignore: true})
		if err != nil || got.name != "demo" || got.template != example.template || !got.git || got.gitignore || !got.githubActions {
			t.Fatalf("wizard = %+v, %v", got, err)
		}
	}

	var output bytes.Buffer

	command := Command()

	command.Writer = &output
	if _, err := askSetup(context.Background(), command, bufio.NewReader(strings.NewReader("demo\n")), options{}); err == nil {
		t.Fatal("EOF accepted incomplete setup")
	}
}

func TestFlagsSkipQuestions(t *testing.T) {
	t.Parallel()

	var (
		output bytes.Buffer
		got    options
	)

	command := Command()
	command.Action = func(ctx context.Context, command *cli.Command) error {
		var err error

		got, err = setup(ctx, command)

		return err
	}
	root := &cli.Command{Name: "karty", Commands: []*cli.Command{command}, Reader: strings.NewReader(""), Writer: &output}

	err := root.Run(
		context.Background(),
		[]string{"karty", "new", "--yes", "--template", "fps", "--git", "--github-actions", "--gitignore=false", "demo"},
	)
	if err != nil || got.name != "demo" || got.template != "fps" || !got.git || !got.githubActions || got.gitignore || output.Len() != 0 {
		t.Fatalf("flags = %+v, %v, %s", got, err, output.String())
	}
}

func TestInvalidNewInputBeforeSDKDownload(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"new"}, {"new", "--template", "unknown", "demo"}, {"new", "../demo"}, {"new", "one", "two"}, {"new", "--no-template", "--template", "fps", "demo"}} {
		command := Command()
		root := &cli.Command{Name: "karty", Commands: []*cli.Command{command}, Reader: strings.NewReader(""), Writer: &bytes.Buffer{}}

		err := root.Run(context.Background(), append([]string{"karty"}, args...))
		if err == nil || strings.Contains(err.Error(), "SDK") {
			t.Fatalf("%v reached SDK setup: %v", args, err)
		}
	}
}

func TestWizardCancellationWhileWaitingForInput(t *testing.T) {
	t.Parallel()

	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)

	go func() {
		_, err := ask(ctx, bufio.NewReader(reader), io.Discard, "Starter", "1")
		done <- err
	}()

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("wizard remained blocked after cancellation")
	}
}

func TestNoTemplateFlag(t *testing.T) {
	t.Parallel()

	command := Command()
	command.Action = func(ctx context.Context, command *cli.Command) error {
		options, err := setup(ctx, command)
		if err == nil && options.template != "blank" {
			t.Fatalf("no-template selected %q", options.template)
		}

		return err
	}

	root := &cli.Command{Name: "karty", Commands: []*cli.Command{command}, Reader: strings.NewReader(""), Writer: io.Discard}
	if err := root.Run(t.Context(), []string{"karty", "new", "--yes", "--no-template", "demo"}); err != nil {
		t.Fatal(err)
	}
}
