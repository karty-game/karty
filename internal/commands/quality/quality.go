// Package quality runs the project's Go formatter and linter with Karty defaults.
package quality

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/karty-game/karty/internal/packages"
	"github.com/karty-game/karty/internal/process"
	"github.com/karty-game/karty/internal/project"
	"github.com/urfave/cli/v3"
)

// Version pins the golangci-lint version used by generated CI workflows.
const Version = "2.14.0"

//go:embed default.yml
var defaultConfig []byte

// Command creates fmt or lint. Configuration precedence is explicit, project, Karty.
func Command(name string) *cli.Command {
	usage := "format handwritten Go files using golangci-lint"
	if name == "lint" {
		usage = "lint Go packages for the Karty WASI target using golangci-lint"
	}

	flags := []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "use this golangci-lint config instead of the project or Karty default",
		},
		&cli.StringFlag{Name: "golangci-lint", Usage: "path to golangci-lint v2 (default: PATH)"},
	}
	if name == "fmt" {
		flags = append(flags, &cli.BoolFlag{Name: "check", Usage: "report formatting differences without rewriting files"})
	}

	if name == "lint" {
		flags = append(flags, &cli.BoolFlag{Name: "fix", Usage: "apply fixes supported by enabled linters"})
	}

	return &cli.Command{
		Name:      name,
		Usage:     usage,
		ArgsUsage: "[packages...]",
		Flags:     flags,
		Action: func(ctx context.Context, command *cli.Command) error {
			return run(ctx, command, name, (*exec.Cmd).Run)
		},
	}
}

func run(ctx context.Context, command *cli.Command, name string, execute func(*exec.Cmd) error) error {
	if _, err := project.Load("."); err != nil {
		return err
	}

	set, err := packages.Restore(ctx, ".")
	if err != nil {
		return err
	}
	defer set.Close()

	if err := packages.CheckModules(ctx, ".", set); err != nil {
		return err
	}

	executable := command.String("golangci-lint")
	if executable == "" {
		var err error

		executable, err = exec.LookPath("golangci-lint")
		if err != nil {
			return fmt.Errorf("golangci-lint is required; install v%s or pass --golangci-lint /path/to/golangci-lint: %w", Version, err)
		}
	}

	config, cleanup, err := configuration(command.String("config"))
	if err != nil {
		return err
	}
	defer cleanup()

	action := "fmt"
	if name == "lint" {
		action = "run"
	}

	args := []string{action, "--config", config}
	if command.Bool("check") {
		args = append(args, "--diff")
	}

	if command.Bool("fix") {
		args = append(args, "--fix")
	}

	paths := command.Args().Slice()
	if len(paths) == 0 {
		paths = []string{"./..."}
	}

	for _, path := range paths {
		if strings.HasPrefix(path, "-") {
			return fmt.Errorf("unexpected option %q; use karty %s --help: %w", path, name, os.ErrInvalid)
		}
	}

	args = append(args, paths...)
	child := process.CommandContext(ctx, executable, args...)
	child.Env = wasiEnvironment(os.Environ())
	child.Stdin = command.Reader

	child.Stderr = command.ErrWriter
	if command.Bool("check") {
		// fmt --diff may exit successfully when it prints differences. Make check useful in CI.
		var differences strings.Builder

		child.Stdout = io.MultiWriter(command.Writer, &differences)
		if err := execute(child); err != nil {
			return err
		}

		if differences.Len() != 0 {
			return fmt.Errorf("files need Go formatting; run karty fmt: %w", os.ErrInvalid)
		}

		return nil
	}

	child.Stdout = command.Writer

	return execute(child)
}

func configuration(explicit string) (string, func(), error) {
	noop := func() {}
	if explicit != "" {
		return explicit, noop, nil
	}

	for _, name := range []string{".golangci.yml", ".golangci.yaml", ".golangci.toml", ".golangci.json"} {
		if _, err := os.Stat(name); err == nil {
			return name, noop, nil
		} else if !os.IsNotExist(err) {
			return "", noop, err
		}
	}
	// The bundled config sets relative-path-mode to wd, so it need not write into the project.
	file, err := os.CreateTemp("", "karty-golangci-*.yml")
	if err != nil {
		return "", noop, err
	}

	path := file.Name()
	cleanup := func() { _ = os.Remove(path) }
	_, writeErr := file.Write(defaultConfig)
	closeErr := file.Close()

	if writeErr != nil {
		cleanup()

		return "", noop, writeErr
	}

	if closeErr != nil {
		cleanup()

		return "", noop, closeErr
	}

	return path, cleanup, nil
}

func wasiEnvironment(environment []string) []string {
	result := make([]string, 0, len(environment)+4)
	for _, entry := range environment {
		key, _, _ := strings.Cut(entry, "=")
		switch key {
		case "GOOS", "GOARCH", "CGO_ENABLED", "GOWORK":
			continue
		}

		result = append(result, entry)
	}

	return append(result, "GOOS=wasip1", "GOARCH=wasm", "CGO_ENABLED=0", "GOWORK=off")
}
