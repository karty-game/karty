package build

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestWebReleaseDeterministicAndComplete(t *testing.T) {
	t.Parallel()

	root := webReleaseTestFiles(t)
	content := webReleaseTestContent(t, root, ".kld", []byte("level"))
	audio := webReleaseTestContent(t, root, ".kaud", []byte("audio"))
	video := webReleaseTestContent(t, root, ".kvid", []byte("video"))
	webReleaseTestWrite(t, root, "renderer-debug.js", []byte("debug"))
	webReleaseTestWrite(t, root, "karty-web-status.js", []byte("status"))
	webReleaseTestWrite(t, root, "karty-sw.js", []byte("independent worker lifecycle"))
	webReleaseTestWrite(t, root, "not-a-game-asset.txt", []byte("ignore unrelated output"))

	if err := stageWebRelease(t.Context(), root, nil); err != nil {
		t.Fatal(err)
	}

	first, manifest, raw := readWebReleaseTest(t, root)

	if first.Version != 1 || manifest.Version != 1 || len(manifest.Files) != 11 {
		t.Fatalf("incomplete release: %+v, %+v", first, manifest)
	}

	urls := make([]string, 0, len(manifest.Files))

	for _, entry := range manifest.Files {
		urls = append(urls, entry.URL)

		contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(entry.URL)))
		if err != nil {
			t.Fatal(err)
		}

		digest := sha256.Sum256(contents)

		if entry.SHA256 != hex.EncodeToString(digest[:]) || entry.Size != int64(len(contents)) {
			t.Fatalf("wrong decoded metadata: %+v", entry)
		}

		if strings.HasPrefix(entry.URL, "content/") {
			if entry.Role != "background" {
				t.Fatalf("content prefetch role = %q", entry.Role)
			}
		} else if entry.Role != "startup" {
			t.Fatalf("shell startup role = %q", entry.Role)
		}
	}

	if !slices.IsSorted(urls) || !slices.Contains(urls, content) || !slices.Contains(urls, audio) || !slices.Contains(urls, video) ||
		!slices.Contains(urls, "karty-web-status.js") {
		t.Fatalf("unsorted or incomplete files: %v", urls)
	}

	if slices.Contains(urls, "karty-sw.js") || slices.Contains(urls, "not-a-game-asset.txt") {
		t.Fatalf("unowned file in release: %v", urls)
	}

	if err := stageWebRelease(t.Context(), root, nil); err != nil {
		t.Fatal(err)
	}

	second, _, repeated := readWebReleaseTest(t, root)

	if first != second || !bytes.Equal(raw, repeated) {
		t.Fatal("identical staging changed release identity")
	}

	webReleaseTestWrite(t, root, "game.kart", []byte("updated cartridge"))

	if err := stageWebRelease(t.Context(), root, nil); err != nil {
		t.Fatal(err)
	}

	updated, _, _ := readWebReleaseTest(t, root)

	if first.Release == updated.Release {
		t.Fatal("changed cartridge retained old release identity")
	}

	previous, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(first.Manifest)))
	if err != nil || !bytes.Equal(previous, raw) {
		t.Fatalf("previous immutable manifest was changed: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(root, "karty-releases"))
	if err != nil || len(entries) != 2 {
		t.Fatalf("release staging copied files instead of manifests: %v, %v", entries, err)
	}
}

func TestWebReleaseSharedHostDoesNotCopyOrDownload(t *testing.T) {
	t.Parallel()

	root := webReleaseTestFiles(t)

	if err := os.Remove(filepath.Join(root, "karty-host.wasm")); err != nil {
		t.Fatal(err)
	}

	host := &cdnHost{URL: "../runtime/host.wasm", SHA256: strings.Repeat("a", 64)}

	if err := stageWebRelease(t.Context(), root, host); err != nil {
		t.Fatal(err)
	}

	_, manifest, _ := readWebReleaseTest(t, root)

	if manifest.Files[0].URL != host.URL || manifest.Files[0].SHA256 != host.SHA256 || manifest.Files[0].Size != 0 {
		t.Fatalf("shared host metadata = %+v", manifest.Files[0])
	}

	if _, err := os.Stat(filepath.Join(root, "karty-host.wasm")); !os.IsNotExist(err) {
		t.Fatalf("shared host was copied: %v", err)
	}

	for _, invalid := range []*cdnHost{
		{URL: "../../outside.wasm", SHA256: host.SHA256},
		{URL: "https://example.com/host.wasm", SHA256: "invalid"},
		{URL: "game.kart", SHA256: host.SHA256},
	} {
		if err := stageWebRelease(t.Context(), root, invalid); err == nil {
			t.Fatalf("invalid shared host accepted: %+v", invalid)
		}
	}
}

func TestWebReleaseRejectsUnsafeOrUnboundedFiles(t *testing.T) {
	t.Parallel()

	for _, example := range []struct {
		name   string
		change func(*testing.T, string)
	}{
		{name: "shell symlink", change: func(t *testing.T, root string) {
			t.Helper()

			replaceWebReleaseTestSymlink(t, root, "index.html", filepath.Join(root, "game.kart"))
		}},
		{name: "optional status symlink", change: func(t *testing.T, root string) {
			t.Helper()

			replaceWebReleaseTestSymlink(t, root, "karty-web-status.js", filepath.Join(root, "game.kart"))
		}},
		{name: "content directory symlink", change: func(t *testing.T, root string) {
			t.Helper()

			replaceWebReleaseTestSymlink(t, root, "content", t.TempDir())
		}},
		{name: "content file symlink", change: func(t *testing.T, root string) {
			t.Helper()

			name := webReleaseTestContent(t, root, ".kld", []byte("level"))
			replaceWebReleaseTestSymlink(t, root, name, filepath.Join(root, "game.kart"))
		}},
		{name: "release directory symlink", change: func(t *testing.T, root string) {
			t.Helper()

			replaceWebReleaseTestSymlink(t, root, "karty-releases", t.TempDir())
		}},
		{name: "pointer symlink", change: func(t *testing.T, root string) {
			t.Helper()

			replaceWebReleaseTestSymlink(t, root, "karty-latest.json", filepath.Join(root, "game.kart"))
		}},
		{name: "content name", change: func(t *testing.T, root string) {
			t.Helper()

			webReleaseTestWrite(t, root, "content/level.kld", []byte("level"))
		}},
		{name: "content digest", change: func(t *testing.T, root string) {
			t.Helper()

			webReleaseTestWrite(t, root, "content/"+strings.Repeat("a", 64)+".kld", []byte("level"))
		}},
		{name: "unrecognized content", change: func(t *testing.T, root string) {
			t.Helper()

			webReleaseTestWrite(t, root, "content/readme.txt", []byte("unrecognized"))
		}},
		{name: "oversized host", change: func(t *testing.T, root string) {
			t.Helper()

			file, err := os.OpenFile(filepath.Join(root, "karty-host.wasm"), os.O_WRONLY, 0o600)
			if err != nil {
				t.Fatal(err)
			}

			defer file.Close()

			if err := file.Truncate(webReleaseMaxFile + 1); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(example.name, func(t *testing.T) {
			t.Parallel()

			root := webReleaseTestFiles(t)
			example.change(t, root)

			if err := stageWebRelease(t.Context(), root, nil); err == nil {
				t.Fatal("unsafe release accepted")
			}
		})
	}
}

func TestWebReleasePreservesPointerOnCancellationOrConflict(t *testing.T) {
	t.Parallel()

	root := webReleaseTestFiles(t)

	if err := stageWebRelease(t.Context(), root, nil); err != nil {
		t.Fatal(err)
	}

	pointer, _, _ := readWebReleaseTest(t, root)
	canceled, cancel := context.WithCancel(t.Context())
	cancel()

	if err := stageWebRelease(canceled, root, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled build = %v", err)
	}

	webReleaseTestWrite(t, root, pointer.Manifest, []byte("immutable conflict"))

	if err := stageWebRelease(t.Context(), root, nil); err == nil {
		t.Fatal("immutable manifest conflict overwritten")
	}

	contents, err := os.ReadFile(filepath.Join(root, "karty-latest.json"))
	if err != nil {
		t.Fatal(err)
	}

	var after webReleasePointer

	if err := json.Unmarshal(contents, &after); err != nil || after != pointer {
		t.Fatalf("failed staging changed pointer: %+v, %v", after, err)
	}
}

func webReleaseTestFiles(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	for _, name := range []string{"index.html", "karty.js", "wasm_exec.js", "game.kart", "karty-web.js", "karty-host.wasm"} {
		webReleaseTestWrite(t, root, name, []byte(name))
	}

	return root
}

//nolint:gosec // Fixture writers use explicit caller-owned temporary output paths.
func webReleaseTestWrite(t *testing.T, root, name string, contents []byte) {
	t.Helper()

	name = filepath.Join(root, filepath.FromSlash(name))

	if err := os.MkdirAll(
		filepath.Dir(name),
		0o750,
	); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(name, contents, 0o600); err != nil {
		t.Fatal(err)
	}
}

func webReleaseTestContent(t *testing.T, root, extension string, contents []byte) string {
	t.Helper()

	digest := sha256.Sum256(contents)
	name := "content/" + hex.EncodeToString(digest[:]) + extension
	webReleaseTestWrite(t, root, name, contents)

	return name
}

func readWebReleaseTest(t *testing.T, root string) (webReleasePointer, webReleaseManifest, []byte) {
	t.Helper()

	contents, err := os.ReadFile(filepath.Join(root, "karty-latest.json"))
	if err != nil {
		t.Fatal(err)
	}

	var pointer webReleasePointer

	if err := json.Unmarshal(contents, &pointer); err != nil {
		t.Fatal(err)
	}

	if pointer.Manifest != "karty-releases/"+pointer.Release+".json" {
		t.Fatalf("unexpected manifest path: %+v", pointer)
	}

	contents, err = os.ReadFile(filepath.Join(root, filepath.FromSlash(pointer.Manifest)))
	if err != nil {
		t.Fatal(err)
	}

	digest := sha256.Sum256(contents)

	if pointer.Release != hex.EncodeToString(digest[:]) {
		t.Fatal("release identity does not hash exact manifest bytes")
	}

	var manifest webReleaseManifest

	if err := json.Unmarshal(contents, &manifest); err != nil {
		t.Fatal(err)
	}

	return pointer, manifest, contents
}

func replaceWebReleaseTestSymlink(t *testing.T, root, name, destination string) {
	t.Helper()

	name = filepath.Join(root, filepath.FromSlash(name))

	if err := os.Remove(name); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	if err := os.Symlink(destination, name); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
}
