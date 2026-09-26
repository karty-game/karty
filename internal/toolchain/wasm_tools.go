package toolchain

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/karty-game/karty/internal/sdk"
)

// InstallWasmToolsOptions identifies a wasm-tools release and where to install it.
type InstallWasmToolsOptions struct {
	Version  string
	Artifact sdk.ToolArtifact
	CacheDir string
	Platform string
	Download func(context.Context, string) (io.ReadCloser, error)
}

// WasmTools returns the SDK-pinned wasm-tools executable from the Karty cache.
func WasmTools(version string) (string, error) {
	if version == "" {
		return "", errWasmToolsVersion
	}

	cacheDir, err := kartyHome("")
	if err != nil {
		return "", err
	}

	path := filepath.Join(
		cacheDir,
		"tools",
		"wasm-tools",
		version,
		runtime.GOOS+"-"+runtime.GOARCH,
		executableName("wasm-tools"),
	)

	return executable("managed wasm-tools", path)
}

// InstallWasmTools downloads, verifies, and atomically installs wasm-tools.
func InstallWasmTools(ctx context.Context, options InstallWasmToolsOptions) (string, error) {
	if options.Version == "" || options.Artifact.URL == "" || options.Artifact.SHA256 == "" {
		return "", errWasmToolsMetadata
	}

	cacheDir, err := kartyHome(options.CacheDir)
	if err != nil {
		return "", err
	}

	platform := options.Platform
	if platform == "" {
		platform = runtime.GOOS + "-" + runtime.GOARCH
	}

	installDir := filepath.Join(cacheDir, "tools", "wasm-tools", options.Version, platform)

	executablePath := filepath.Join(installDir, executableName("wasm-tools"))
	if managedPath, executableErr := executable("managed wasm-tools", executablePath); executableErr == nil {
		return managedPath, nil
	}

	mkdirErr := os.MkdirAll(filepath.Dir(installDir), 0o750)
	if mkdirErr != nil {
		return "", fmt.Errorf("create wasm-tools cache directory: %w", mkdirErr)
	}

	download := options.Download
	if download == nil {
		download = downloadURL
	}

	archive, err := download(ctx, options.Artifact.URL)
	if err != nil {
		return "", fmt.Errorf("download wasm-tools %s: %w", options.Version, err)
	}
	defer archive.Close()

	staging, err := os.MkdirTemp(filepath.Dir(installDir), ".wasm-tools-")
	if err != nil {
		return "", fmt.Errorf("create wasm-tools staging directory: %w", err)
	}
	defer os.RemoveAll(staging)

	extractErr := extractWasmTools(staging, archive, options.Artifact.SHA256)
	if extractErr != nil {
		return "", extractErr
	}

	validateErr := validateWasmTools(ctx, filepath.Join(staging, executableName("wasm-tools")))
	if validateErr != nil {
		return "", validateErr
	}

	renameErr := os.Rename(staging, installDir)
	if renameErr != nil {
		return "", fmt.Errorf("install wasm-tools: %w", renameErr)
	}

	return executable("managed wasm-tools", executablePath)
}

func extractWasmTools(destination string, source io.Reader, expectedSHA256 string) error {
	contents, readErr := io.ReadAll(source)
	if readErr != nil {
		return fmt.Errorf("read wasm-tools archive: %w", readErr)
	}

	actual := sha256.Sum256(contents)
	if hex.EncodeToString(actual[:]) != expectedSHA256 {
		return fmt.Errorf("wasm-tools archive checksum = %x, want %s: %w", actual, expectedSHA256, errWasmToolsChecksum)
	}

	if bytes.HasPrefix(contents, []byte("PK\x03\x04")) {
		return extractWasmToolsZip(destination, contents)
	}

	gzipReader, gzipErr := gzip.NewReader(bytes.NewReader(contents))
	if gzipErr != nil {
		return fmt.Errorf("open wasm-tools archive: %w", gzipErr)
	}
	defer gzipReader.Close()

	reader := tar.NewReader(gzipReader)
	for {
		header, nextErr := reader.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}

		if nextErr != nil {
			return fmt.Errorf("read wasm-tools archive: %w", nextErr)
		}

		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != executableName("wasm-tools") {
			continue
		}

		path := filepath.Join(destination, executableName("wasm-tools"))

		writeErr := writeArchiveFile(path, reader, header.Size, header.FileInfo().Mode())
		if writeErr != nil {
			return writeErr
		}
	}

	if _, executableErr := executable(
		"downloaded wasm-tools",
		filepath.Join(destination, executableName("wasm-tools")),
	); executableErr != nil {
		return executableErr
	}

	return nil
}

func extractWasmToolsZip(destination string, contents []byte) error {
	archive, err := zip.NewReader(bytes.NewReader(contents), int64(len(contents)))
	if err != nil {
		return fmt.Errorf("open wasm-tools ZIP archive: %w", err)
	}

	for _, entry := range archive.File {
		if entry.FileInfo().IsDir() || filepath.Base(entry.Name) != executableName("wasm-tools") {
			continue
		}

		path := filepath.Join(destination, executableName("wasm-tools"))

		if entry.UncompressedSize64 > maxArchiveSize {
			return fmt.Errorf("wasm-tools archive entry is too large: %s: %w", entry.Name, errWasmEntryTooLarge)
		}

		input, openErr := entry.Open()
		if openErr != nil {
			return openErr
		}

		copyErr := writeArchiveFile(path, input, int64(entry.UncompressedSize64), entry.Mode())
		closeErr := input.Close()

		if copyErr != nil {
			return copyErr
		}

		if closeErr != nil {
			return closeErr
		}
	}

	if _, executableErr := executable(
		"downloaded wasm-tools",
		filepath.Join(destination, executableName("wasm-tools")),
	); executableErr != nil {
		return executableErr
	}

	return nil
}

func validateWasmTools(ctx context.Context, path string) error {
	command := exec.CommandContext(ctx, path, "--version")
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("validate downloaded wasm-tools: %w\n%s", err, output)
	}

	return nil
}
