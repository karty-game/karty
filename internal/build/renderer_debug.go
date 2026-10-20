package build

import (
	"context"
	_ "embed"
	"fmt"
	"os"
)

//go:embed templates/renderer-debug.js
var rendererDebugScript string

func stageRendererDebugShell(ctx context.Context, directory string, enabled bool) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	root, err := os.OpenRoot(directory)
	if err != nil {
		if !enabled && os.IsNotExist(err) {
			return "", nil
		}

		return "", err
	}
	defer root.Close()

	const name = "renderer-debug.js"

	if !enabled {
		if err := root.Remove(name); err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("remove disabled renderer debugger: %w", err)
		}

		return "", nil
	}

	if info, err := root.Lstat(name); err == nil && !info.Mode().IsRegular() {
		return "", fmt.Errorf("renderer debugger output must be a regular file: %w", os.ErrInvalid)
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	if err := writeRootFile(ctx, root, name, []byte(rendererDebugScript)); err != nil {
		return "", fmt.Errorf("stage renderer debugger: %w", err)
	}

	digest, err := rootFileHash(ctx, root, name)
	if err != nil {
		return "", err
	}

	return `<script>globalThis.kartyRendererDebugEnabled = true;</script>` +
		`<script defer src="renderer-debug.js?v=` + digest + `"></script>`, nil
}
