package scaffold

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/karty-game/karty/internal/process"
	"github.com/karty-game/karty/internal/sdk"
)

//go:embed extras/game.yml.tmpl
var workflow string

// Extras controls optional, user-owned project setup.
type Extras struct {
	Git           bool
	Gitignore     bool
	GitHubActions bool
}

// AddExtras modifies only a newly created project; it never commits or publishes.
func AddExtras(ctx context.Context, destination string, manifest sdk.Manifest, options Extras) error {
	if options.Gitignore {
		if err := ensureGitignore(destination); err != nil {
			return err
		}
	}

	if !options.Gitignore {
		if err := os.Remove(filepath.Join(destination, ".gitignore")); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	if options.GitHubActions {
		parsed, err := template.New("workflow").Parse(workflow)
		if err != nil {
			return err
		}

		var rendered bytes.Buffer
		if err := parsed.Execute(&rendered, manifest); err != nil {
			return err
		}

		directory := filepath.Join(destination, ".github", "workflows")
		if err := os.MkdirAll(directory, 0o750); err != nil {
			return err
		}

		if err := os.WriteFile(filepath.Join(directory, "game.yml"), rendered.Bytes(), 0o600); err != nil {
			return err
		}
	}

	if options.Git {
		command := process.CommandContext(ctx, "git", "init", "--initial-branch=main", ".")

		command.Dir = destination
		if output, err := command.CombinedOutput(); err != nil {
			return fmt.Errorf("git init failed; run git init in %s: %w\n%s", destination, err, output)
		}
	}

	return nil
}

func ensureGitignore(destination string) error {
	file, err := os.OpenFile(filepath.Join(destination, ".gitignore"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if os.IsExist(err) {
		return nil
	}

	if err != nil {
		return err
	}

	_, writeErr := file.WriteString(".karty/\ndist/\n*_codegen.go\n")
	closeErr := file.Close()

	if writeErr != nil {
		return writeErr
	}

	return closeErr
}
