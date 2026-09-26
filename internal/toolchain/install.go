package toolchain

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/karty-game/karty/internal/sdk"
)

// InstallTinyGoOptions identifies a TinyGo release and where to install it.
type InstallTinyGoOptions struct {
	Version  string
	Artifact sdk.ToolArtifact
	CacheDir string
	Platform string
	Download func(context.Context, string) (io.ReadCloser, error)
}

const maxArchiveSize = uint64(1<<63 - 1)

// InstallTinyGo downloads, verifies, and atomically installs TinyGo.
func InstallTinyGo(ctx context.Context, options InstallTinyGoOptions) (string, error) {
	if options.Version == "" || options.Artifact.URL == "" || options.Artifact.SHA256 == "" {
		return "", errTinyGoMetadata
	}

	cacheDir, err := kartyHome(options.CacheDir)
	if err != nil {
		return "", err
	}

	platform := options.Platform
	if platform == "" {
		platform = runtime.GOOS + "-" + runtime.GOARCH
	}

	installDir := filepath.Join(cacheDir, "tools", "tinygo", options.Version, platform)

	executablePath := filepath.Join(installDir, "tinygo", "bin", executableName("tinygo"))
	if managedPath, executableErr := executable("managed TinyGo", executablePath); executableErr == nil {
		return managedPath, nil
	}

	mkdirErr := os.MkdirAll(filepath.Dir(installDir), 0o750)
	if mkdirErr != nil {
		return "", fmt.Errorf("create TinyGo cache directory: %w", mkdirErr)
	}

	download := options.Download
	if download == nil {
		download = downloadURL
	}

	archive, err := download(ctx, options.Artifact.URL)
	if err != nil {
		return "", fmt.Errorf("download TinyGo %s: %w", options.Version, err)
	}
	defer archive.Close()

	staging, err := os.MkdirTemp(filepath.Dir(installDir), ".tinygo-")
	if err != nil {
		return "", fmt.Errorf("create TinyGo staging directory: %w", err)
	}
	defer os.RemoveAll(staging)

	extractErr := extractTinyGo(staging, archive, options.Artifact.SHA256)
	if extractErr != nil {
		return "", extractErr
	}

	if _, executableErr := executable(
		"downloaded TinyGo",
		filepath.Join(staging, "tinygo", "bin", executableName("tinygo")),
	); executableErr != nil {
		return "", executableErr
	}

	validateErr := validateTinyGo(ctx, filepath.Join(staging, "tinygo", "bin", executableName("tinygo")))
	if validateErr != nil {
		return "", validateErr
	}

	renameErr := os.Rename(staging, installDir)
	if renameErr != nil {
		return "", fmt.Errorf("install TinyGo: %w", renameErr)
	}

	return executable("managed TinyGo", executablePath)
}

func kartyHome(cacheDir string) (string, error) {
	if cacheDir != "" {
		return cacheDir, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve Karty home directory: %w", err)
	}

	return filepath.Join(home, ".karty"), nil
}

func downloadURL(ctx context.Context, url string) (io.ReadCloser, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		closeErr := response.Body.Close()
		if closeErr != nil {
			return nil, fmt.Errorf("close HTTP response: %w", closeErr)
		}

		return nil, fmt.Errorf("unexpected HTTP status %s: %w", response.Status, errUnexpectedHTTP)
	}

	return response.Body, nil
}

func validateTinyGo(ctx context.Context, path string) error {
	command := exec.CommandContext(ctx, path, "version")
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("validate downloaded TinyGo: %w\n%s", err, output)
	}

	return nil
}

func extractTinyGo(destination string, source io.Reader, expectedSHA256 string) error {
	contents, readErr := io.ReadAll(source)
	if readErr != nil {
		return fmt.Errorf("read TinyGo archive: %w", readErr)
	}

	actual := sha256.Sum256(contents)
	if hex.EncodeToString(actual[:]) != expectedSHA256 {
		return fmt.Errorf("TinyGo archive checksum = %x, want %s: %w", actual, expectedSHA256, errTinyGoChecksum)
	}

	if bytes.HasPrefix(contents, []byte("PK\x03\x04")) {
		return extractTinyGoZip(destination, contents)
	}

	return extractTinyGoTar(destination, contents)
}

func extractTinyGoTar(destination string, contents []byte) error {
	gzipReader, err := gzip.NewReader(bytes.NewReader(contents))
	if err != nil {
		return fmt.Errorf("open TinyGo archive: %w", err)
	}
	defer gzipReader.Close()

	return extractTinyGoTarEntries(destination, tar.NewReader(gzipReader))
}

func extractTinyGoTarEntries(destination string, reader *tar.Reader) error {
	for {
		header, nextErr := reader.Next()
		if nextErr == io.EOF {
			return nil
		}

		if nextErr != nil {
			return fmt.Errorf("read TinyGo archive: %w", nextErr)
		}

		if header.Name == "tinygo" && header.Typeflag == tar.TypeDir {
			continue
		}

		relative, valid := archiveRelativePath(header.Name, "tinygo")
		if !valid {
			return fmt.Errorf("invalid TinyGo archive path: %s: %w", header.Name, errInvalidTinyGoPath)
		}

		path := filepath.Join(destination, "tinygo", relative)
		if header.Typeflag == tar.TypeDir {
			if mkdirErr := os.MkdirAll(path, 0o750); mkdirErr != nil {
				return mkdirErr
			}

			continue
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		writeErr := writeArchiveFile(path, reader, header.Size, header.FileInfo().Mode())
		if writeErr != nil {
			return writeErr
		}
	}
}

func extractTinyGoZip(destination string, contents []byte) error {
	archive, err := zip.NewReader(bytes.NewReader(contents), int64(len(contents)))
	if err != nil {
		return fmt.Errorf("open TinyGo ZIP archive: %w", err)
	}

	for _, entry := range archive.File {
		relative, valid := archiveRelativePath(entry.Name, "tinygo")
		if !valid {
			return fmt.Errorf("invalid TinyGo archive path: %s: %w", entry.Name, errInvalidTinyGoPath)
		}

		path := filepath.Join(destination, "tinygo", relative)
		if entry.FileInfo().IsDir() {
			if mkdirErr := os.MkdirAll(path, 0o750); mkdirErr != nil {
				return mkdirErr
			}

			continue
		}

		if entry.UncompressedSize64 > maxArchiveSize {
			return fmt.Errorf("TinyGo archive entry is too large: %s: %w", entry.Name, errTinyGoEntryTooLarge)
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

	return nil
}

func archiveRelativePath(name, root string) (string, bool) {
	cleanName := filepath.Clean(name)
	if cleanName == root {
		return "", true
	}

	relative, ok := strings.CutPrefix(cleanName, root+"/")

	return relative, ok && relative != "." && relative != ".." && !filepath.IsAbs(relative) &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func writeArchiveFile(path string, source io.Reader, size int64, mode fs.FileMode) error {
	if mkdirErr := os.MkdirAll(filepath.Dir(path), 0o750); mkdirErr != nil {
		return mkdirErr
	}

	file, openErr := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if openErr != nil {
		return openErr
	}

	_, copyErr := io.CopyN(file, source, size)
	closeErr := file.Close()

	if copyErr != nil {
		return copyErr
	}

	return closeErr
}
