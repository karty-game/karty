package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateRequiresSeparateLinkedSections(t *testing.T) {
	t.Parallel()

	for _, layout := range [][2]string{{"guide", "guide"}, {"guide", "other/design"}, {"other", "design"}} {
		t.Run(layout[0]+"_"+layout[1], func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()

			output, designOutput := filepath.Join(root, layout[0]), filepath.Join(root, layout[1])
			if err := generate(output, designOutput, ""); !errors.Is(err, os.ErrInvalid) {
				t.Fatalf("invalid section destinations accepted: %v", err)
			}

			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("generated files before validating section destinations: %v", err)
			}
		})
	}
}
