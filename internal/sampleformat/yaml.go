// Package sampleformat keeps generated sample authoring files in the repository's pinned format.
package sampleformat

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// YAML formats generated authoring bytes before publication or drift comparison.
// Contributor tasks put the pinned Prettier executable on PATH.
func YAML(ctx context.Context, source []byte) ([]byte, error) {
	command := exec.CommandContext(ctx, "prettier", "--parser", "yaml")
	command.Stdin = bytes.NewReader(source)

	var diagnostics bytes.Buffer

	command.Stderr = &diagnostics

	formatted, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("format sample YAML: %s: %w", diagnostics.String(), err)
	}

	return formatted, nil
}
