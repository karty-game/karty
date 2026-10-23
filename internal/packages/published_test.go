package packages

import (
	"path/filepath"
	"testing"

	"github.com/karty-game/karty/internal/testpackages"
)

func fpsKitSource(t *testing.T) string {
	t.Helper()

	source, err := testpackages.Source("fps-kit")
	if err != nil {
		t.Fatal(err)
	}

	return source
}

func fpsKitArchive(t *testing.T) string {
	t.Helper()

	source, err := testpackages.Archive("fps-kit")
	if err != nil {
		t.Fatal(err)
	}

	archive := filepath.Join(t.TempDir(), "fps-kit.kpkg")
	write(t, archive, read(t, source))

	return archive
}
