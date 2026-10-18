package devserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/cartridge"
)

func fixtureBuild(t *testing.T, root string, levelBytes, code []byte, features []string) {
	t.Helper()

	manifest, err := cartridge.EncodeManifest(
		cartridge.Manifest{
			ProjectName: "test",
			Compiler:    "go",
			Width:       640,
			Height:      360,
			Features:    features,
			Levels: []cartridge.LevelDependency{
				{Name: "levels.room", Kind: "world", ContentSHA256: digest(levelBytes), Size: uint64(len(levelBytes)), EnvelopeVersion: 1},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	wasm, err := cartridge.EmbedSection([]byte("\x00asm\x01\x00\x00\x00"), "test.code", code)
	if err != nil {
		t.Fatal(err)
	}

	wasm, err = cartridge.EmbedSection(wasm, cartridge.ManifestSectionName, manifest)
	if err != nil {
		t.Fatal(err)
	}

	// Match the actual build shell's eight-byte cache tokens. Full-digest-only
	// fixtures missed a real reload regression during a light-color-only edit.
	launcher := []byte(`fetch("game.kart?v=` + digest(wasm)[:shellHashLength] + `")`)

	files := map[string][]byte{
		"game.kart":       wasm,
		"karty.js":        launcher,
		"wasm_exec.js":    []byte("runtime"),
		"karty-host.wasm": []byte("host"),
		"index.html": []byte(
			`<html><head></head><body><script src="karty.js?v=` + digest(launcher)[:shellHashLength] + `"></script></body></html>`,
		),
		"content/" + digest(levelBytes) + ".kld": levelBytes,
	}
	for name, data := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func captureFixture(t *testing.T, root string, revision uint64) *Snapshot {
	t.Helper()

	snapshot, err := Capture(t.Context(), root, revision)
	if err != nil {
		t.Fatal(err)
	}

	return snapshot
}

func TestRuntimeIdentityAllowsOnlyLevelContentChanges(t *testing.T) {
	t.Parallel()

	firstRoot, secondRoot := t.TempDir(), t.TempDir()
	fixtureBuild(t, firstRoot, []byte("old level"), []byte("game code"), nil)
	fixtureBuild(t, secondRoot, []byte("longer edited level"), []byte("game code"), nil)

	first, second := captureFixture(t, firstRoot, 1), captureFixture(t, secondRoot, 2)
	if first.Generation.ID == second.Generation.ID || first.Generation.RuntimeIdentity != second.Generation.RuntimeIdentity {
		t.Fatal("level content must change generation without changing runtime")
	}

	for _, change := range []string{"code", "host", "glue", "launcher", "html", "feature", "sound"} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			code := []byte("game code")

			var features []string

			if change == "code" {
				code = []byte("different code")
			}

			if change == "feature" {
				features = []string{cartridge.FeatureTextureQOIv1}
			}

			fixtureBuild(t, root, []byte("old level"), code, features)

			changeRuntimeFixture(t, root, change)

			got := captureFixture(t, root, 2)
			if got.Generation.RuntimeIdentity == first.Generation.RuntimeIdentity {
				t.Fatal("runtime change was classified as level-only")
			}
		})
	}
}

func TestCaptureConfinesAndVerifiesArtifacts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fixtureBuild(t, root, []byte("level"), []byte("code"), nil)
	original := captureFixture(t, root, 1)

	path := filepath.Join(root, original.Generation.Levels[0].Artifact)
	if err := os.WriteFile(path, []byte("wrong"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Capture(t.Context(), root, 2); !errors.Is(err, ErrSnapshot) {
		t.Fatalf("corrupt level accepted: %v", err)
	}

	if !bytes.Equal(original.files[original.Generation.Levels[0].Artifact], []byte("level")) {
		t.Fatal("snapshot changed with dist")
	}

	if err := os.WriteFile(path, []byte("level"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(t.TempDir(), filepath.Join(root, "outside")); err != nil {
		t.Fatal(err)
	}

	if _, err := Capture(t.Context(), root, 2); !errors.Is(err, ErrSnapshot) {
		t.Fatalf("symlink accepted: %v", err)
	}
}

func TestCaptureCancellationAndFileBounds(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fixtureBuild(t, root, []byte("level"), []byte("code"), nil)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := Capture(ctx, root, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation ignored: %v", err)
	}

	file, err := os.Create(filepath.Join(root, "oversized"))
	if err != nil {
		t.Fatal(err)
	}

	if err := file.Truncate(maxArtifactBytes + 1); err != nil {
		t.Fatal(err)
	}

	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := Capture(t.Context(), root, 2); !errors.Is(err, ErrSnapshot) {
		t.Fatalf("oversized file accepted: %v", err)
	}
}

func TestNormalizeManifestRetainsOtherCustomSections(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fixtureBuild(t, root, []byte("level"), []byte(strings.Repeat("karty.manifest.v1", 20)), nil)
	captureFixture(t, root, 1)
}

func TestShellIdentityNormalizesOnlyCompleteKnownHashes(t *testing.T) {
	t.Parallel()

	checksum := digest([]byte("cartridge"))
	for _, token := range []string{checksum, checksum[:shellHashLength]} {
		got, err := normalizeShellHash(t.Context(), []byte(`fetch("game.kart?v=`+token+`")`), "game.kart", checksum)
		if err != nil {
			t.Fatal(err)
		}

		if string(got) != `fetch("game.kart?v=CONTENT")` {
			t.Fatalf("known cache key was retained: %s", got)
		}
	}

	for _, token := range []string{checksum + "extra", checksum[:shellHashLength] + "extra", strings.Repeat("f", shellHashLength)} {
		original := []byte(`fetch("game.kart?v=` + token + `")`)

		got, err := normalizeShellHash(t.Context(), original, "game.kart", checksum)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(got, original) {
			t.Fatal("unrecognized shell content was normalized")
		}
	}
}

func changeRuntimeFixture(t *testing.T, root, change string) {
	t.Helper()

	name := map[string]string{"host": "karty-host.wasm", "glue": "wasm_exec.js", "launcher": "karty.js", "html": "index.html"}[change]
	if name != "" {
		file, err := os.OpenFile(filepath.Join(root, name), os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := file.WriteString("changed"); err != nil {
			t.Fatal(err)
		}

		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}

	if change == "sound" {
		if err := os.WriteFile(filepath.Join(root, "sound.qoa"), []byte("sound"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:])
}
