package toolchain_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/toolchain"
)

func TestInstallWasmToolsInstallsVerifiedArchive(t *testing.T) {
	t.Parallel()
	archive := wasmToolsArchive(t)
	sum := sha256.Sum256(archive)
	cacheDir := t.TempDir()

	path, err := toolchain.InstallWasmTools(context.Background(), toolchain.InstallWasmToolsOptions{
		Version: "1.259.0",
		Artifact: sdk.ToolArtifact{
			URL:    "https://example.invalid/wasm-tools.tar.gz",
			SHA256: hex.EncodeToString(sum[:]),
		},
		CacheDir: cacheDir,
		Platform: "test-platform",
		Download: func(context.Context, string) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(archive)), nil
		},
	})
	if err != nil {
		t.Fatalf("InstallWasmTools() error = %v", err)
	}

	want := filepath.Join(cacheDir, "tools", "wasm-tools", "1.259.0", "test-platform", testExecutableName("wasm-tools"))
	if path != want {
		t.Errorf("InstallWasmTools() = %q, want %q", path, want)
	}
}

func TestInstallWasmToolsInstallsZIPArchive(t *testing.T) {
	t.Parallel()
	archive := wasmToolsZipArchive(t)
	sum := sha256.Sum256(archive)

	path, err := toolchain.InstallWasmTools(context.Background(), toolchain.InstallWasmToolsOptions{
		Version:  "1.259.0",
		Artifact: sdk.ToolArtifact{URL: "https://example.invalid/wasm-tools.zip", SHA256: hex.EncodeToString(sum[:])},
		CacheDir: t.TempDir(),
		Platform: "test-platform",
		Download: func(context.Context, string) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(archive)), nil
		},
	})
	if err != nil {
		t.Fatalf("InstallWasmTools() ZIP error = %v", err)
	}

	if path == "" {
		t.Fatal("InstallWasmTools() ZIP path is empty")
	}
}

func wasmToolsArchive(t *testing.T) []byte {
	t.Helper()

	var archive bytes.Buffer

	gzipWriter := gzip.NewWriter(&archive)
	tarWriter := tar.NewWriter(gzipWriter)

	contents := []byte("#!/bin/sh\nexit 0\n")
	if err := tarWriter.WriteHeader(
		&tar.Header{Name: "wasm-tools-1.259.0-x86_64-linux/wasm-tools", Mode: 0o755, Size: int64(len(contents))},
	); err != nil {
		t.Fatal(err)
	}

	if _, err := tarWriter.Write(contents); err != nil {
		t.Fatal(err)
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}

	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}

	return archive.Bytes()
}

func wasmToolsZipArchive(t *testing.T) []byte {
	t.Helper()

	var archive bytes.Buffer

	writer := zip.NewWriter(&archive)
	contents := []byte("#!/bin/sh\nexit 0\n")
	header := &zip.FileHeader{Name: "wasm-tools-1.259.0-x86_64-windows/wasm-tools", Method: zip.Deflate}
	header.SetMode(0o755)

	entry, err := writer.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := entry.Write(contents); err != nil {
		t.Fatal(err)
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	return archive.Bytes()
}
