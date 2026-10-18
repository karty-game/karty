package scaffold_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/scaffold"
	"github.com/karty-game/karty/internal/sdk"
	"go.yaml.in/yaml/v3"
)

func TestProjectOptionsAndGitInitialization(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	directory := filepath.Join(t.TempDir(), "demo")
	if err := scaffold.CreateTemplate(directory, "demo", manifest, "2d"); err != nil {
		t.Fatal(err)
	}

	if err := scaffold.AddExtras(t.Context(), directory, manifest, scaffold.Extras{Git: true, GitHubActions: true}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(directory, ".gitignore")); !os.IsNotExist(err) {
		t.Fatalf("gitignore=false left ignore file: %v", err)
	}

	if _, err := os.Stat(filepath.Join(directory, ".git")); err != nil {
		t.Fatal(err)
	}

	command := exec.CommandContext(context.Background(), "git", "-C", directory, "rev-parse", "--verify", "HEAD")
	if err := command.Run(); err == nil {
		t.Fatal("setup created a commit")
	}

	data, err := os.ReadFile(filepath.Join(directory, ".github/workflows/game.yml"))
	if err != nil {
		t.Fatal(err)
	}

	var workflow map[string]any
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}

	for _, required := range []string{manifest.Tools.Go, "karty build --target web", "karty fmt --check", "karty lint", "contents: read", "persist-credentials: false", "karty.toml", "RUNTIME_LICENSE.md", "SDK_LICENSE.md"} {
		if !strings.Contains(string(data), required) {
			t.Errorf("workflow missing %q", required)
		}
	}
}

func TestRejectBadNamesAndPreserveExistingDirectory(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "keep.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"../escape", "two words", "", "-option"} {
		if err := scaffold.ValidateDestination(filepath.Join(t.TempDir(), "new"), name); err == nil {
			t.Errorf("accepted %q", name)
		}
	}

	if err := scaffold.ValidateDestination(directory, "demo"); err == nil {
		t.Fatal("accepted existing directory")
	}

	if data, err := os.ReadFile(filepath.Join(directory, "keep.txt")); err != nil || string(data) != "keep" {
		t.Fatal("changed existing project")
	}
}

func TestSDKReportsMissing3DStarter(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	manifest.Templates.Game = "missing-template-fixture"

	for _, selection := range []string{"fps", "isometric"} {
		directory := filepath.Join(t.TempDir(), "demo")

		err := scaffold.CreateTemplate(directory, "demo", manifest, selection)
		if err == nil || !strings.Contains(err.Error(), "does not include") {
			t.Fatalf("unexpected missing-template result: %v", err)
		}

		if _, err := os.Stat(directory); !os.IsNotExist(err) {
			t.Fatalf("unsupported starter wrote output: %v", err)
		}
	}
}

func TestGitignoreIncludedForLegacyUITemplate(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	directory := filepath.Join(t.TempDir(), "demo")
	if err := scaffold.CreateTemplate(directory, "demo", manifest, "ui"); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(filepath.Join(directory, ".gitignore")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	if err := scaffold.AddExtras(t.Context(), directory, manifest, scaffold.Extras{Gitignore: true}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(directory, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}

	for _, line := range []string{".karty/", "dist/", "*_codegen.go"} {
		if !strings.Contains(string(data), line) {
			t.Errorf("missing %s", line)
		}
	}
}
