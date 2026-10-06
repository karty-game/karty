// karty-samples builds sample sites and assembles validated static previews.
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 1 && args[0] == "build" {
		return buildSamples(".")
	}

	if len(args) == 6 && args[0] == "assemble" {
		var openPRs []int
		if err := json.Unmarshal([]byte(args[5]), &openPRs); err != nil {
			return err
		}

		if args[4] != "" && args[4] != "true" && args[4] != "false" {
			return fmt.Errorf("invalid remove flag: %w", os.ErrInvalid)
		}

		return assemble(args[1], args[2], args[3], args[4] == "true", openPRs)
	}

	return fmt.Errorf("usage: karty-samples build | assemble STATE SOURCE DESTINATION REMOVE OPEN_PRS_JSON: %w", os.ErrInvalid)
}
