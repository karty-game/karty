package toolchain

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/karty-game/karty/internal/contextio"
	"github.com/karty-game/karty/internal/process"
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
	if err := ctx.Err(); err != nil {
		return "", err
	}

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

	extractErr := extractTinyGo(ctx, staging, archive, options.Artifact.SHA256)
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

	if err := ctx.Err(); err != nil {
		return "", err
	}

	renameErr := os.Rename(staging, installDir)
	if renameErr != nil {
		return "", fmt.Errorf("install TinyGo: %w", renameErr)
	}

	return executable("managed TinyGo", executablePath)
}

func kartyHome(cacheDir string) (string, error) {
	return kartyHomeWithEnv(cacheDir, os.Getenv)
}

func kartyHomeWithEnv(cacheDir string, getenv func(string) string) (string, error) {
	if cacheDir == "" {
		cacheDir = getenv("KARTY_HOME")
	}

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
	command := process.CommandContext(ctx, path, "version")
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("validate downloaded TinyGo: %w\n%s", err, output)
	}

	return nil
}

func extractTinyGo(ctx context.Context, destination string, source io.Reader, expectedSHA256 string) error {
	contents, readErr := contextio.ReadAll(ctx, source)
	if readErr != nil {
		return fmt.Errorf("read TinyGo archive: %w", readErr)
	}

	actual, hashErr := contextio.Sum256(ctx, contents)
	if hashErr != nil {
		return hashErr
	}

	if hex.EncodeToString(actual[:]) != expectedSHA256 {
		return fmt.Errorf("TinyGo archive checksum = %x, want %s: %w", actual, expectedSHA256, errTinyGoChecksum)
	}

	if bytes.HasPrefix(contents, []byte("PK\x03\x04")) {
		return extractTinyGoZip(ctx, destination, contents)
	}

	return extractTinyGoTar(ctx, destination, contents)
}

func extractTinyGoTar(ctx context.Context, destination string, contents []byte) error {
	gzipReader, err := gzip.NewReader(contextio.Reader(ctx, bytes.NewReader(contents)))
	if err != nil {
		return fmt.Errorf("open TinyGo archive: %w", err)
	}
	defer gzipReader.Close()

	return extractTinyGoTarEntries(ctx, destination, tar.NewReader(contextio.Reader(ctx, gzipReader)))
}

func extractTinyGoTarEntries(ctx context.Context, destination string, reader *tar.Reader) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

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

		writeErr := writeArchiveFile(ctx, path, reader, header.Size, header.FileInfo().Mode())
		if writeErr != nil {
			return writeErr
		}
	}
}

func extractTinyGoZip(ctx context.Context, destination string, contents []byte) error {
	archive, err := zip.NewReader(bytes.NewReader(contents), int64(len(contents)))
	if err != nil {
		return fmt.Errorf("open TinyGo ZIP archive: %w", err)
	}

	for _, entry := range archive.File {
		if err := ctx.Err(); err != nil {
			return err
		}

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

		copyErr := writeArchiveFile(ctx, path, input, int64(entry.UncompressedSize64), entry.Mode())
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
	// Archive paths always use forward slashes. Reject Windows separators and
	// volume/stream syntax before converting to a local filesystem path.
	if strings.ContainsAny(name, `\:`) {
		return "", false
	}

	cleanName := path.Clean(name)
	if cleanName == root {
		return "", true
	}

	relative, ok := strings.CutPrefix(cleanName, root+"/")
	if !ok {
		return "", false
	}

	local, err := filepath.Localize(relative)

	return local, err == nil
}

func writeArchiveFile(ctx context.Context, path string, source io.Reader, size int64, mode fs.FileMode) error {
	if mkdirErr := os.MkdirAll(filepath.Dir(path), 0o750); mkdirErr != nil {
		return mkdirErr
	}

	file, openErr := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if openErr != nil {
		return openErr
	}

	_, copyErr := contextio.CopyN(ctx, file, source, size)
	closeErr := file.Close()

	if copyErr != nil {
		return copyErr
	}

	return closeErr
}
