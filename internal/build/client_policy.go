package build

import (
	"context"
	"fmt"
	"strings"

	"github.com/karty-game/karty/internal/process"
)

const errClientDecoderDependency staticError = "client dependency closure includes an unapproved runtime decoder"
const errEmptyClientDependencies staticError = "inspect client dependencies: compiler returned an empty dependency list"

// CheckClientDependencies inspects the complete cartridge dependency closure
// using the selected compiler's WASI environment and package staging paths.
func CheckClientDependencies(ctx context.Context, directory, compiler, executable string, packages ...string) error {
	arguments := []string{"list", "-deps"}
	if compiler == "tinygo" {
		arguments = append(arguments, "-target=wasi", "-buildmode=c-shared", "-scheduler=none")
	}

	arguments = append(arguments, packages...)

	command := process.CommandContext(ctx, executable, arguments...)
	command.Dir = directory
	command.Env = compilerEnvironment(compiler)

	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("inspect client dependencies: %w\n%s", err, output)
	}

	if len(strings.TrimSpace(string(output))) == 0 {
		return errEmptyClientDependencies
	}

	for dependency := range strings.FieldsSeq(string(output)) {
		if forbiddenClientDecoder(dependency) {
			return fmt.Errorf("%s: %w", dependency, errClientDecoderDependency)
		}
	}

	return nil
}

func forbiddenClientDecoder(dependency string) bool {
	for _, prefix := range []string{"encoding/json", "encoding/csv", "encoding/xml", "encoding/gob", "go.yaml.in/", "gopkg.in/yaml.",
		"github.com/go-yaml/", "github.com/pelletier/go-toml", "github.com/BurntSushi/toml", "github.com/json-iterator/",
		"github.com/goccy/go-json", "github.com/goccy/go-yaml"} {
		if dependency == prefix || strings.HasPrefix(dependency, prefix+"/") ||
			strings.HasSuffix(prefix, "/") && strings.HasPrefix(dependency, prefix) ||
			strings.HasSuffix(prefix, ".") && strings.HasPrefix(dependency, prefix) {
			return true
		}
	}

	return dependency == "log/slog"
}
