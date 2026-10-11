package build

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const errClientJSONDependency staticError = "client dependency closure includes JSON support"

// CheckClientDependencies inspects the complete cartridge dependency closure
// using the selected compiler's WASI environment and package staging paths.
func CheckClientDependencies(ctx context.Context, directory, compiler, executable string, packages ...string) error {
	arguments := []string{"list", "-deps"}
	if compiler == "tinygo" {
		arguments = append(arguments, "-target=wasi", "-buildmode=c-shared", "-scheduler=none")
	}

	arguments = append(arguments, packages...)

	command := exec.CommandContext(ctx, executable, arguments...)
	command.Dir = directory
	command.Env = compilerEnvironment(compiler)

	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("inspect client dependencies: %w\n%s", err, output)
	}

	for dependency := range strings.FieldsSeq(string(output)) {
		if dependency == "encoding/json" || strings.HasPrefix(dependency, "encoding/json/") || dependency == "log/slog" {
			return fmt.Errorf("%s: %w", dependency, errClientJSONDependency)
		}
	}

	return nil
}
