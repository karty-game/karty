package build

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

type managedHostTransport func(*http.Request) (*http.Response, error)

func (f managedHostTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestManagedHostResolvedBeforeLevelProfileAndStagesCompleteInventory(t *testing.T) {
	t.Setenv("KARTY_HOME", t.TempDir())
	t.Setenv("KARTY_HOST_WEB", "")
	directory, bundle := webHostBundleFixture(t)

	names := make([]string, 1, 1+len(bundle.Files))
	names[0] = hostBundleName

	for name := range bundle.Files {
		names = append(names, name)
	}

	sort.Strings(names)

	var output bytes.Buffer

	archive := zip.NewWriter(&output)

	for _, name := range names {
		payload, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}

		file, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := file.Write(payload); err != nil {
			t.Fatal(err)
		}
	}

	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}

	payload := output.Bytes()
	requests := 0
	previous := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: managedHostTransport(func(r *http.Request) (*http.Response, error) {
		requests++

		if r.URL.String() != "https://example.invalid/host.zip" {
			t.Fatal("unexpected managed request", r.URL)
		}

		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(payload)), Header: http.Header{}}, nil
	})}

	t.Cleanup(func() { http.DefaultClient = previous })

	manifest := sdk.Manifest{Version: "0.0.21"}
	manifest.Host.Version, manifest.Host.Format, manifest.Host.Renderer = "0.0.21", sdk.HostBundleFormat, sdk.HostBundleRenderer
	manifest.Artifacts.Host.Bundles = map[string]sdk.HostBundleArtifact{
		"web": {URL: "https://example.invalid/host.zip", SHA256: fmt.Sprintf("%x", sha256.Sum256(payload)), Format: "zip"},
	}

	options := Options{Target: "web"}
	if err := resolveManagedHost(t.Context(), manifest, &options); err != nil {
		t.Fatal(err)
	}

	if options.hostBundle == nil || options.hostBundle.Renderer != "webgpu" || options.Host == "" || requests != 1 {
		t.Fatal("managed renderer identity unavailable before levelbuild")
	}

	game := filepath.Join(t.TempDir(), "game.kart")
	if err := os.WriteFile(game, []byte("\x00asm\x01\x00\x00\x00"), 0o600); err != nil {
		t.Fatal(err)
	}

	distribution := t.TempDir()
	if err := stageRequestedTarget(t.Context(), manifest, "Managed", distribution, t.TempDir(), game, nil, options); err != nil {
		t.Fatal(err)
	}

	for _, name := range names {
		if _, err := os.Stat(filepath.Join(distribution, "web", name)); err != nil {
			t.Fatal("managed bundle file lost", name, err)
		}
	}

	if _, err := os.Stat(filepath.Join(distribution, "web", "karty.js")); !os.IsNotExist(err) {
		t.Fatal("legacy launcher selected")
	}

	descriptor, err := os.ReadFile(filepath.Join(distribution, "web", hostBundleName))
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(descriptor), options.hostBundle.Directory) {
		t.Fatal("cache filesystem path leaked into staged descriptor")
	}

	if requests != 1 {
		t.Fatal("staging redownloaded verified host")
	}
}

func TestManagedNativeRawHostRequiresBundleBeforeAssetBuild(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	for name, contents := range map[string][]byte{
		"karty.toml": []byte(fmt.Sprintf("[project]\nname='managed-native'\n[project.resolution]\nwidth=64\nheight=64\n[sdk]\nversion='%s'\n[[assets.texture]]\nname='bad'\nsource='bad.png'\n", release.SDKVersion())),
		"go.mod":     []byte("module example.com/managed-native\n\ngo 1.27.0\n"),
		"bad.png":    []byte("invalid image that must never reach asset processing"),
		"karty-host": nativeHeader(t, "linux-amd64"),
	} {
		if err := os.WriteFile(filepath.Join(directory, name), contents, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	var progress []string

	err := RunWithOptions(t.Context(), directory, Options{Target: "native", Host: filepath.Join(directory, "karty-host"),
		Progress: func(label string) { progress = append(progress, label) }})
	if !errors.Is(err, os.ErrInvalid) || !strings.Contains(err.Error(), "requires a verified host-bundle@1") {
		t.Fatal("managed SDK accepted a raw native host or reached asset processing", err, progress)
	}

	for _, label := range progress {
		if label == "Preparing tools" || label == "Processing images and audio" || label == "Building levels" {
			t.Fatal("raw native host rejected after build work", progress)
		}
	}

	if _, err := os.Stat(filepath.Join(directory, ".karty", "engine")); !os.IsNotExist(err) {
		t.Fatal("raw host started SDK code generation", err)
	}
}

func TestUnsupportedSDKHostProfilesNeverResolveArtifacts(t *testing.T) {
	t.Parallel()

	for _, scenario := range []struct{ format, renderer string }{{}, {"raw", "webgpu"}, {sdk.HostBundleFormat, "ebiten"}} {
		manifest := sdk.Manifest{Version: release.SDKVersion()}
		manifest.Host.Format, manifest.Host.Renderer = scenario.format, scenario.renderer

		options := Options{Target: "native", Host: "unverified-file"}
		if err := resolveManagedHost(t.Context(), manifest, &options); !errors.Is(err, os.ErrInvalid) {
			t.Fatal("unsupported profile reached artifact resolution", scenario, err)
		}
	}
}
