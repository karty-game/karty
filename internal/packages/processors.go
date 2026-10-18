package packages

import (
	"fmt"
	"strings"
)

const maxProcessors = 16

// Processor declares a build tool. Import, export, restore and schema never run it.
// A consuming project must explicitly select it in build.processors.
type Processor struct {
	Name    string   `toml:"name"`
	Command []string `toml:"command"`
}

func validateProcessors(processors []Processor) error {
	if len(processors) > maxProcessors {
		return fmt.Errorf("%w: at most 16 build processors per package", ErrPackage)
	}

	seen := map[string]bool{}
	for _, processor := range processors {
		if !namePattern.MatchString(processor.Name) || seen[processor.Name] || len(processor.Command) == 0 || len(processor.Command) > 32 {
			return fmt.Errorf("%w: invalid or duplicate processor %q", ErrPackage, processor.Name)
		}

		seen[processor.Name] = true
		for _, argument := range processor.Command {
			if argument == "" || len(argument) > 4096 || strings.ContainsRune(argument, 0) {
				return fmt.Errorf("%w: invalid processor argument", ErrPackage)
			}
		}
	}

	return nil
}
