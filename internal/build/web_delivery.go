package build

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/karty-game/karty/internal/contextio"
)

//go:embed templates/web-bootstrap.js.tmpl
var webBootstrapTemplate string

//go:embed templates/web-service-worker.js.tmpl
var webServiceWorkerTemplate string

//go:embed templates/web-status.js.tmpl
var webStatusTemplate string

//nolint:gosec // Only fixed CLI-owned script basenames are joined beneath the caller's staging directory.
func stageWebDelivery(ctx context.Context, directory string) (string, string, error) {
	for _, file := range []struct{ name, contents string }{
		{"karty-web.js", webBootstrapTemplate},
		{"karty-sw.js", webServiceWorkerTemplate},
		{"karty-web-status.js", webStatusTemplate},
	} {
		path := filepath.Join(directory, file.name)

		if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
			return "", "", fmt.Errorf("browser delivery output must be a regular file: %w", os.ErrInvalid)
		} else if err != nil && !os.IsNotExist(err) {
			return "", "", err
		}

		if err := contextio.WriteFile(ctx, path, []byte(file.contents), 0o600); err != nil {
			return "", "", fmt.Errorf("stage browser delivery script: %w", err)
		}
	}

	bootstrap, err := fileHash(ctx, filepath.Join(directory, "karty-web.js"))
	if err != nil {
		return "", "", err
	}

	status, err := fileHash(ctx, filepath.Join(directory, "karty-web-status.js"))
	if err != nil {
		return "", "", err
	}

	return bootstrap, status, nil
}
