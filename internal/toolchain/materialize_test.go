package toolchain

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/sdk"
)

// Engine-free release fixture; tests replace transport bytes, never the URL policy.
func materializeReleaseManifest() sdk.Manifest {
	manifest := sdk.Manifest{Version: "0.0.8"}
	manifest.Tools.Materialize = "2.0.0"
	manifest.Tools.MaterializeRevision = "3ad39f1308e3b2b62da81f557adc6d32697b616a"
	manifest.Artifacts.Materialize = map[string]sdk.MaterialToolArtifact{}

	for platform, digest := range map[string]string{
		"linux-amd64":   "676e0adfdec3945f9ffd47585faaabf7307cce0204d0cfc35eef6eb126ff41e6",
		"linux-arm64":   "09d93b61eca2d7f24139294a2900a86bfef911c8d7cf4535431738d292df7b92",
		"darwin-arm64":  "1afd52bff3129506e7b3708b244532ba24476af1d231189e908f343cde8d8d6a",
		"windows-amd64": "3dc10e0ef85471ca7846205619a511e891d2edfc9999a634a7adcd373d5b0a14",
	} {
		executable := "bin/materialize-cli"
		if platform == "windows-amd64" {
			executable += ".exe"
		}

		manifest.Artifacts.Materialize[platform] = sdk.MaterialToolArtifact{
			URL: "https://github.com/karty-game/karty-tools/releases/download/materialize-v2.0.0/" +
				"materialize-2.0.0-" + platform + ".zip",
			SHA256: digest, Format: "zip", Executable: executable,
		}
	}

	return manifest
}

func TestMaterializeSpecPreservesAllFourPlatforms(t *testing.T) {
	t.Parallel()

	manifest := materializeReleaseManifest()

	spec, err := MaterializeSpec(manifest)
	if err != nil {
		t.Fatal(err)
	}

	if spec.Tool != MaterialToolMaterialize || spec.Revision != manifest.Tools.MaterializeRevision || len(spec.Artifacts) != 4 {
		t.Fatalf("wrong spec: %+v", spec)
	}

	for platform, want := range manifest.Artifacts.Materialize {
		got := spec.Artifacts[platform]
		if got.URL != want.URL || got.SHA256 != want.SHA256 || got.Format != want.Format || got.Executable != want.Executable {
			t.Fatalf("metadata lost for %s: %+v", platform, got)
		}

		if _, err := materializeSpecForPlatform(manifest, platform); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := materializeSpecForPlatform(manifest, "darwin-amd64"); err == nil {
		t.Fatal("unsupported platform accepted")
	}

	// Adaptation must not alias mutable SDK maps.
	delete(spec.Artifacts, "linux-amd64")

	if len(manifest.Artifacts.Materialize) != 4 {
		t.Fatal("adapter changed manifest")
	}
}

type materializeTransport func(*http.Request) (*http.Response, error)

func (transport materializeTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func setMaterializeTransport(t *testing.T, transport materializeTransport) {
	t.Helper()

	original := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: transport}

	t.Cleanup(func() { http.DefaultClient = original })
}

func isolatedMaterializeHome(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	return home
}

//nolint:paralleltest // Changes process environment and the default HTTP client.
func TestEnsureMaterializeRejectsPinsBeforeWrites(t *testing.T) {
	home := isolatedMaterializeHome(t)
	setMaterializeTransport(t, func(_ *http.Request) (*http.Response, error) {
		t.Fatal("invalid pin reached network")

		return nil, os.ErrInvalid
	})

	for _, kind := range []string{"absent", "partial", "platform", "malformed"} {
		manifest := materializeReleaseManifest()

		switch kind {
		case "absent":
			manifest = sdk.Manifest{Version: "0.0.7"}
		case "partial":
			manifest.Tools.MaterializeRevision = ""
		case "platform":
			delete(manifest.Artifacts.Materialize, runtime.GOOS+"-"+runtime.GOARCH)
		case "malformed":
			artifact := manifest.Artifacts.Materialize["linux-amd64"]
			artifact.Executable = "../escape"
			manifest.Artifacts.Materialize["linux-amd64"] = artifact
		}

		if _, err := Ensure(t.Context(), manifest, EnsureOptions{NeedGo: true, NeedMaterialize: true}); err == nil {
			t.Fatalf("%s pin accepted", kind)
		}
	}

	if _, err := Ensure(t.Context(), materializeReleaseManifest(), EnsureOptions{}); err != nil {
		t.Fatalf("ordinary unselected Ensure demanded Materialize: %v", err)
	}

	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatalf("validation wrote to cache: %v, %v", entries, err)
	}
}

//nolint:paralleltest // Changes process environment and the default HTTP client.
func TestEnsureMaterializeOverrideIsOffline(t *testing.T) {
	home := isolatedMaterializeHome(t)
	setMaterializeTransport(t, func(_ *http.Request) (*http.Response, error) {
		t.Fatal("override reached network")

		return nil, os.ErrInvalid
	})

	path := filepath.Join(t.TempDir(), "override")
	if err := os.WriteFile(path, []byte("not executed"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatal(err)
	}

	paths, err := Ensure(t.Context(), sdk.Manifest{}, EnsureOptions{NeedMaterialize: true, MaterializeOverride: path})
	if err != nil || paths.Materialize != path {
		t.Fatalf("override: %+v, %v", paths, err)
	}

	for _, invalid := range []string{filepath.Dir(path), path + "-missing"} {
		if _, err := EnsureMaterialize(t.Context(), sdk.Manifest{}, invalid); err == nil {
			t.Fatalf("invalid override %s accepted", invalid)
		}
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0o600); err != nil {
			t.Fatal(err)
		}

		if _, err := EnsureMaterialize(t.Context(), sdk.Manifest{}, path); err == nil {
			t.Fatal("non-executable override accepted")
		}
	}

	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatalf("override wrote cache: %v, %v", entries, err)
	}
}

//nolint:paralleltest // Subtests change process environment and the default HTTP client.
func TestEnsureMaterializeFixtureInstallWarmAndBadHash(t *testing.T) {
	t.Run("warm integrity", func(t *testing.T) { checkMaterializeFixtureInstall(t, false) })
	t.Run("bad hash", func(t *testing.T) { checkMaterializeFixtureInstall(t, true) })
}

func checkMaterializeFixtureInstall(t *testing.T, badHash bool) {
	t.Helper()
	isolatedMaterializeHome(t)

	manifest := materializeReleaseManifest()
	platform := runtime.GOOS + "-" + runtime.GOARCH

	artifact, supported := manifest.Artifacts.Materialize[platform]
	if !supported {
		t.Skip("no released Materialize for this platform")
	}

	contents := materialFixture(t,
		materialFixtureMember{name: artifact.Executable, body: "fixture; never executed", mode: 0o600},
		materialFixtureMember{name: "NOTICE.txt", body: "preserved notice", mode: 0o600})
	if !badHash {
		artifact.SHA256 = fmt.Sprintf("%x", sha256.Sum256(contents))
	}

	manifest.Artifacts.Materialize[platform] = artifact
	calls := 0

	setMaterializeTransport(t, func(request *http.Request) (*http.Response, error) {
		calls++

		if request.URL.String() != artifact.URL {
			t.Fatalf("wrong release URL: %s", request.URL)
		}

		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(contents))}, nil
	})

	paths, err := Ensure(t.Context(), manifest, EnsureOptions{NeedMaterialize: true})
	if badHash {
		if err == nil || !strings.Contains(err.Error(), "checksum mismatch") || paths.Materialize != "" {
			t.Fatalf("bad archive accepted: %+v, %v", paths, err)
		}

		return
	}

	if err != nil || !filepath.IsAbs(paths.Materialize) {
		t.Fatalf("install: %+v, %v", paths, err)
	}

	warm, err := Ensure(t.Context(), manifest, EnsureOptions{NeedMaterialize: true})
	if err != nil || warm.Materialize != paths.Materialize || calls != 1 {
		t.Fatalf("warm install: %+v, %v, downloads=%d", warm, err, calls)
	}

	if err := os.WriteFile(paths.Materialize, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := EnsureMaterialize(context.Background(), manifest, ""); err == nil || calls != 1 {
		t.Fatalf("warm tampering accepted or downloaded: %v, downloads=%d", err, calls)
	}
}
