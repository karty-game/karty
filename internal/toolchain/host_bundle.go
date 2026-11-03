package toolchain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/karty-game/karty/internal/contextio"
	"github.com/pelletier/go-toml/v2"
)

const HostBundleName = "host-bundle.toml"
const HostBundleDescriptorLimit = 128 << 10
const HostBundleFileLimit = 256 << 20
const HostBundleTotalLimit = 512 << 20
const HostBundleCountLimit = 128

type HostBundleFile struct {
	Size   int64  `toml:"size"`
	SHA256 string `toml:"sha256"`
}

type HostBundle struct {
	Schema     int                       `toml:"schema"`
	Renderer   string                    `toml:"renderer"`
	Target     string                    `toml:"target"`
	Entrypoint string                    `toml:"entrypoint"`
	Files      map[string]HostBundleFile `toml:"files"`
	Directory  string                    `toml:"-"`
}

// ValidateLocalHost validates an explicitly selected bundle without executing it.
// An unset selection is the only optional case.
func ValidateLocalHost(ctx context.Context, selected, target, platform string) (bool, error) {
	bundle, err := ReadHostBundle(ctx, selected, target, platform)

	return bundle != nil, err
}

// ResolveLocalHostBundle returns a verified bundle's executable entrypoint.
// Explicit single-file hosts without a complete descriptor are rejected.
func ResolveLocalHostBundle(ctx context.Context, selected, target, platform string) (string, bool, error) {
	bundle, err := ReadHostBundle(ctx, selected, target, platform)
	if err != nil || bundle == nil {
		return selected, false, err
	}

	return filepath.Join(bundle.Directory, bundle.Entrypoint), true, nil
}

// ReadHostBundle validates an explicit directory or entrypoint using its complete inventory.
// A nil bundle and nil error identify an unset selection.
//
//nolint:nilnil // An unset selection is resolved from the signed SDK later.
func ReadHostBundle(ctx context.Context, selected, target, platform string) (*HostBundle, error) {
	if selected == "" {
		return nil, nil
	}

	info, err := os.Lstat(selected)
	if err != nil {
		return nil, err
	}

	directory := selected
	if !info.IsDir() {
		directory = filepath.Dir(selected)
	}

	descriptor := filepath.Join(directory, HostBundleName)
	if _, err := os.Lstat(descriptor); os.IsNotExist(err) && !info.IsDir() {
		return nil, fmt.Errorf("local host selection requires a verified host-bundle@1 host with %s: %w", HostBundleName, os.ErrInvalid)
	} else if err != nil {
		return nil, fmt.Errorf("local host bundle descriptor: %w", err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("local host bundle cannot be a symlink: %w", os.ErrInvalid)
	}

	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	bundle, err := decodeHostBundle(ctx, root)
	if err != nil {
		return nil, err
	}

	bundle.Directory = directory

	if !info.IsDir() && filepath.Base(selected) != bundle.Entrypoint {
		return nil, fmt.Errorf("select the bundle directory or its declared entrypoint: %w", os.ErrInvalid)
	}

	if err := bundle.validateTarget(target, platform); err != nil {
		return nil, err
	}

	if err := bundle.verify(ctx, root); err != nil {
		return nil, err
	}

	if err := bundle.verifyEntrypoint(root); err != nil {
		return nil, err
	}

	return bundle, nil
}

func (bundle *HostBundle) verifyEntrypoint(root *os.Root) error {
	if bundle.Target != "web" {
		return ValidateNativeHost(filepath.Join(bundle.Directory, bundle.Entrypoint), bundle.Target)
	}

	file, err := root.Open(bundle.Entrypoint)
	if err != nil {
		return err
	}

	var magic [8]byte

	_, readErr := io.ReadFull(file, magic[:])
	closeErr := file.Close()

	if readErr != nil {
		return readErr
	}

	if closeErr != nil {
		return closeErr
	}

	if magic != [8]byte{0, 'a', 's', 'm', 1, 0, 0, 0} {
		return fmt.Errorf("web host bundle entrypoint is not Wasm: %w", os.ErrInvalid)
	}

	return nil
}

func decodeHostBundle(ctx context.Context, root *os.Root) (*HostBundle, error) {
	info, err := root.Lstat(HostBundleName)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > HostBundleDescriptorLimit {
		return nil, fmt.Errorf("host bundle descriptor must be a bounded regular file: %w", os.ErrInvalid)
	}

	file, err := root.Open(HostBundleName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := contextio.ReadAll(ctx, io.LimitReader(file, HostBundleDescriptorLimit+1))
	if err != nil {
		return nil, err
	}

	if len(data) > HostBundleDescriptorLimit {
		return nil, fmt.Errorf("host bundle descriptor exceeds its size bound: %w", os.ErrInvalid)
	}

	var bundle HostBundle
	if err := toml.NewDecoder(strings.NewReader(string(data))).DisallowUnknownFields().Decode(&bundle); err != nil {
		return nil, fmt.Errorf("decode host bundle: %w", err)
	}

	if bundle.Schema != 1 || bundle.Renderer != "webgpu" || len(bundle.Files) == 0 || len(bundle.Files) > HostBundleCountLimit {
		return nil, fmt.Errorf("unsupported host bundle schema, renderer or file count: %w", os.ErrInvalid)
	}

	return &bundle, nil
}

func (bundle *HostBundle) validateTarget(target, platform string) error {
	if target == "web" {
		if bundle.Target != "web" || bundle.Entrypoint != "karty-host.wasm" {
			return fmt.Errorf("host bundle does not target web: %w", os.ErrInvalid)
		}

		for _, name := range WebGPUHostFiles() {
			if _, ok := bundle.Files[name]; !ok {
				return fmt.Errorf("host bundle is missing %s: %w", name, os.ErrInvalid)
			}
		}

		return bundle.validateWebTextureCodec()
	}

	native, err := NativePlatform(platform)
	if err != nil {
		return err
	}

	entry := "karty-host-webgpu"
	if strings.HasPrefix(native, "windows-") {
		entry += ".exe"
	}

	if bundle.Target != native || (bundle.Entrypoint != entry && bundle.Entrypoint != NativeHostName(native)) {
		return fmt.Errorf("host bundle does not target %s: %w", native, os.ErrInvalid)
	}

	for _, name := range bundle.nativeLibraries() {
		if _, ok := bundle.Files[name]; !ok {
			return fmt.Errorf("host bundle is missing %s: %w", name, os.ErrInvalid)
		}
	}

	return nil
}

func (bundle *HostBundle) nativeLibraries() []string {
	switch {
	case strings.HasPrefix(bundle.Target, "linux-"):
		return []string{"libkarty_webgpu.so", "libwgpu_native.so", "libSDL3.so.0"}
	case strings.HasPrefix(bundle.Target, "darwin-"):
		return []string{"libkarty_webgpu.dylib", "libwgpu_native.dylib", "libSDL3.dylib"}
	case strings.HasPrefix(bundle.Target, "windows-"):
		return []string{"karty_webgpu.dll", "wgpu_native.dll", "SDL3.dll"}
	}

	return nil
}

func WebGPUHostFiles() []string {
	return []string{"karty-host.wasm", "runtime.bin", "wasm_exec.js", "client.mjs", "runtime.mjs", "bootstrap.mjs"}
}

func (bundle *HostBundle) verify(ctx context.Context, root *os.Root) error {
	if _, ok := bundle.Files[bundle.Entrypoint]; !ok {
		return fmt.Errorf("host bundle entrypoint is not declared: %w", os.ErrInvalid)
	}

	var total int64

	seen := make(map[string]bool, len(bundle.Files))
	for name, entry := range bundle.Files {
		if !validBundlePath(name) || !bundle.allowedFile(name) || entry.Size <= 0 || entry.Size > HostBundleFileLimit ||
			!validHostBundleDigest(entry.SHA256) {
			return fmt.Errorf("invalid host bundle file %q: %w", name, os.ErrInvalid)
		}

		folded := strings.ToLower(name)
		if seen[folded] {
			return fmt.Errorf("host bundle filenames collide across platforms: %w", os.ErrInvalid)
		}

		seen[folded] = true

		total += entry.Size
		if total > HostBundleTotalLimit {
			return fmt.Errorf("host bundle exceeds its size bound: %w", os.ErrInvalid)
		}

		if err := VerifyHostBundleFile(ctx, root, name, entry); err != nil {
			return err
		}
	}

	if bundle.Target == "web" && bundle.Files["runtime.bin"] != bundle.Files["karty-host.wasm"] {
		return fmt.Errorf("web host aliases must have identical bytes: %w", os.ErrInvalid)
	}

	return nil
}

func validBundlePath(name string) bool {
	return name != "." && path.Clean(name) == name && !path.IsAbs(name) && !strings.ContainsAny(name, "\\:\x00\r\n<>|?*\"") &&
		!strings.HasPrefix(name, "../") && !strings.Contains(name, "//")
}

func (bundle *HostBundle) allowedFile(name string) bool {
	if bundle.Target == "web" {
		return slices.Contains(WebGPUHostFiles(), name) || slices.Contains(WebGPUTextureCodecFiles(), name) ||
			slices.Contains(WebGPUTextureCodecLicenses(), name) || name == "licenses/Karty-RUNTIME_LICENSE.md"
	}

	if name == bundle.Entrypoint || strings.HasPrefix(name, "licenses/") {
		return true
	}

	if slices.Contains(bundle.nativeLibraries(), name) {
		return true
	}

	if strings.HasPrefix(bundle.Target, "windows-") {
		return slices.Contains([]string{"dxcompiler.dll", "dxil.dll", "d3d10warp.dll", "provenance/dxc-provenance.json",
			"WARP-LICENSE.TXT", "WARP-README.md", "warp-provenance.json"}, name)
	}

	return false
}

func VerifyHostBundleFile(ctx context.Context, root *os.Root, name string, entry HostBundleFile) error {
	for current := name; current != "."; current = path.Dir(current) {
		info, err := root.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || (current == name && (!info.Mode().IsRegular() || info.Size() != entry.Size)) {
			return fmt.Errorf("host bundle file %s is missing, changed or a symlink: %w", name, os.ErrInvalid)
		}
	}

	file, err := root.Open(name)
	if err != nil {
		return err
	}
	defer file.Close()

	digest := sha256.New()

	size, err := contextio.Copy(ctx, digest, io.LimitReader(file, entry.Size+1))
	if err != nil {
		return err
	}

	if size != entry.Size || hex.EncodeToString(digest.Sum(nil)) != entry.SHA256 {
		return fmt.Errorf("host bundle checksum mismatch for %s: %w", name, os.ErrInvalid)
	}

	return nil
}

func validHostBundleDigest(value string) bool {
	b, err := hex.DecodeString(value)

	return err == nil && len(b) == sha256.Size && value == strings.ToLower(value)
}
