package sdk

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/karty-game/karty/internal/contextio"
	"github.com/pelletier/go-toml/v2"
)

var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?$`)

const maxBundleBytes = 32 << 20
const bundleTimeout = 2 * time.Minute

// ErrNotInstalled distinguishes an absent SDK bundle from a damaged installation.
var ErrNotInstalled = errors.New("SDK is not installed")

// Resolve uses only the exact installed, checksummed SDK.
// Install published versions explicitly with `karty sdk install VERSION`.

func Resolve(version string) (Manifest, error) {
	return ResolveContext(context.Background(), version)
}

func ResolveContext(ctx context.Context, version string) (Manifest, error) {
	if err := ctx.Err(); err != nil {
		return Manifest{}, err
	}

	if err := validateVersion(version); err != nil {
		return Manifest{}, err
	}

	root, err := sdkCache()
	if err != nil {
		return Manifest{}, err
	}

	data, err := readBundleFileContext(ctx, filepath.Join(root, version, "sdk.zip"))
	if os.IsNotExist(err) {
		return Manifest{}, fmt.Errorf("SDK %s is not installed; run karty sdk install %s: %w: %w", version, version, ErrNotInstalled, err)
	}

	if err != nil {
		return Manifest{}, err
	}

	hash, err := contextio.ReadFile(ctx, filepath.Join(root, version, "sha256"))
	if err != nil {
		return Manifest{}, err
	}

	if err := verifyBundleHashContext(ctx, data, string(hash)); err != nil {
		return Manifest{}, err
	}

	return readBundleContext(ctx, version, data)
}

func readBundle(version string, data []byte) (Manifest, error) {
	return readBundleContext(context.Background(), version, data)
}

func readBundleContext(ctx context.Context, version string, data []byte) (Manifest, error) {
	if err := ctx.Err(); err != nil {
		return Manifest{}, err
	}

	if len(data) > maxBundleBytes {
		return Manifest{}, bundleError("SDK bundle exceeds size limit")
	}

	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Manifest{}, err
	}

	if err := validateBundleEntries(ctx, archive.File); err != nil {
		return Manifest{}, err
	}

	tree := contextio.Filesystem(ctx, archive)

	compatibility, err := readCompatibility(tree, version)
	if err != nil {
		return Manifest{}, err
	}

	if (compatibility.UISchema != 9 && compatibility.UISchema != 10 && compatibility.UISchema != 11) ||
		(compatibility.ProjectCodegen != 1 && compatibility.ProjectCodegen != 2) {
		return Manifest{}, bundleError("SDK requires unsupported bundle, UI schema or project generator; upgrade Karty")
	}

	if compatibility.ProjectCodegen == authoredActionProjectCodegen {
		if _, err := fs.ReadFile(tree, "contracts/actions-v1.schema.json"); err != nil {
			if canceled := ctx.Err(); canceled != nil {
				return Manifest{}, canceled
			}

			return Manifest{}, bundleError("SDK project generator requires the authored action schema")
		}
	}

	raw, err := fs.ReadFile(tree, "manifest.toml")
	if err != nil {
		return Manifest{}, err
	}

	var manifest Manifest
	if err = toml.Unmarshal(raw, &manifest); err != nil {
		return Manifest{}, err
	}

	if err = validateManifest(version, manifest); err != nil {
		return Manifest{}, err
	}

	manifest.resources = archive

	manifest.Compatibility = compatibility
	for _, compiler := range []string{"go", "tinygo"} {
		if _, err = ClientFilesContext(ctx, manifest, compiler); err != nil {
			return Manifest{}, err
		}
	}

	for _, prefix := range []string{"docs/" + manifest.Docs.Version, "templates/game/" + manifest.Templates.Game} {
		if _, err = fs.Stat(tree, prefix); err != nil {
			return Manifest{}, err
		}
	}

	return manifest, nil
}

func validateBundleEntries(ctx context.Context, files []*zip.File) error {
	seen := map[string]bool{}

	var total uint64

	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return err
		}

		if !fs.ValidPath(file.Name) || strings.Contains(file.Name, "\\") || !file.Mode().IsRegular() || seen[file.Name] {
			return bundleError("invalid SDK entry %q", file.Name)
		}

		seen[file.Name] = true
		if file.UncompressedSize64 > maxBundleBytes {
			return bundleError("SDK entry exceeds size limit")
		}

		total += file.UncompressedSize64
		if total > maxBundleBytes || len(seen) > 2048 {
			return bundleError("SDK bundle exceeds limits")
		}
	}

	return nil
}

// Resources exposes only the selected SDK archive's public build inputs.
func Resources(manifest Manifest) (fs.FS, error) {
	return ResourcesContext(context.Background(), manifest)
}

func ResourcesContext(ctx context.Context, manifest Manifest) (fs.FS, error) {
	if manifest.resources != nil {
		return contextio.Filesystem(ctx, manifest.resources), ctx.Err()
	}

	resolved, err := ResolveContext(ctx, manifest.Version)
	if err != nil {
		return nil, err
	}

	return contextio.Filesystem(ctx, resolved.resources), ctx.Err()
}

// ClientFiles copies pre-generated SDK bindings; no WIT generator runs in the CLI.
func ClientFiles(manifest Manifest, compiler string) (map[string][]byte, error) {
	return ClientFilesContext(context.Background(), manifest, compiler)
}

func ClientFilesContext(ctx context.Context, manifest Manifest, compiler string) (map[string][]byte, error) {
	if compiler != "go" && compiler != "tinygo" {
		return nil, bundleError("unsupported compiler %q", compiler)
	}

	tree, err := ResourcesContext(ctx, manifest)
	if err != nil {
		return nil, err
	}

	result := map[string][]byte{}

	for _, name := range []string{"game.go", "components.go", "protocol.go", "renderers.go"} {
		data, err := fs.ReadFile(tree, "bindings/"+compiler+"/engine/"+name)
		if err != nil {
			return nil, err
		}

		result["engine/"+name] = data
	}

	return result, nil
}
func sdkCache() (string, error) {
	root := os.Getenv("KARTY_HOME")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		root = filepath.Join(home, ".karty")
	}

	return filepath.Join(root, "sdks"), nil
}
func verifyBundleHash(data []byte, want string) error {
	return verifyBundleHashContext(context.Background(), data, want)
}

func verifyBundleHashContext(ctx context.Context, data []byte, want string) error {
	hash, err := contextio.Sum256(ctx, data)
	if err != nil {
		return err
	}

	if len(want) != 64 || hex.EncodeToString(hash[:]) != want {
		return bundleError("SDK bundle checksum mismatch")
	}

	return nil
}

// InstallPublished verifies the signed release manifest before trusting its bundle URL.
func InstallPublished(ctx context.Context, version string) error {
	if err := validateVersion(version); err != nil {
		return err
	}

	manifest, err := ResolvePublished(ctx, version)
	if err != nil {
		return err
	}

	if manifest.Bundle.URL == "" || len(manifest.Bundle.SHA256) != 64 {
		return bundleError("release has no SDK bundle")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, manifest.Bundle.URL, nil)
	if err != nil {
		return err
	}

	response, err := (&http.Client{Timeout: bundleTimeout}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return bundleError("download SDK bundle: %s", response.Status)
	}

	data, err := contextio.ReadAll(ctx, io.LimitReader(response.Body, maxBundleBytes+1))
	if err != nil {
		return err
	}

	bundled, err := readBundleContext(ctx, version, data)
	if err != nil {
		return err
	}

	bundled.resources = nil
	bundled.Compatibility = Compatibility{}
	expected := manifest

	expected.Bundle = ToolArtifact{}
	if !reflect.DeepEqual(bundled, expected) {
		return bundleError("signed manifest and SDK bundle metadata differ")
	}

	return InstallBundleContext(ctx, version, data, manifest.Bundle.SHA256)
}

// InstallBundle installs a bounded, checksummed SDK atomically. This also supports
// candidate artifacts produced locally by engine contributors.
func InstallBundle(version string, data []byte, checksum string) error {
	return InstallBundleContext(context.Background(), version, data, checksum)
}

func InstallBundleContext(ctx context.Context, version string, data []byte, checksum string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := validateVersion(version); err != nil {
		return err
	}

	if err := verifyBundleHashContext(ctx, data, checksum); err != nil {
		return err
	}

	if _, err := readBundleContext(ctx, version, data); err != nil {
		return err
	}

	root, err := sdkCache()
	if err != nil {
		return err
	}

	if err = os.MkdirAll(root, 0750); err != nil {
		return err
	}

	target := filepath.Join(root, version)
	if existing, err := contextio.ReadFile(ctx, filepath.Join(target, "sha256")); err == nil {
		if string(existing) != checksum {
			return bundleError(
				"SDK %s is immutable; installed checksum differs; remove %q only if it is a pre-release local candidate, then reinstall",
				version, target,
			)
		}

		contents, err := readBundleFileContext(ctx, filepath.Join(target, "sdk.zip"))
		if err != nil {
			return err
		}

		return verifyBundleHashContext(ctx, contents, checksum)
	}

	stage, err := os.MkdirTemp(root, ".sdk-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)

	if err = contextio.WriteFile(ctx, filepath.Join(stage, "sdk.zip"), data, 0600); err != nil {
		return err
	}

	if err = contextio.WriteFile(ctx, filepath.Join(stage, "sha256"), []byte(checksum), 0600); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	return os.Rename(stage, target)
}

const errBundle staticError = "invalid SDK bundle"

func bundleError(message string, args ...any) error {
	return fmt.Errorf("%w: %s", errBundle, fmt.Sprintf(message, args...))
}

func readBundleFile(path string) ([]byte, error) {
	return readBundleFileContext(context.Background(), path)
}

func readBundleFileContext(ctx context.Context, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if !info.Mode().IsRegular() || info.Size() > maxBundleBytes {
		return nil, bundleError("invalid cached archive")
	}

	data, err := contextio.ReadAll(ctx, io.LimitReader(file, maxBundleBytes+1))
	if err != nil {
		return nil, err
	}

	if len(data) > maxBundleBytes {
		return nil, bundleError("cached archive exceeds size limit")
	}

	return data, nil
}

const authoredActionProjectCodegen = 2
const tomlBundleFormat = 2
const semanticVersionParts = 3

func readCompatibility(archive fs.FS, version string) (Compatibility, error) {
	if bundleUsesTOML(version) {
		return readTOMLCompatibility(archive)
	}

	// Released SDK archives are immutable. Their build-only format1 metadata is
	// read only for explicitly selected versions preceding the binary candidate.
	data, err := compatibilityBytes(archive, "bundle.json")
	if err != nil {
		return Compatibility{}, err
	}

	var compatibility Compatibility
	if err := json.Unmarshal(data, &compatibility); err != nil {
		return Compatibility{}, err
	}

	if compatibility.Format != 1 {
		return Compatibility{}, bundleError("SDK bundle format is unsupported")
	}

	return compatibility, nil
}

func readTOMLCompatibility(archive fs.FS) (Compatibility, error) {
	if _, err := fs.Stat(archive, "bundle.json"); err == nil {
		return Compatibility{}, bundleError("current SDK cannot contain legacy JSON bundle metadata")
	} else if !os.IsNotExist(err) {
		return Compatibility{}, err
	}

	data, err := compatibilityBytes(archive, "bundle.toml")
	if err != nil {
		return Compatibility{}, err
	}

	var compatibility Compatibility

	decoder := toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields()
	if err := decoder.Decode(&compatibility); err != nil {
		return Compatibility{}, err
	}

	if compatibility.Format != tomlBundleFormat {
		return Compatibility{}, bundleError("SDK bundle format is unsupported")
	}

	return compatibility, nil
}

func compatibilityBytes(archive fs.FS, name string) ([]byte, error) {
	data, err := fs.ReadFile(archive, name)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 || len(data) > 1024 {
		return nil, bundleError("SDK bundle metadata exceeds bounds")
	}

	return data, nil
}

func bundleUsesTOML(version string) bool {
	release, _, _ := strings.Cut(version, "-")

	parts := strings.Split(release, ".")
	if len(parts) != semanticVersionParts {
		return true
	}

	major, majorErr := strconv.ParseUint(parts[0], 10, 64)
	minor, minorErr := strconv.ParseUint(parts[1], 10, 64)
	patch, patchErr := strconv.ParseUint(parts[2], 10, 64)

	return majorErr != nil || minorErr != nil || patchErr != nil || major > 0 || minor > 0 || patch >= 11
}
