package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/karty-game/karty/internal/toolchain"
)

const hostedRuntimeName = "hosted-runtime.json"
const hostedRuntimeFormat = "karty.hosted-runtime@1"

var hostedRuntimeHash = regexp.MustCompile(`^[a-f0-9]{64}$`)

type hostedRuntimeInventory struct {
	Format       string                       `json:"format"`
	BundleSHA256 string                       `json:"bundleSha256"`
	Files        map[string]hostedRuntimeFile `json:"files"`
}

type hostedRuntimeFile struct {
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// Only the site representation omits the byte-identical fallback alias. Source
// bundles are fully verified before their unchanged files are shared.
func shareSampleRuntime(site, sample, version string) error {
	directory := filepath.Join(site, sample)

	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}

	validationErr := validateLocalSampleWeb(root, sample)
	closeErr := root.Close()

	if validationErr != nil {
		return validationErr
	}

	if closeErr != nil {
		return closeErr
	}

	if !runtimeVersion.MatchString(version) {
		return fmt.Errorf("invalid hosted SDK version: %w", os.ErrInvalid)
	}

	bundle, err := toolchain.ReadHostBundle(context.Background(), directory, "web", "")
	if err != nil {
		return err
	}

	descriptor, err := os.ReadFile(filepath.Join(directory, toolchain.HostBundleName))
	if err != nil {
		return err
	}

	inventory := hostedRuntimeInventory{
		Format:       hostedRuntimeFormat,
		BundleSHA256: hashBytes(descriptor),
		Files:        map[string]hostedRuntimeFile{},
	}
	for name, file := range bundle.Files {
		if name != "runtime.bin" {
			inventory.Files[name] = hostedRuntimeFile{Size: file.Size, SHA256: file.SHA256}
		}
	}

	encoded, err := json.Marshal(inventory)
	if err != nil {
		return err
	}

	shared := path.Join("runtime", version, hashBytes(encoded))
	if err = writeHostedRuntime(site, directory, shared, encoded, inventory); err != nil {
		return err
	}

	return restageHostedSample(directory, "../"+shared+"/", inventory)
}

func writeHostedRuntime(site, directory, shared string, encoded []byte, inventory hostedRuntimeInventory) error {
	output, err := os.OpenRoot(site)
	if err != nil {
		return err
	}
	defer output.Close()

	for _, name := range []string{"runtime", path.Dir(shared), shared, path.Join(shared, "licenses")} {
		if err = output.Mkdir(name, 0o750); err != nil && !os.IsExist(err) {
			return err
		}

		if err = requireRuntimeDirectory(output, name); err != nil {
			return err
		}
	}

	if err := requireHostedRuntimeVersion(output, path.Dir(shared), path.Base(shared)); err != nil {
		return err
	}

	source, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer source.Close()

	for name := range inventory.Files {
		data, err := source.ReadFile(name)
		if err != nil {
			return err
		}

		target := path.Join(shared, name)
		if info, err := output.Lstat(target); err == nil && !info.Mode().IsRegular() {
			return fmt.Errorf("hosted runtime target is not regular: %w", os.ErrInvalid)
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}

		if err := output.WriteFile(target, data, 0o600); err != nil {
			return err
		}
	}

	descriptor := path.Join(shared, hostedRuntimeName)
	if info, err := output.Lstat(descriptor); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("hosted runtime inventory is not regular: %w", os.ErrInvalid)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	if err = output.WriteFile(descriptor, encoded, 0o600); err != nil {
		return err
	}

	return validateHostedRuntime(os.DirFS(filepath.Join(site, filepath.FromSlash(shared))), path.Base(shared))
}

func requireHostedRuntimeVersion(root *os.Root, directory, checksum string) error {
	entries, err := fs.ReadDir(root.FS(), directory)
	if err != nil {
		return err
	}

	if len(entries) != 1 || entries[0].Name() != checksum || !entries[0].IsDir() {
		return fmt.Errorf("one SDK version must use one exact hosted runtime: %w", os.ErrInvalid)
	}

	return nil
}

func validateHostedRuntime(source fs.FS, checksum string) error {
	file, err := source.Open(hostedRuntimeName)
	if err != nil {
		return err
	}

	data, readErr := io.ReadAll(io.LimitReader(file, toolchain.HostBundleDescriptorLimit+1))
	closeErr := file.Close()

	if readErr != nil {
		return readErr
	}

	if closeErr != nil {
		return closeErr
	}

	if len(data) > toolchain.HostBundleDescriptorLimit || hashBytes(data) != checksum {
		return fmt.Errorf("hosted runtime inventory checksum mismatch: %w", os.ErrInvalid)
	}

	inventory := hostedRuntimeInventory{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&inventory); err != nil {
		return err
	}

	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("hosted runtime inventory has trailing data: %w", os.ErrInvalid)
	}

	if inventory.Format != hostedRuntimeFormat || !hostedRuntimeHash.MatchString(inventory.BundleSHA256) {
		return fmt.Errorf("invalid hosted runtime format or source pin: %w", os.ErrInvalid)
	}

	required := hostedRuntimeFiles()
	if len(required) != len(inventory.Files) {
		return fmt.Errorf("hosted runtime inventory is incomplete: %w", os.ErrInvalid)
	}

	for _, name := range required {
		entry, ok := inventory.Files[name]
		if !ok || entry.Size <= 0 || entry.Size > maxSampleBytes || !hostedRuntimeHash.MatchString(entry.SHA256) {
			return fmt.Errorf("invalid hosted runtime file %s: %w", name, os.ErrInvalid)
		}

		if err := verifyHostedRuntimeFile(source, name, entry); err != nil {
			return err
		}
	}

	return fs.WalkDir(source, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if name == "." {
			return nil
		}

		if entry.IsDir() && name == "licenses" {
			return nil
		}

		if !entry.Type().IsRegular() || (name != hostedRuntimeName && !slices.Contains(required, name)) {
			return fmt.Errorf("unexpected hosted runtime entry %s: %w", name, os.ErrInvalid)
		}

		return nil
	})
}

func verifyHostedRuntimeFile(source fs.FS, name string, entry hostedRuntimeFile) error {
	file, err := source.Open(name)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := io.Reader(file)

	if name == "karty-host.wasm" {
		var header [8]byte
		if _, err := io.ReadFull(file, header[:]); err != nil {
			return err
		}

		if string(header[:]) != "\x00asm\x01\x00\x00\x00" {
			return fmt.Errorf("hosted runtime is not WASM: %w", os.ErrInvalid)
		}

		reader = io.MultiReader(bytes.NewReader(header[:]), file)
	}

	digest := sha256.New()
	count, readErr := io.Copy(digest, io.LimitReader(reader, entry.Size+1))
	closeErr := file.Close()

	if readErr != nil {
		return readErr
	}

	if closeErr != nil {
		return closeErr
	}

	if count != entry.Size || hex.EncodeToString(digest.Sum(nil)) != entry.SHA256 {
		return fmt.Errorf("hosted runtime file checksum mismatch: %s: %w", name, os.ErrInvalid)
	}

	return nil
}

func hostedRuntimeFiles() []string {
	names := slices.DeleteFunc(toolchain.WebGPUHostFiles(), func(name string) bool { return name == "runtime.bin" })
	names = append(names, toolchain.WebGPUTextureCodecFiles()...)
	names = append(names, "licenses/Karty-RUNTIME_LICENSE.md")

	return append(names, toolchain.WebGPUTextureCodecLicenses()...)
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:])
}
