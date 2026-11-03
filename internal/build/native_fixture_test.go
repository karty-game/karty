package build_test

import (
	"testing"

	"github.com/karty-game/karty/internal/build"
)

func nativeHostFixture(t *testing.T) string {
	t.Helper()

	return build.NativeHostBundleFixture(t)
}
