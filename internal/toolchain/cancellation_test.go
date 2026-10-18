package toolchain

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty/internal/sdk"
)

type canceledArchiveReader struct {
	input  io.Reader
	cancel context.CancelFunc
	closed bool
}

func (reader *canceledArchiveReader) Read(contents []byte) (int, error) {
	n, err := reader.input.Read(contents)
	reader.cancel()

	return n, err
}

func (reader *canceledArchiveReader) Close() error {
	reader.closed = true

	return nil
}

func TestToolArchiveCancellationClosesStreamAndRemovesInstallStage(t *testing.T) {
	t.Parallel()
	archive := materialFixture(t, materialFixtureMember{name: "unused", body: "fixture payload", mode: 0o600})
	checksum := fmt.Sprintf("%x", sha256.Sum256(archive))

	for _, tool := range []string{"tinygo", "wasm-tools"} {
		t.Run(tool, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			stream := &canceledArchiveReader{input: bytes.NewReader(archive), cancel: cancel}
			home := t.TempDir()
			download := func(context.Context, string) (io.ReadCloser, error) { return stream, nil }
			artifact := sdk.ToolArtifact{URL: "https://example.invalid/archive", SHA256: checksum}

			var err error
			if tool == "tinygo" {
				_, err = InstallTinyGo(
					ctx,
					InstallTinyGoOptions{Version: "fixture", Artifact: artifact, CacheDir: home, Platform: "test", Download: download},
				)
			} else {
				_, err = InstallWasmTools(
					ctx,
					InstallWasmToolsOptions{Version: "fixture", Artifact: artifact, CacheDir: home, Platform: "test", Download: download},
				)
			}

			if !errors.Is(err, context.Canceled) || !stream.closed {
				t.Fatalf("install ignored cancellation/close: closed=%v err=%v", stream.closed, err)
			}

			entries, err := os.ReadDir(filepath.Join(home, "tools", tool, "fixture"))
			if err != nil || len(entries) != 0 {
				t.Fatalf("partial install/stage remains: %v %v", entries, err)
			}
		})
	}
}

func TestMaterialArchiveCancellationDoesNotPublish(t *testing.T) {
	t.Parallel()
	archive := materialFixture(t, materialFixtureMember{name: "bin/materialize-cli", body: "fixture", mode: 0o600})
	options := materialFixtureOptions(t, "linux-amd64", archive)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	stream := &canceledArchiveReader{input: bytes.NewReader(archive), cancel: cancel}

	options.Download = func(context.Context, string) (io.ReadCloser, error) { return stream, nil }
	if _, err := InstallMaterialTool(ctx, options); !errors.Is(err, context.Canceled) || !stream.closed {
		t.Fatalf("material install cancellation: closed=%v err=%v", stream.closed, err)
	}

	parent := filepath.Join(options.CacheDir, "tools", string(options.Spec.Tool), options.Spec.Revision)

	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatalf("partial material install/stage remains: %v %v", entries, err)
	}
}

func TestArchiveEntryCancellationClosesOutput(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	input := &canceledArchiveReader{input: bytes.NewReader(make([]byte, 1<<20)), cancel: cancel}

	name := filepath.Join(t.TempDir(), "entry")
	if err := writeArchiveFile(ctx, name, input, 1<<20, 0o600); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}

	info, err := os.Stat(name)
	if err != nil || info.Size() >= 1<<20 {
		t.Fatalf("entry copy completed after cancellation: %v %v", info, err)
	}

	if err := os.Remove(name); err != nil {
		t.Fatalf("canceled copy retained open output: %v", err)
	}
}
