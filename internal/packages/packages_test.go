package packages

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, name string, data []byte) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(name), 0750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(name, data, 0600); err != nil {
		t.Fatal(err)
	}
}
func read(t *testing.T, name string) []byte {
	t.Helper()

	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func fixture(t *testing.T) (string, string) {
	t.Helper()
	parent := t.TempDir()
	kit := filepath.Join(parent, "kit")

	project := filepath.Join(parent, "game")
	for _, name := range []string{ManifestName, "go.mod", "prefabs.yaml", "assets/checker.png", "assets/wood.png", "assets/frame.png", "fps/controller.go", "controls/input.go"} {
		write(t, filepath.Join(kit, name), read(t, filepath.Join("../../testdata/local-packages/library", name)))
	}

	write(t, filepath.Join(project, "karty.toml"), []byte("# Keep project comments\n[project]\nname = 'test'\n[sdk]\nversion = '0.0.13'\n"))
	write(t, filepath.Join(project, "go.mod"), []byte("module example.com/game\n\ngo 1.27.0\n\n// Keep module comments\n"))

	return kit, project
}

func export(t *testing.T, kit, path string) string {
	t.Helper()

	_, digest, err := Export(t.Context(), kit, path)
	if err != nil {
		t.Fatal(err)
	}

	return digest
}
func restore(t *testing.T, project string) *Set {
	t.Helper()

	set, err := Restore(t.Context(), project)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(set.Close)

	return set
}

func TestLocalArchiveIsPortablePinnedAndRebuildsItsCache(t *testing.T) {
	t.Parallel()
	kit, project := fixture(t)
	write(t, filepath.Join(kit, ".karty/ignored"), []byte("not distributed"))
	archive := filepath.Join(filepath.Dir(kit), "kit-0.1.0.kpkg")
	digest := export(t, kit, archive)

	second := filepath.Join(filepath.Dir(kit), "second.kpkg")
	if export(t, kit, second) != digest || !bytes.Equal(read(t, archive), read(t, second)) {
		t.Fatal("non-deterministic archive")
	}

	if definition, err := Import(t.Context(), project, archive, false); err != nil || definition.Name != "placeholder-kit" {
		t.Fatalf("import did not use the manifest name: %q: %v", definition.Name, err)
	}

	sources, _, err := LoadSources(t.Context(), project)
	if err != nil {
		t.Fatal(err)
	}

	if sources["placeholder-kit"].SHA256 != digest || !strings.HasPrefix(sources["placeholder-kit"].Archive, "packages/") {
		t.Fatalf("archive was not vendored/pinned: %+v", sources)
	}

	if !bytes.Contains(read(t, filepath.Join(project, "karty.toml")), []byte("# Keep project comments")) ||
		!bytes.Contains(read(t, filepath.Join(project, "go.mod")), []byte("// Keep module comments")) {
		t.Fatal("import lost unrelated comments")
	}

	if err := os.RemoveAll(kit); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(archive); err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(filepath.Join(project, ".karty")); err != nil {
		t.Fatal(err)
	}

	set := restore(t, project)
	if err := CheckModules(t.Context(), project, set); err != nil {
		t.Fatal(err)
	}

	if set.Items["placeholder-kit"].Manifest.Version != "0.1.0" {
		t.Fatal("lost version")
	}
	// A second checkout needs only authored manifest/module and vendored archive.
	checkout := t.TempDir()
	for _, name := range []string{"karty.toml", "go.mod", sources["placeholder-kit"].Archive} {
		write(t, filepath.Join(checkout, name), read(t, filepath.Join(project, name)))
	}

	if len(restore(t, checkout).Items) != 1 {
		t.Fatal("fresh checkout did not restore")
	}

	root := set.Items["placeholder-kit"].Root
	if _, err := root.Stat(".karty/ignored"); !os.IsNotExist(err) {
		t.Fatal("export included generated output")
	}

	write(t, filepath.Join(project, sources["placeholder-kit"].Archive), []byte("tampered archive"))

	if next, err := Restore(t.Context(), project); err == nil {
		next.Close()
		t.Fatal("warm cache bypassed archive pin")
	}
}

func TestFolderLinkIsLiveAndCanBeExplicitlyReplacedByArchive(t *testing.T) {
	t.Parallel()

	kit, project := fixture(t)
	if _, err := Import(t.Context(), project, kit, false); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(project, ".karty/packages/placeholder-kit")

	resolved, err := filepath.EvalSymlinks(link)
	if err != nil || resolved != kit {
		t.Fatalf("not linked: %s %v", resolved, err)
	}

	data := read(t, filepath.Join(kit, "assets/wood.png"))
	write(t, filepath.Join(kit, "assets/wood.png"), append(data, 0))

	if !bytes.Equal(read(t, filepath.Join(link, "assets/wood.png")), append(data, 0)) {
		t.Fatal("folder link is a stale copy")
	}

	archive := filepath.Join(filepath.Dir(kit), "kit.kpkg")
	export(t, kit, archive)

	before := read(t, filepath.Join(project, "karty.toml"))
	if _, err := Import(t.Context(), project, archive, false); err == nil {
		t.Fatal("implicit update accepted")
	}

	if !bytes.Equal(before, read(t, filepath.Join(project, "karty.toml"))) {
		t.Fatal("rejected update changed manifest")
	}

	if _, err := Import(t.Context(), project, archive, true); err != nil {
		t.Fatal(err)
	}

	sources, _, err := LoadSources(t.Context(), project)
	if err != nil || sources["placeholder-kit"].Folder != "" || sources["placeholder-kit"].Archive == "" {
		t.Fatalf("source not replaced: %+v %v", sources, err)
	}

	if err := CheckModules(t.Context(), project, restore(t, project)); err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(filepath.Join(project, ".karty")); err != nil {
		t.Fatal(err)
	}

	restore(t, project)
}

func TestCacheTamperAndOutputRedirectionFailClosed(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"cache bytes", "cache link", "package link file", "output parent"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			kit, project := fixture(t)
			archive := filepath.Join(filepath.Dir(kit), "kit.kpkg")
			export(t, kit, archive)

			if _, err := Import(t.Context(), project, archive, false); err != nil {
				t.Fatal(err)
			}

			set := restore(t, project)
			directory := set.Items["placeholder-kit"].Directory

			switch mode {
			case "cache bytes":
				write(t, filepath.Join(directory, "assets/wood.png"), []byte("bad"))
			case "cache link":
				if err := os.Remove(filepath.Join(directory, "assets/wood.png")); err != nil {
					t.Fatal(err)
				}

				if err := os.Symlink(filepath.Join(kit, "assets/wood.png"), filepath.Join(directory, "assets/wood.png")); err != nil {
					t.Fatal(err)
				}
			case "package link file":
				if err := os.Remove(filepath.Join(project, ".karty/packages/placeholder-kit")); err != nil {
					t.Fatal(err)
				}

				write(t, filepath.Join(project, ".karty/packages/placeholder-kit"), []byte("user-owned"))
			case "output parent":
				set.Close()

				if err := os.RemoveAll(filepath.Join(project, ".karty")); err != nil {
					t.Fatal(err)
				}

				if err := os.Symlink(kit, filepath.Join(project, ".karty")); err != nil {
					t.Fatal(err)
				}
			}

			if next, err := Restore(t.Context(), project); err == nil {
				next.Close()
				t.Fatal("invalid cache/output accepted")
			}
		})
	}
}

func TestBadArchiveAndInvalidExportsDoNotChangeProjectFiles(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"bad zip", "bad SDK", "unknown field", "unused invalid prefab", "source link"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			kit, project := fixture(t)
			before := read(t, filepath.Join(project, "karty.toml"))
			module := read(t, filepath.Join(project, "go.mod"))
			input := kit

			switch mode {
			case "bad zip":
				input = filepath.Join(filepath.Dir(kit), "bad.kpkg")
				write(t, input, []byte("not zip"))
			case "bad SDK":
				name := filepath.Join(kit, ManifestName)
				write(t, name, bytes.ReplaceAll(read(t, name), []byte("0.0.13"), []byte("0.0.99")))
			case "unknown field":
				name := filepath.Join(kit, ManifestName)
				write(t, name, append([]byte("unknown = true\n"), read(t, name)...))
			case "unused invalid prefab":
				name := filepath.Join(kit, "prefabs.yaml")
				write(t, name, append(read(t, name), []byte("\n  - id: bad\n    instances: [{id: recursive, prefab: bad}]\n")...))
			case "source link":
				if err := os.Symlink(filepath.Join(project, "go.mod"), filepath.Join(kit, "leak")); err != nil {
					t.Fatal(err)
				}

				input = filepath.Join(filepath.Dir(kit), "export.kpkg")
				if _, _, err := Export(t.Context(), kit, input); err == nil {
					t.Fatal("export followed source link")
				}

				return
			}

			if _, err := Import(t.Context(), project, input, false); err == nil {
				t.Fatal("invalid import succeeded")
			}

			if !bytes.Equal(before, read(t, filepath.Join(project, "karty.toml"))) ||
				!bytes.Equal(module, read(t, filepath.Join(project, "go.mod"))) {
				t.Fatal("invalid import changed user files")
			}
		})
	}
}

func TestArchiveRejectsUnsafeEntriesBeforeExtraction(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"../outside", "/absolute", "C:/drive", "back\\slash", "NUL.png", "trailing.", "a/../../outside"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer

			writer := zip.NewWriter(&out)

			entry, err := writer.Create(name)
			if err != nil {
				t.Fatal(err)
			}

			_, _ = entry.Write([]byte("bad"))

			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}

			if _, err := decodeArchive(t.Context(), out.Bytes()); !errors.Is(err, ErrPackage) {
				t.Fatal("unsafe name accepted")
			}
		})
	}

	for _, mode := range []string{"duplicate", "case collision", "symlink", "inventory mismatch", "prefix file collision"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer

			writer := zip.NewWriter(&out)

			add := func(name string, mode os.FileMode) {
				header := &zip.FileHeader{Name: name, Method: zip.Store}
				header.SetMode(mode)

				file, err := writer.CreateHeader(header)
				if err != nil {
					t.Fatal(err)
				}

				_, _ = file.Write([]byte("test"))
			}
			switch mode {
			case "duplicate":
				add("same", 0644)
				add("same", 0644)
			case "case collision":
				add("Same", 0644)
				add("same", 0644)
			case "symlink":
				add("link", os.ModeSymlink|0777)
			case "inventory mismatch":
				data, _ := json.Marshal(inventory{FormatVersion: 1})
				file, _ := writer.Create(InventoryName)
				_, _ = file.Write(data)

				add(ManifestName, 0644)
			case "prefix file collision":
				add("A", 0644)
				add("a/file", 0644)
			}

			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}

			if _, err := decodeArchive(t.Context(), out.Bytes()); !errors.Is(err, ErrPackage) {
				t.Fatal("malformed archive accepted")
			}
		})
	}
}

func TestArchiveBoundsAreCheckedBeforePayloadAllocation(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer

	writer := zip.NewWriter(&out)
	header := &zip.FileHeader{Name: "oversized.png", Method: zip.Store, UncompressedSize64: MaxFileBytes + 1, CompressedSize64: 0}
	header.SetMode(0644)

	if _, err := writer.CreateRaw(header); err != nil {
		t.Fatal(err)
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := decodeArchive(t.Context(), out.Bytes()); !errors.Is(err, ErrPackage) {
		t.Fatalf("oversized entry was not rejected before decoding: %v", err)
	}
}

func TestCancelledOperationsDoNotPublish(t *testing.T) {
	t.Parallel()
	kit, project := fixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, _, err := Export(ctx, kit, filepath.Join(filepath.Dir(kit), "cancelled.kpkg")); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}

	if _, err := Import(ctx, project, kit, false); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
