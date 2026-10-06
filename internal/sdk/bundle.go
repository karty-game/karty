package sdk

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?$`)

const maxBundleBytes = 32 << 20
const bundleTimeout = 2 * time.Minute

// Resolve uses only the exact installed, checksummed SDK.
// Install published versions explicitly with `karty sdk install VERSION`.

func Resolve(version string) (Manifest, error) {
	if err := validateVersion(version); err != nil {
		return Manifest{}, err
	}

	root, err := sdkCache()
	if err != nil {
		return Manifest{}, err
	}

	data, err := readBundleFile(filepath.Join(root, version, "sdk.zip"))
	if os.IsNotExist(err) {
		return Manifest{}, fmt.Errorf("SDK %s is not installed; run karty sdk install %s: %w", version, version, err)
	}

	if err != nil {
		return Manifest{}, err
	}

	hash, err := os.ReadFile(filepath.Join(root, version, "sha256"))
	if err != nil {
		return Manifest{}, err
	}

	if err := verifyBundleHash(data, string(hash)); err != nil {
		return Manifest{}, err
	}

	return readBundle(version, data)
}

func readBundle(version string, data []byte) (Manifest, error) {
	if len(data) > maxBundleBytes {
		return Manifest{}, bundleError("SDK bundle exceeds size limit")
	}

	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Manifest{}, err
	}

	seen := map[string]bool{}

	var total uint64

	for _, file := range archive.File {
		if !fs.ValidPath(file.Name) || strings.Contains(file.Name, "\\") || !file.Mode().IsRegular() || seen[file.Name] {
			return Manifest{}, bundleError("invalid SDK entry %q", file.Name)
		}

		seen[file.Name] = true
		if file.UncompressedSize64 > maxBundleBytes {
			return Manifest{}, bundleError("SDK entry exceeds size limit")
		}

		total += file.UncompressedSize64
		if total > maxBundleBytes || len(seen) > 2048 {
			return Manifest{}, bundleError("SDK bundle exceeds limits")
		}
	}

	metadata, err := fs.ReadFile(archive, "bundle.json")
	if err != nil {
		return Manifest{}, err
	}

	//nolint:tagliatelle // Archive format uses explicit snake_case keys.
	var compatibility struct {
		Format         int `json:"format"`
		UISchema       int `json:"ui_schema"`
		ProjectCodegen int `json:"project_codegen"`
	}
	if err = json.Unmarshal(metadata, &compatibility); err != nil {
		return Manifest{}, err
	}

	if compatibility.Format != 1 || (compatibility.UISchema != 9 && compatibility.UISchema != 10 && compatibility.UISchema != 11) ||
		(compatibility.ProjectCodegen != 1 && compatibility.ProjectCodegen != 2) {
		return Manifest{}, bundleError("SDK requires unsupported bundle, UI schema or project generator; upgrade Karty")
	}

	if compatibility.ProjectCodegen == authoredActionProjectCodegen {
		if _, err := fs.ReadFile(archive, "contracts/actions-v1.schema.json"); err != nil {
			return Manifest{}, bundleError("SDK project generator requires the authored action schema")
		}
	}

	raw, err := fs.ReadFile(archive, "manifest.toml")
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
	for _, compiler := range []string{"go", "tinygo"} {
		if _, err = ClientFiles(manifest, compiler); err != nil {
			return Manifest{}, err
		}
	}

	for _, prefix := range []string{"docs/" + manifest.Docs.Version, "templates/game/" + manifest.Templates.Game} {
		if _, err = fs.Stat(archive, prefix); err != nil {
			return Manifest{}, err
		}
	}

	return manifest, nil
}

// Resources exposes only the selected SDK archive's public build inputs.
func Resources(manifest Manifest) (fs.FS, error) {
	if manifest.resources != nil {
		return manifest.resources, nil
	}

	resolved, err := Resolve(manifest.Version)
	if err != nil {
		return nil, err
	}

	return resolved.resources, nil
}

// ClientFiles copies pre-generated SDK bindings; no WIT generator runs in the CLI.
func ClientFiles(manifest Manifest, compiler string) (map[string][]byte, error) {
	if compiler != "go" && compiler != "tinygo" {
		return nil, bundleError("unsupported compiler %q", compiler)
	}

	tree, err := Resources(manifest)
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
	hash := sha256.Sum256(data)
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

	data, err := io.ReadAll(io.LimitReader(response.Body, maxBundleBytes+1))
	if err != nil {
		return err
	}

	bundled, err := readBundle(version, data)
	if err != nil {
		return err
	}

	bundled.resources = nil
	expected := manifest

	expected.Bundle = ToolArtifact{}
	if !reflect.DeepEqual(bundled, expected) {
		return bundleError("signed manifest and SDK bundle metadata differ")
	}

	return InstallBundle(version, data, manifest.Bundle.SHA256)
}

// InstallBundle installs a bounded, checksummed SDK atomically. This also supports
// candidate artifacts produced locally by engine contributors.
func InstallBundle(version string, data []byte, checksum string) error {
	if err := validateVersion(version); err != nil {
		return err
	}

	if err := verifyBundleHash(data, checksum); err != nil {
		return err
	}

	if _, err := readBundle(version, data); err != nil {
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
	if existing, err := os.ReadFile(filepath.Join(target, "sha256")); err == nil {
		if string(existing) != checksum {
			return bundleError(
				"SDK %s is immutable; installed checksum differs; remove %q only if it is a pre-release local candidate, then reinstall",
				version, target,
			)
		}

		contents, err := readBundleFile(filepath.Join(target, "sdk.zip"))
		if err != nil {
			return err
		}

		return verifyBundleHash(contents, checksum)
	}

	stage, err := os.MkdirTemp(root, ".sdk-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)

	if err = os.WriteFile(filepath.Join(stage, "sdk.zip"), data, 0600); err != nil {
		return err
	}

	if err = os.WriteFile(filepath.Join(stage, "sha256"), []byte(checksum), 0600); err != nil {
		return err
	}

	return os.Rename(stage, target)
}

const errBundle staticError = "invalid SDK bundle"

func bundleError(message string, args ...any) error {
	return fmt.Errorf("%w: %s", errBundle, fmt.Sprintf(message, args...))
}

func readBundleFile(path string) ([]byte, error) {
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

	data, err := io.ReadAll(io.LimitReader(file, maxBundleBytes+1))
	if err != nil {
		return nil, err
	}

	if len(data) > maxBundleBytes {
		return nil, bundleError("cached archive exceeds size limit")
	}

	return data, nil
}

const authoredActionProjectCodegen = 2
