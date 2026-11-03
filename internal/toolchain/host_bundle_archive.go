package toolchain

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/karty-game/karty/internal/contextio"
)

func extractManagedHostBundle(ctx context.Context, directory string, contents []byte) error {
	archive, err := zip.NewReader(bytes.NewReader(contents), int64(len(contents)))
	if err != nil {
		return err
	}

	if err := validateHostBundleArchive(archive); err != nil {
		return err
	}

	for _, entry := range archive.File {
		if err := ctx.Err(); err != nil {
			return err
		}

		if !entry.FileInfo().IsDir() {
			if err := extractHostBundleEntry(ctx, directory, entry); err != nil {
				return err
			}
		}
	}

	return nil
}

// Validate every member before extracting any prefix into the private staging directory.
func validateHostBundleArchive(archive *zip.Reader) error {
	if len(archive.File) == 0 || len(archive.File) > 2*HostBundleCountLimit+1 {
		return fmt.Errorf("host bundle ZIP member count: %w", os.ErrInvalid)
	}

	seen := make(map[string]bool, len(archive.File))

	var total uint64

	files := 0

	for _, entry := range archive.File {
		name := strings.TrimSuffix(entry.Name, "/")
		if !validBundlePath(name) || name == "" || len(name) > 1024 || seen[strings.ToLower(name)] || entry.Mode()&os.ModeSymlink != 0 ||
			(!entry.FileInfo().IsDir() && !entry.Mode().IsRegular()) {
			return fmt.Errorf("invalid host bundle ZIP member %q: %w", entry.Name, os.ErrInvalid)
		}

		seen[strings.ToLower(name)] = true

		if entry.FileInfo().IsDir() {
			if entry.UncompressedSize64 != 0 {
				return os.ErrInvalid
			}

			continue
		}

		files++

		limit := uint64(HostBundleFileLimit)
		if name == HostBundleName {
			limit = HostBundleDescriptorLimit
		}

		if files > HostBundleCountLimit+1 || entry.UncompressedSize64 == 0 || entry.UncompressedSize64 > limit ||
			entry.UncompressedSize64 > HostBundleTotalLimit+HostBundleDescriptorLimit-total {
			return fmt.Errorf("host bundle ZIP decoded size: %w", os.ErrInvalid)
		}

		total += entry.UncompressedSize64
	}

	return nil
}

// The complete archive admission above owns path and decoded-allocation bounds.
func extractHostBundleEntry(ctx context.Context, directory string, entry *zip.File) error {
	name := entry.Name

	destination := filepath.Join(directory, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
		return err
	}

	input, err := entry.Open()
	if err != nil {
		return err
	}

	mode := os.FileMode(0o600)
	if entry.Mode()&0o111 != 0 {
		mode = 0o700
	}

	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		input.Close()

		return err
	}

	size, copyErr := contextio.Copy(ctx, output, io.LimitReader(input, int64(entry.UncompressedSize64)+1))
	inputErr, outputErr := input.Close(), output.Close()

	if copyErr != nil {
		return copyErr
	}

	if inputErr != nil {
		return inputErr
	}

	if outputErr != nil {
		return outputErr
	}

	if uint64(size) != entry.UncompressedSize64 {
		return os.ErrInvalid
	}

	return nil
}

// A cache descriptor is not itself a trust anchor. Compare it with the exact
// descriptor in the retained, signature-pinned archive before trusting hashes.
func matchManagedDescriptor(ctx context.Context, directory string, contents []byte) error {
	archive, err := zip.NewReader(bytes.NewReader(contents), int64(len(contents)))
	if err != nil {
		return err
	}

	var expected []byte

	for _, entry := range archive.File {
		if entry.Name != HostBundleName {
			continue
		}

		if expected != nil || entry.UncompressedSize64 > HostBundleDescriptorLimit {
			return os.ErrInvalid
		}

		input, err := entry.Open()
		if err != nil {
			return err
		}

		expected, err = contextio.ReadAll(ctx, io.LimitReader(input, HostBundleDescriptorLimit+1))
		closeErr := input.Close()

		if err != nil {
			return err
		}

		if closeErr != nil {
			return closeErr
		}
	}

	if len(expected) == 0 || len(expected) > HostBundleDescriptorLimit {
		return os.ErrInvalid
	}

	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()

	info, err := root.Lstat(HostBundleName)
	if err != nil || !info.Mode().IsRegular() || info.Size() != int64(len(expected)) {
		return errHostChecksum
	}

	file, err := root.Open(HostBundleName)
	if err != nil {
		return err
	}

	actual, readErr := contextio.ReadAll(ctx, io.LimitReader(file, int64(len(expected))+1))
	closeErr := file.Close()

	if readErr != nil {
		return readErr
	}

	if closeErr != nil {
		return closeErr
	}

	if !bytes.Equal(actual, expected) {
		return errHostChecksum
	}

	return nil
}

func verifyManagedHostBundle(ctx context.Context, directory, target string) (*HostBundle, error) {
	mode, platform := "native", target
	if target == "web" {
		mode, platform = "web", ""
	}

	bundle, err := ReadHostBundle(ctx, directory, mode, platform)
	if err != nil {
		return nil, err
	}

	if bundle == nil {
		return nil, errHostMetadata
	}

	if target != "web" {
		for _, name := range []string{"licenses/Karty-RUNTIME_LICENSE.md", "licenses/SDL-LICENSE.txt", "licenses/wgpu-native-LICENSE-APACHE.txt", "licenses/wgpu-native-LICENSE-MIT.txt", "licenses/Go-WebGPU-LICENSE.txt", "licenses/go-sdl3-LICENSE.txt"} {
			if _, ok := bundle.Files[name]; !ok {
				return nil, fmt.Errorf("managed host bundle missing license %s: %w", name, os.ErrInvalid)
			}
		}

		if strings.HasPrefix(target, "windows-") {
			if _, ok := bundle.Files["dxcompiler.dll"]; !ok {
				return nil, fmt.Errorf("managed Windows host bundle missing DXC: %w", os.ErrInvalid)
			}
		}
	}
	// Reject undeclared archive/cache files, even when their paths are otherwise
	// safe. Only the complete signed inventory may reach distribution staging.
	err = filepath.WalkDir(directory, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		if entry.Type()&os.ModeSymlink != 0 {
			return os.ErrInvalid
		}

		if entry.IsDir() {
			return nil
		}

		relative, err := filepath.Rel(directory, filename)
		if err != nil {
			return err
		}

		name := filepath.ToSlash(relative)
		if name == HostBundleName {
			return nil
		}

		if _, ok := bundle.Files[name]; !ok {
			return fmt.Errorf("undeclared host bundle file %s: %w", path.Clean(name), os.ErrInvalid)
		}

		return nil
	})

	return bundle, err
}
