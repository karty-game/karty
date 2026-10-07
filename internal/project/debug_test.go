package project_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty/internal/project"
	"github.com/karty-game/karty/internal/release"
)

func TestProjectRendererDebugDefaultsAndOptIn(t *testing.T) {
	t.Parallel()

	for _, enabled := range []bool{false, true} {
		directory := t.TempDir()

		manifest := "[project]\nname='debug'\n[sdk]\nversion='" + release.SDKVersion() + "'\n"
		if enabled {
			manifest += "[project.debug]\nrenderer=true\n"
		}

		if err := os.WriteFile(filepath.Join(directory, "karty.toml"), []byte(manifest), 0o600); err != nil {
			t.Fatal(err)
		}

		config, err := project.Load(directory)
		if err != nil || config.Project.Debug.Renderer != enabled {
			t.Fatalf("renderer debug=%t: %v", enabled, err)
		}
	}
}
