package scaffold_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/scaffold"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/skills"
)

func TestEveryStarterIncludesCompleteGameSkill(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	files := embeddedGameSkill(t)

	for _, starter := range []string{"blank", "2d", "fps", "isometric", "ui", "none", "game"} {
		t.Run(starter, func(t *testing.T) {
			t.Parallel()

			directory := filepath.Join(t.TempDir(), "demo")
			if err := scaffold.CreateTemplate(directory, "demo", manifest, starter); err != nil {
				t.Fatal(err)
			}

			for path, expected := range files {
				actual, err := os.ReadFile(filepath.Join(directory, ".agents", "skills", filepath.FromSlash(path)))
				if err != nil {
					t.Fatal(err)
				}

				if !bytes.Equal(actual, expected) {
					t.Errorf("installed skill file %s differs from the maintained source", path)
				}
			}
		})
	}
}

func embeddedGameSkill(t *testing.T) map[string][]byte {
	t.Helper()

	files := make(map[string][]byte)

	err := fs.WalkDir(skills.Files, "karty-game", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			return nil
		}

		contents, err := skills.Files.ReadFile(path)
		if err != nil {
			return err
		}

		files[path] = contents

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := files["karty-game/SKILL.md"]; !ok {
		t.Fatal("discoverable SKILL.md missing from embedded game skill")
	}

	return files
}

func TestCreateTemplatePreservesExistingAgentSkill(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()

	skillPath := filepath.Join(directory, ".agents", "skills", "karty-game", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o750); err != nil {
		t.Fatal(err)
	}

	const custom = "User-maintained project guidance.\n"

	if err := os.WriteFile(skillPath, []byte(custom), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := scaffold.CreateTemplate(directory, "demo", sdk.Manifest{}, "blank"); err == nil {
		t.Fatal("accepted an existing project")
	}

	actual, err := os.ReadFile(skillPath)
	if err != nil || string(actual) != custom {
		t.Fatalf("existing agent skill changed: %q, error = %v", actual, err)
	}

	if _, err := os.Stat(filepath.Join(directory, "karty.toml")); !os.IsNotExist(err) {
		t.Fatalf("existing project received scaffold files: %v", err)
	}
}
