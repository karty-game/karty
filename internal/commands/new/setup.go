package newcommand

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/urfave/cli/v3"
)

type options struct {
	name          string
	template      string
	git           bool
	gitignore     bool
	githubActions bool
}

func setup(ctx context.Context, command *cli.Command) (options, error) {
	result := options{
		name:          command.Args().First(),
		template:      command.String("template"),
		git:           command.Bool("git"),
		gitignore:     command.Bool("gitignore"),
		githubActions: command.Bool("github-actions"),
	}
	if command.NArg() > 1 {
		return result, fmt.Errorf("expected one project name; usage: karty new [options] <name>: %w", os.ErrInvalid)
	}

	if command.Bool("no-template") {
		if command.IsSet("template") && result.template != "blank" && result.template != "none" {
			return result, fmt.Errorf("--no-template cannot be combined with --template %s: %w", result.template, os.ErrInvalid)
		}

		result.template = "blank"
	}

	input, terminal := command.Reader.(*os.File)
	if command.Bool("yes") || !terminal || !isatty.IsTerminal(input.Fd()) {
		if result.name == "" {
			return result, fmt.Errorf("expected a project name; usage: karty new [options] <name>: %w", os.ErrInvalid)
		}

		return result, nil
	}

	return askSetup(ctx, command, bufio.NewReader(command.Reader), result)
}

func askSetup(ctx context.Context, command *cli.Command, input *bufio.Reader, result options) (options, error) {
	var err error
	if result.name == "" {
		result.name, err = ask(ctx, input, command.Writer, "Project name", "my-game")
		if err != nil {
			return result, err
		}
	}

	if !command.IsSet("template") && !command.Bool("no-template") {
		for {
			result.template, err = ask(
				ctx,
				input,
				command.Writer,
				"Starter: 1) 2D  2) 3D first-person  3) 3D isometric  4) UI  5) Blank (no template)",
				"1",
			)
			if err != nil {
				return result, err
			}

			switch strings.ToLower(result.template) {
			case "1", "2d", "game":
				result.template = "2d"
			case "2", "fps":
				result.template = "fps"
			case "3", "isometric":
				result.template = "isometric"
			case "4", "ui":
				result.template = "ui"
			case "5", "blank", "none":
				result.template = "blank"
			default:
				fmt.Fprintln(command.Writer, "Choose 1, 2, 3, 4 or 5.")

				continue
			}

			break
		}
	}

	for _, question := range []struct {
		flag, prompt string
		target       *bool
	}{
		{"git", "Initialize Git (no commit)?", &result.git},
		{"gitignore", "Include .gitignore?", &result.gitignore},
		{"github-actions", "Include GitHub Actions web build?", &result.githubActions},
	} {
		if command.IsSet(question.flag) {
			continue
		}

		*question.target, err = askBool(ctx, input, command.Writer, question.prompt, *question.target)
		if err != nil {
			return result, err
		}
	}

	return result, nil
}

func ask(ctx context.Context, input *bufio.Reader, output io.Writer, label, fallback string) (string, error) {
	fmt.Fprintf(output, "%s [%s]: ", label, fallback)

	type response struct {
		text string
		err  error
	}

	result := make(chan response, 1)

	go func() {
		text, err := input.ReadString('\n')
		result <- response{text, err}
	}()

	var answer string

	select {
	case <-ctx.Done():
		return "", fmt.Errorf("setup canceled before completion: %w", ctx.Err())
	case read := <-result:
		if read.err != nil {
			return "", fmt.Errorf("setup canceled before completion: %w", read.err)
		}

		answer = read.text
	}

	answer = strings.TrimSpace(answer)
	if answer == "" {
		return fallback, nil
	}

	return answer, nil
}

func askBool(ctx context.Context, input *bufio.Reader, output io.Writer, label string, fallback bool) (bool, error) {
	hint := "y/N"
	if fallback {
		hint = "Y/n"
	}

	for {
		value, err := ask(ctx, input, output, label, hint)
		if err != nil {
			return false, err
		}

		switch strings.ToLower(value) {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		case "y/n":
			return fallback, nil
		default:
			fmt.Fprintln(output, "Enter yes or no.")
		}
	}
}
