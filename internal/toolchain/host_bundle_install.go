package toolchain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/karty-game/karty/internal/contextio"
	"github.com/karty-game/karty/internal/sdk"
)

const hostBundleArchiveLimit = 256 << 20
const hostBundleDownloadTimeout = 2 * time.Minute

// EnsureManagedHostBundle installs and verifies a declared complete bundle.
// It never falls back to a raw artifact when bundle delivery is selected.
func EnsureManagedHostBundle(ctx context.Context, manifest sdk.Manifest, target string) (*HostBundle, error) {
	if err := sdk.ValidateHostBundles(manifest); err != nil {
		return nil, err
	}

	if manifest.Host.Format != sdk.HostBundleFormat {
		return nil, errHostMetadata
	}

	selected, err := bundleTarget(target)
	if err != nil {
		return nil, err
	}

	requested := manifest

	if _, ok := manifest.Artifacts.Host.Bundles[selected]; !ok {
		published, err := sdk.ResolvePublished(ctx, manifest.Version)
		if err != nil {
			return nil, fmt.Errorf("resolve signed %s host bundle for SDK %s: %w", selected, manifest.Version, err)
		}

		if published.Host.Version != manifest.Host.Version || published.Host.Format != manifest.Host.Format ||
			published.Host.Renderer != manifest.Host.Renderer {
			return nil, errHostMetadata
		}

		manifest = published
	}

	artifact, ok := manifest.Artifacts.Host.Bundles[selected]
	if !ok {
		return nil, fmt.Errorf("SDK %s lacks %s host bundle: %w", manifest.Version, selected, errHostMetadata)
	}

	cache, err := kartyHome("")
	if err != nil {
		return nil, err
	}

	return installManagedHostBundleWithSDK(ctx, cache, selected, artifact, downloadURL, requested)
}

func bundleTarget(target string) (string, error) {
	if target == "web" {
		return "web", nil
	}

	return hostPlatform(target)
}

func installManagedHostBundleWithSDK(ctx context.Context, cache, target string, artifact sdk.HostBundleArtifact,
	download func(context.Context, string) (io.ReadCloser, error), manifest sdk.Manifest,
) (*HostBundle, error) {
	version := manifest.Host.Version
	probe := sdk.Manifest{}
	probe.Host.Version, probe.Host.Format, probe.Host.Renderer = version, sdk.HostBundleFormat, sdk.HostBundleRenderer

	probe.Artifacts.Host.Bundles = map[string]sdk.HostBundleArtifact{target: artifact}
	if err := sdk.ValidateHostBundles(probe); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	parent := filepath.Join(cache, "hosts", version, target)

	destination := filepath.Join(parent, "bundle-"+artifact.SHA256)
	if _, err := os.Lstat(destination); err == nil {
		return verifyCachedManagedHostSDK(ctx, destination, target, artifact.SHA256, manifest)
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	if err := os.MkdirAll(parent, 0o750); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, hostBundleDownloadTimeout)
	defer cancel()

	source, err := download(ctx, artifact.URL)
	if err != nil {
		return nil, err
	}

	contents, readErr := contextio.ReadAll(ctx, io.LimitReader(source, hostBundleArchiveLimit+1))
	closeErr := source.Close()

	if readErr != nil {
		return nil, readErr
	}

	if closeErr != nil {
		return nil, closeErr
	}

	if len(contents) == 0 || len(contents) > hostBundleArchiveLimit {
		return nil, fmt.Errorf("host bundle compressed size: %w", os.ErrInvalid)
	}

	digest := sha256.Sum256(contents)
	if hex.EncodeToString(digest[:]) != artifact.SHA256 {
		return nil, errHostChecksum
	}

	staging, err := os.MkdirTemp(parent, ".host-bundle-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(staging)

	payload := filepath.Join(staging, "payload")
	if err := os.Mkdir(payload, 0o750); err != nil {
		return nil, err
	}

	if err := contextio.WriteFile(ctx, filepath.Join(staging, "archive.zip"), contents, 0o600); err != nil {
		return nil, err
	}

	if err := extractManagedHostBundle(ctx, payload, contents); err != nil {
		return nil, err
	}

	bundle, err := verifyManagedHostBundle(ctx, payload, target)
	if err != nil {
		return nil, err
	}

	if err := validateManagedHostSDK(manifest, target, bundle); err != nil {
		return nil, err
	}

	if target != "web" {
		if err := os.Chmod(filepath.Join(payload, bundle.Entrypoint), 0o700); err != nil {
			return nil, err
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := os.Rename(staging, destination); err != nil {
		// Another installer may have published the same immutable archive.
		if existing, checkErr := verifyCachedManagedHostSDK(ctx, destination, target, artifact.SHA256, manifest); checkErr == nil {
			return existing, nil
		}

		return nil, err
	}

	bundle.Directory = filepath.Join(destination, "payload")

	return bundle, nil
}

func validateManagedHostSDK(manifest sdk.Manifest, target string, bundle *HostBundle) error {
	if target == "web" {
		return ValidateWebHostSDK(manifest, bundle)
	}

	return nil
}

func verifyCachedManagedHostSDK(ctx context.Context, destination, target, digest string, manifest sdk.Manifest) (*HostBundle, error) {
	bundle, err := verifyCachedManagedHostBundle(ctx, destination, target, digest)
	if err != nil {
		return nil, err
	}

	if err := validateManagedHostSDK(manifest, target, bundle); err != nil {
		return nil, err
	}

	return bundle, nil
}

func verifyCachedManagedHostBundle(ctx context.Context, container, target, digest string) (*HostBundle, error) {
	info, err := os.Lstat(container)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errHostNotFile
	}

	archivePath := filepath.Join(container, "archive.zip")

	info, err = os.Lstat(archivePath)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > hostBundleArchiveLimit {
		return nil, errHostNotFile
	}

	file, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}

	contents, readErr := contextio.ReadAll(ctx, io.LimitReader(file, hostBundleArchiveLimit+1))
	closeErr := file.Close()

	if readErr != nil {
		return nil, readErr
	}

	if closeErr != nil {
		return nil, closeErr
	}

	sum := sha256.Sum256(contents)
	if hex.EncodeToString(sum[:]) != digest {
		return nil, errHostChecksum
	}

	directory := filepath.Join(container, "payload")
	if err := matchManagedDescriptor(ctx, directory, contents); err != nil {
		return nil, err
	}

	bundle, err := verifyManagedHostBundle(ctx, directory, target)
	if err != nil {
		return nil, err
	}

	if target != "web" && runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(directory, bundle.Entrypoint))
		if err != nil || info.Mode()&0o111 == 0 {
			return nil, errNotExecutable
		}
	}

	if !strings.HasPrefix(bundle.Directory, container+string(os.PathSeparator)) {
		return nil, os.ErrInvalid
	}

	return bundle, nil
}
