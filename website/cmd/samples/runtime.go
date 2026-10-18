package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/toolchain"
)

var runtimeVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
var runtimeFile = regexp.MustCompile(`^[a-f0-9]{64}\.wasm$`)

// CDN samples share a raw host from the signed public release. A local engine
// override cannot change the bytes in this mirror.
func mirrorSampleHost(site, version string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), noticeTimeout)
	defer cancel()

	manifest, err := sdk.ResolvePublished(ctx, version)
	if err != nil {
		return "", err
	}

	source, err := toolchain.EnsureHost(ctx, manifest, "web")
	if err != nil {
		return "", err
	}

	return copyRuntimeHost(site, version, manifest.Artifacts.Host.Web.SHA256, source)
}

func copyRuntimeHost(site, version, checksum, source string) (string, error) {
	name := checksum + ".wasm"
	if !runtimeVersion.MatchString(version) || !runtimeFile.MatchString(name) {
		return "", fmt.Errorf("invalid shared runtime version or checksum: %w", os.ErrInvalid)
	}

	input, err := os.OpenRoot(filepath.Dir(source))
	if err != nil {
		return "", err
	}
	defer input.Close()

	if info, err := input.Lstat(filepath.Base(source)); err != nil {
		return "", err
	} else if !info.Mode().IsRegular() || info.Size() > maxSampleBytes {
		return "", fmt.Errorf("invalid runtime source: %w", os.ErrInvalid)
	}

	contents, err := input.ReadFile(filepath.Base(source))
	if err != nil {
		return "", err
	}

	if err = verifyRuntime(bytes.NewReader(contents), checksum); err != nil {
		return "", err
	}

	output, err := os.OpenRoot(site)
	if err != nil {
		return "", err
	}
	defer output.Close()

	if err = output.Mkdir("runtime", 0o755); err != nil && !os.IsExist(err) {
		return "", err
	}

	if err = requireRuntimeDirectory(output, "runtime"); err != nil {
		return "", err
	}

	if err = output.Mkdir(filepath.Join("runtime", version), 0o755); err != nil && !os.IsExist(err) {
		return "", err
	}

	if err = requireRuntimeDirectory(output, filepath.Join("runtime", version)); err != nil {
		return "", err
	}

	path := filepath.Join("runtime", version, name)
	if info, err := output.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return "", fmt.Errorf("runtime output must be a regular file: %w", os.ErrInvalid)
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	if err = output.WriteFile(path, contents, 0o600); err != nil {
		return "", err
	}

	return "../runtime/" + version + "/" + name, nil
}

func requireRuntimeDirectory(root *os.Root, path string) error {
	info, err := root.Lstat(path)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		return fmt.Errorf("runtime output directory must not be linked: %w", os.ErrInvalid)
	}

	return nil
}

func validateRuntime(source fs.FS) error {
	versions, err := fs.ReadDir(source, "runtime")
	if err != nil {
		return err
	}

	if len(versions) == 0 {
		return fmt.Errorf("shared runtime has no versions: %w", os.ErrInvalid)
	}

	for _, version := range versions {
		if !version.IsDir() || !runtimeVersion.MatchString(version.Name()) {
			return fmt.Errorf("invalid shared runtime version: %w", os.ErrInvalid)
		}

		files, err := fs.ReadDir(source, "runtime/"+version.Name())
		if err != nil {
			return err
		}

		if len(files) != 1 || !files[0].Type().IsRegular() || !runtimeFile.MatchString(files[0].Name()) {
			return fmt.Errorf("shared runtime must contain one checksum-named WASM host: %w", os.ErrInvalid)
		}

		file, err := source.Open("runtime/" + version.Name() + "/" + files[0].Name())
		if err != nil {
			return err
		}

		verifyErr := verifyRuntime(file, strings.TrimSuffix(files[0].Name(), ".wasm"))
		closeErr := file.Close()

		if verifyErr != nil {
			return verifyErr
		}

		if closeErr != nil {
			return closeErr
		}
	}

	return nil
}

func verifyRuntime(reader io.Reader, checksum string) error {
	var header [8]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return err
	}

	if string(header[:]) != "\x00asm\x01\x00\x00\x00" {
		return fmt.Errorf("shared runtime is not a raw WASM host: %w", os.ErrInvalid)
	}

	digest := sha256.New()
	_, _ = digest.Write(header[:])

	count, err := io.Copy(digest, io.LimitReader(reader, maxSampleBytes+1))
	if err != nil {
		return err
	}

	if count+int64(len(header)) > maxSampleBytes || hex.EncodeToString(digest.Sum(nil)) != checksum {
		return fmt.Errorf("shared runtime checksum mismatch or size exceeded: %w", os.ErrInvalid)
	}

	return nil
}
