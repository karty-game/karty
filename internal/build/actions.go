package build

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/karty-game/karty/internal/actionbuild"
	"github.com/karty-game/karty/internal/levelbuild"
	"github.com/karty-game/karty/internal/sdk"
)

func compileAuthoredActions(directory, module string, manifest sdk.Manifest, artifacts []levelbuild.Artifact) ([]byte, error) {
	declarations, err := actionbuild.Discover(directory, module)
	if err != nil {
		return nil, err
	}

	levels := make([]actionbuild.Level, 0, len(artifacts))
	hasActions := len(declarations) != 0

	for _, artifact := range artifacts {
		if info, err := os.Lstat(filepath.Join(artifact.SourceDirectory, "actions.json")); err == nil {
			if !info.Mode().IsRegular() {
				return nil, actionOutputError("actions.json must be a regular file")
			}

			hasActions = true
		} else if !os.IsNotExist(err) {
			return nil, err
		}

		actors := map[string]bool{}
		for _, id := range artifact.AuthoredActors {
			actors[id] = true
		}

		levels = append(levels, actionbuild.Level{Name: artifact.Name, Directory: artifact.SourceDirectory, Actors: actors})
	}

	if !hasActions {
		return nil, nil
	}

	resources, err := sdk.Resources(manifest)
	if err != nil {
		return nil, err
	}

	contract, err := fs.ReadFile(resources, "contracts/actions-v1.schema.json")
	if err != nil {
		return nil, fmt.Errorf("authored actions require a supporting SDK bundle (SDK 0.0.9): %w", err)
	}

	result, err := actionbuild.Compile(directory, module, declarations, levels, contract)
	if err != nil {
		return nil, err
	}

	if err := writeActionOutputs(directory, result); err != nil {
		return nil, err
	}

	return result.Source, nil
}

func writeActionOutputs(directory string, result actionbuild.Result) error {
	derived := filepath.Join(directory, ".karty")
	if info, err := os.Lstat(derived); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return actionOutputError("symlinked .karty directory")
	}

	root, err := os.OpenRoot(derived)
	if err != nil {
		return err
	}
	defer root.Close()

	if info, err := root.Lstat("actions"); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return actionOutputError("symlinked action output directory")
	}

	if err = root.MkdirAll("actions", 0750); err != nil {
		return err
	}

	if info, err := root.Lstat("actions/levels"); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return actionOutputError("symlinked level schema output")
	}

	if err = root.MkdirAll("actions/levels", 0750); err != nil {
		return err
	}

	for name, data := range result.LevelSchemas {
		if !fs.ValidPath(name) || strings.ContainsAny(name, "/\\") {
			return actionOutputError("invalid level schema output key")
		}

		path := "actions/levels/" + name + ".schema.json"
		if info, err := root.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return actionOutputError("symlinked level schema output %s", path)
		}

		if err = root.WriteFile(path, data, 0600); err != nil {
			return err
		}
	}

	for _, entry := range []struct {
		name string
		data []byte
	}{{"catalog.json", result.Catalog}, {"schema.json", result.Schema}} {
		path := "actions/" + entry.name
		if info, err := root.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return actionOutputError("symlinked action output %s", path)
		}

		if err = root.WriteFile(path, entry.data, 0600); err != nil {
			return err
		}
	}

	return nil
}

var ErrActionOutput = errors.New("invalid action output")

func actionOutputError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrActionOutput, fmt.Sprintf(format, args...))
}
