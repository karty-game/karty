package levelbuild

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOfflineBakeRejectsCacheSymlinkAndCancelledWrite(t *testing.T) {
	t.Parallel()

	for _, relative := range []string{".karty", ".karty/bakes", ".karty/bakes/lighting"} {
		t.Run(relative, func(t *testing.T) {
			t.Parallel()
			root := lightingLevelFixture(t, "version: 4\n"+lightingRoomYAML+directLightmapYAML)
			directory := filepath.Join(root, "levels", "lighting")
			path := filepath.Join(root, filepath.FromSlash(relative))

			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatal(err)
			}

			outside := t.TempDir()
			if err := os.Symlink(outside, path); err != nil {
				t.Fatal(err)
			}

			if _, err := prepareBakeDirectory(root, directory); !errors.Is(err, ErrSource) {
				t.Fatalf("bake followed escaping symlink: %v", err)
			}

			if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
				t.Fatal("bake wrote outside project")
			}
		})
	}

	root := lightingLevelFixture(t, "version: 4\n"+lightingRoomYAML+directLightmapYAML)
	enableFixtureLightmap(t, root, "enabled = true\noffline = true\nlights = [\"red\"]\npage_size = 512\ndensity = 1\n")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := BakeAll(ctx, root, BakeOptions{}, nil); err == nil {
		t.Fatal("canceled bake succeeded")
	}

	if _, err := os.Stat(filepath.Join(root, ".karty")); !os.IsNotExist(err) {
		t.Fatalf("canceled bake created output: %v", err)
	}
}
