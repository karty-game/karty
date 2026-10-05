package build

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed templates/renderer-debug.js
var rendererDebugScript string

func stageRendererDebugShell(directory string, enabled bool) (string, error) {
	path := filepath.Join(directory, "renderer-debug.js")
	if !enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("remove disabled renderer debugger: %w", err)
		}

		return "", nil
	}

	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return "", fmt.Errorf("renderer debugger output must be a regular file: %w", os.ErrInvalid)
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	if err := os.WriteFile(path, []byte(rendererDebugScript), 0o600); err != nil {
		return "", fmt.Errorf("stage renderer debugger: %w", err)
	}

	digest, err := fileHash(path)
	if err != nil {
		return "", err
	}

	return `<script>globalThis.kartyRendererDebugEnabled = true;</script>` +
		`<script defer src="renderer-debug.js?v=` + digest + `"></script>`, nil
}
