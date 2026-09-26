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

func TestInstallTinyGoInstallsVerifiedArchive(t *testing.T) {
	t.Parallel()
	archive := tinyGoArchive(t)
	sum := sha256.Sum256(archive)
	cacheDir := t.TempDir()

	path, err := toolchain.InstallTinyGo(context.Background(), toolchain.InstallTinyGoOptions{
		Version:  "0.42.0",
		Artifact: sdk.ToolArtifact{URL: "https://example.invalid/tinygo.tar.gz", SHA256: hex.EncodeToString(sum[:])},
		CacheDir: cacheDir,
		Platform: "test-platform",
		Download: func(context.Context, string) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(archive)), nil
		},
	})
	if err != nil {
		t.Fatalf("InstallTinyGo() error = %v", err)
	}

	want := filepath.Join(
		cacheDir,
		"tools",
		"tinygo",
		"0.42.0",
		"test-platform",
		"tinygo",
		"bin",
		testExecutableName("tinygo"),
	)
	if path != want {
		t.Errorf("InstallTinyGo() = %q, want %q", path, want)
	}
}

func TestInstallTinyGoRejectsInvalidChecksum(t *testing.T) {
	t.Parallel()
	archive := tinyGoArchive(t)

	_, err := toolchain.InstallTinyGo(context.Background(), toolchain.InstallTinyGoOptions{
		Version:  "0.42.0",
		Artifact: sdk.ToolArtifact{URL: "https://example.invalid/tinygo.tar.gz", SHA256: "invalid"},
		CacheDir: t.TempDir(),
		Platform: "test-platform",
		Download: func(context.Context, string) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(archive)), nil
		},
	})
	if err == nil {
		t.Fatal("InstallTinyGo() error = nil, want checksum error")
	}
}

func TestInstallTinyGoInstallsZIPArchive(t *testing.T) {
	t.Parallel()
	archive := tinyGoZipArchive(t)
	sum := sha256.Sum256(archive)

	path, err := toolchain.InstallTinyGo(context.Background(), toolchain.InstallTinyGoOptions{
		Version:  "0.42.0",
		Artifact: sdk.ToolArtifact{URL: "https://example.invalid/tinygo.zip", SHA256: hex.EncodeToString(sum[:])},
		CacheDir: t.TempDir(),
		Platform: "test-platform",
		Download: func(context.Context, string) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(archive)), nil
		},
	})
	if err != nil {
		t.Fatalf("InstallTinyGo() ZIP error = %v", err)
	}

	if path == "" {
		t.Fatal("InstallTinyGo() ZIP path is empty")
	}
}

func tinyGoArchive(t *testing.T) []byte {
	t.Helper()

	var archive bytes.Buffer

	gzipWriter := gzip.NewWriter(&archive)

	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{Name: "tinygo/", Typeflag: tar.TypeDir, Mode: 0o755}); err != nil {
		t.Fatal(err)
	}

	contents := []byte("#!/bin/sh\nexit 0\n")
	if err := tarWriter.WriteHeader(
		&tar.Header{Name: "tinygo/bin/tinygo", Mode: 0o755, Size: int64(len(contents))},
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

func tinyGoZipArchive(t *testing.T) []byte {
	t.Helper()

	var archive bytes.Buffer

	writer := zip.NewWriter(&archive)
	contents := []byte("#!/bin/sh\nexit 0\n")
	header := &zip.FileHeader{Name: "tinygo/bin/tinygo", Method: zip.Deflate}
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

func testExecutableName(name string) string {
	if filepath.Separator == '\\' {
		return name + ".exe"
	}

	return name
}
