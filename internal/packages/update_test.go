package packages

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveInsideGeneratedOutputsIsVendoredBeforeRecording(t *testing.T) {
	t.Parallel()

	kit, project := fixture(t)
	if err := os.MkdirAll(filepath.Join(project, "dist"), 0750); err != nil {
		t.Fatal(err)
	}

	input := filepath.Join(project, "dist/kit.kpkg")
	export(t, kit, input)

	if _, err := Import(t.Context(), project, input, false); err != nil {
		t.Fatal(err)
	}

	sources, _, err := LoadSources(t.Context(), project)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(sources["placeholder-kit"].Archive, "packages/") {
		t.Fatal("recorded disposable archive as source")
	}

	if err := os.RemoveAll(filepath.Join(project, "dist")); err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(filepath.Join(project, ".karty")); err != nil {
		t.Fatal(err)
	}

	restore(t, project)
}

func TestExplicitUpdatePreservesOtherTablesAndRemovesOnlyItsOwnedGoModule(t *testing.T) {
	t.Parallel()

	kit, project := fixture(t)
	if _, err := Import(t.Context(), project, kit, false); err != nil {
		t.Fatal(err)
	}

	manifest := filepath.Join(project, "karty.toml")
	suffix := []byte("\n# Preserve unrelated settings\n[project.web]\nruntime_url = 'https://example.invalid/runtime/'\n")
	write(t, manifest, append(read(t, manifest), suffix...))

	module := filepath.Join(project, "go.mod")
	write(t, module, append(read(t, module), []byte("\nrequire example.com/unrelated v0.1.0\n")...))

	if err := os.Remove(filepath.Join(kit, "go.mod")); err != nil {
		t.Fatal(err)
	}

	if _, err := Import(t.Context(), project, kit, true); err != nil {
		t.Fatal(err)
	}

	if !bytes.HasSuffix(read(t, manifest), suffix) {
		t.Fatal("update rewrote unrelated TOML/comments")
	}

	updated := read(t, module)
	if bytes.Contains(updated, []byte("github.com/karty-game/stdlib-spike")) ||
		!bytes.Contains(updated, []byte("example.com/unrelated v0.1.0")) {
		t.Fatal("did not limit Go edits to the replaced package")
	}

	if err := os.Remove(module); err != nil {
		t.Fatal(err)
	}

	if err := CheckModules(t.Context(), project, restore(t, project)); err != nil {
		t.Fatal(err)
	}

	if _, err := Import(t.Context(), project, kit, true); err != nil {
		t.Fatal("resource-only workspace update required go.mod:", err)
	}
}

func TestRejectedDuplicateIdentityPreservesAuthoredDependencyFiles(t *testing.T) {
	t.Parallel()

	kit, project := fixture(t)
	if _, err := Import(t.Context(), project, kit, false); err != nil {
		t.Fatal(err)
	}

	before := read(t, filepath.Join(project, "karty.toml"))

	module := read(t, filepath.Join(project, "go.mod"))

	second := filepath.Join(t.TempDir(), "second-kit")
	if err := os.CopyFS(second, os.DirFS(kit)); err != nil {
		t.Fatal(err)
	}

	name := filepath.Join(second, "go.mod")
	write(
		t,
		name,
		bytes.ReplaceAll(read(t, name), []byte("github.com/karty-game/stdlib-spike"), []byte("github.com/karty-game/another-package")),
	)

	if _, err := Import(t.Context(), project, second, false); err == nil {
		t.Fatal("duplicate package name accepted without --replace")
	}

	if !bytes.Equal(before, read(t, filepath.Join(project, "karty.toml"))) ||
		!bytes.Equal(module, read(t, filepath.Join(project, "go.mod"))) {
		t.Fatal("failed import changed authored dependencies")
	}
}

func TestRejectedCaseCollisionRollsBackAuthoredDependencyFiles(t *testing.T) {
	t.Parallel()

	kit, project := fixture(t)
	if _, err := Import(t.Context(), project, kit, false); err != nil {
		t.Fatal(err)
	}

	before := read(t, filepath.Join(project, "karty.toml"))
	module := read(t, filepath.Join(project, "go.mod"))

	second := filepath.Join(t.TempDir(), "second-kit")
	if err := os.CopyFS(second, os.DirFS(kit)); err != nil {
		t.Fatal(err)
	}

	manifest := filepath.Join(second, ManifestName)
	write(t, manifest, bytes.ReplaceAll(read(t, manifest), []byte("placeholder-kit"), []byte("Placeholder-kit")))

	modulePath := filepath.Join(second, "go.mod")
	write(
		t,
		modulePath,
		bytes.ReplaceAll(read(t, modulePath), []byte("github.com/karty-game/stdlib-spike"), []byte("example.com/another-package")),
	)

	if _, err := Import(t.Context(), project, second, false); err == nil {
		t.Fatal("case-colliding package accepted")
	}

	if !bytes.Equal(before, read(t, filepath.Join(project, "karty.toml"))) ||
		!bytes.Equal(module, read(t, filepath.Join(project, "go.mod"))) {
		t.Fatal("failed import changed authored dependencies")
	}

	if err := CheckModules(t.Context(), project, restore(t, project)); err != nil {
		t.Fatal("rollback did not retain the original package mapping:", err)
	}
}

func TestRestoreRequiresProjectKeyToMatchPackageManifestName(t *testing.T) {
	t.Parallel()

	kit, project := fixture(t)
	if _, err := Import(t.Context(), project, kit, false); err != nil {
		t.Fatal(err)
	}

	manifest := filepath.Join(project, "karty.toml")
	write(t, manifest, bytes.ReplaceAll(read(t, manifest), []byte("[packages.placeholder-kit]"), []byte("[packages.arbitrary]")))

	if set, err := Restore(t.Context(), project); err == nil {
		set.Close()
		t.Fatal("project declaration renamed package identity")
	}

	if _, err := os.Lstat(filepath.Join(project, ".karty/packages/arbitrary")); !os.IsNotExist(err) {
		t.Fatal("mismatched package link was published")
	}
}

func TestPackageRequiresItsOwnManifestAndPortableName(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"missing manifest", "", "../escape", "publisher/name", "has.dot", "NUL", strings.Repeat("a", 49)} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			kit, project := fixture(t)

			manifest := filepath.Join(kit, ManifestName)
			if name == "missing manifest" {
				if err := os.Remove(manifest); err != nil {
					t.Fatal(err)
				}
			} else {
				write(t, manifest, bytes.ReplaceAll(read(t, manifest), []byte("placeholder-kit"), []byte(name)))
			}

			before := read(t, filepath.Join(project, "karty.toml"))
			if _, err := Import(t.Context(), project, kit, false); err == nil {
				t.Fatal("accepted a package without a valid manifest identity")
			}

			archive := filepath.Join(project, "invalid.kpkg")
			if _, _, err := Export(t.Context(), kit, archive); err == nil {
				t.Fatal("export accepted a package without a valid manifest identity")
			}

			if !bytes.Equal(before, read(t, filepath.Join(project, "karty.toml"))) {
				t.Fatal("rejected package changed the project manifest")
			}
		})
	}
}
