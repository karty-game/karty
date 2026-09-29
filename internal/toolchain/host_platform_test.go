package toolchain

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/karty-game/karty/internal/sdk"
)

func TestInstallDistributionHostUsesDestinationPlatform(t *testing.T) {
	t.Parallel()

	data := []byte("fixture executable")

	var requests atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)

		_, _ = w.Write(data)
	}))
	defer server.Close()

	asset := sdk.ToolArtifact{URL: server.URL, SHA256: fmt.Sprintf("%x", sha256.Sum256(data))}

	cache := t.TempDir()
	for _, target := range []string{"windows-arm64", "linux-arm64"} {
		path, err := installHost(context.Background(), cache, "0.0.4", target, asset)
		if err != nil {
			t.Fatal(err)
		}

		want := filepath.Join(cache, "hosts", "0.0.4", target, "karty-host")
		if path != want {
			t.Fatalf("got %s, want %s", path, want)
		}

		contents, err := os.ReadFile(path)
		if err != nil || string(contents) != string(data) {
			t.Fatalf("wrong staged host: %s, %v", contents, err)
		}

		if _, err := installHost(context.Background(), cache, "0.0.4", target, asset); err != nil {
			t.Fatal(err)
		}
	}

	if requests.Load() != 2 {
		t.Fatalf("expected independent cached downloads, got %d", requests.Load())
	}

	asset.SHA256 = fmt.Sprintf("%064x", 0)
	if _, err := installHost(context.Background(), t.TempDir(), "0.0.4", "windows-arm64", asset); err == nil {
		t.Fatal("incorrect checksum accepted")
	}
}

func TestHostPlatformRejectsUnsupportedOrUnsafeTargets(t *testing.T) {
	t.Parallel()

	for _, target := range []string{"../../escape", "darwin-amd64", "linux-386", "/tmp/host"} {
		if _, err := hostPlatform(target); err == nil {
			t.Fatalf("accepted %s", target)
		}
	}
}

func TestPublishedHostMetadataSelectsDistributionPlatform(t *testing.T) {
	t.Parallel()

	var manifest sdk.Manifest

	manifest.Artifacts.Host.Native = map[string]sdk.ToolArtifact{
		"windows-arm64": {URL: "https://example.com/windows-arm64", SHA256: "checksum"},
	}
	// A supplied destination entry must not trigger metadata lookup for the developer OS.
	got, err := manifestWithPublishedHost(context.Background(), manifest, "windows-arm64")
	if err != nil || got.Artifacts.Host.Native["windows-arm64"].URL == "" {
		t.Fatalf("%+v, %v", got, err)
	}
}
