package toolchain

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/worldmaterialgpu"
	"github.com/karty-game/karty/internal/sdk"
)

func installManagedHostBundle(ctx context.Context, cache, version, target string, artifact sdk.HostBundleArtifact,
	download func(context.Context, string) (io.ReadCloser, error),
) (*HostBundle, error) {
	manifest := sdk.Manifest{}
	manifest.Host.Version = version

	return installManagedHostBundleWithSDK(ctx, cache, target, artifact, download, manifest)
}

func TestManagedHostBundleRejectsUnsafeHostVersionBeforeDownload(t *testing.T) {
	t.Parallel()

	data, pin := managedBundleFixture(t, "web", nil)
	downloads := 0

	bundle, err := installManagedHostBundle(t.Context(), t.TempDir(), "../0.0.21", "web", pin, bundleDownloader(data, &downloads))
	if err == nil || bundle != nil || downloads != 0 {
		t.Fatal("unsafe host version reached download", err, downloads)
	}
}

func addWebCodecFixture(bundle *HostBundle, files map[string][]byte) {
	for _, name := range append(WebGPUTextureCodecFiles(), WebGPUTextureCodecLicenses()...) {
		data := []byte("fixture " + name)
		if name == "transcoder.wasm" {
			data = []byte("\x00asm\x01\x00\x00\x00")
		}

		files[name] = data
		bundle.Files[name] = HostBundleFile{Size: int64(len(data)), SHA256: fmt.Sprintf("%x", sha256.Sum256(data))}
	}
}

func codecSDKFixture() sdk.Manifest {
	manifest := sdk.Manifest{Version: "0.0.21"}
	manifest.Host.Version = "0.0.21"
	manifest.Assets.Capabilities.Runtime = []asset.Capability{worldmaterialgpu.Feature}

	return manifest
}

func TestWebHostCodecFamilyMustBeCompleteAndLicensed(t *testing.T) {
	t.Parallel()

	for _, missing := range append([]string{""}, append(WebGPUTextureCodecFiles(), WebGPUTextureCodecLicenses()...)...) {
		t.Run("missing-"+missing, func(t *testing.T) {
			t.Parallel()
			data, pin := managedBundleFixture(t, "web", func(bundle *HostBundle, files map[string][]byte) {
				addWebCodecFixture(bundle, files)

				if missing != "" {
					delete(bundle.Files, missing)
					delete(files, missing)
				}
			})
			downloads := 0

			bundle, err := installManagedHostBundle(t.Context(), t.TempDir(), "0.0.21", "web", pin, bundleDownloader(data, &downloads))
			if missing != "" {
				if err == nil || bundle != nil {
					t.Fatal("partial/unlicensed codec family installed", missing)
				}

				return
			}

			if err != nil || bundle == nil || ValidateWebHostSDK(codecSDKFixture(), bundle) != nil || downloads != 1 {
				t.Fatal("complete codec family rejected", err)
			}
		})
	}
}

func TestWebBundleRequiresCodecBeforeCachePublication(t *testing.T) {
	t.Parallel()
	data, pin := managedBundleFixture(t, "web", func(bundle *HostBundle, files map[string][]byte) {
		for _, name := range WebGPUTextureCodecFiles() {
			delete(bundle.Files, name)
			delete(files, name)
		}
	})
	cache := t.TempDir()
	downloads := 0

	manifest := codecSDKFixture()
	if _, err := installManagedHostBundleWithSDK(t.Context(), cache, "web", pin, bundleDownloader(data, &downloads), manifest); err == nil {
		t.Fatal("incomplete current host admitted")
	}

	destination := filepath.Join(cache, "hosts", manifest.Host.Version, "web", "bundle-"+pin.SHA256)
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatal("incomplete host entered cache", err)
	}

	if ValidateWebHostSDK(manifest, nil) == nil {
		t.Fatal("unverified single-file host admitted")
	}
}

func TestManagedWebCodecCacheRetainsAndVerifiesWorker(t *testing.T) {
	t.Parallel()
	data, pin := managedBundleFixture(t, "web", addWebCodecFixture)
	cache := t.TempDir()
	downloads := 0
	manifest := codecSDKFixture()

	first, err := installManagedHostBundleWithSDK(t.Context(), cache, "web", pin, bundleDownloader(data, &downloads), manifest)
	if err != nil {
		t.Fatal(err)
	}

	warm, err := installManagedHostBundleWithSDK(t.Context(), cache, "web", pin, bundleDownloader(data, &downloads), manifest)
	if err != nil || first.Directory != warm.Directory || downloads != 1 {
		t.Fatal("complete managed web codec cache not reused", err)
	}

	if err := os.WriteFile(filepath.Join(first.Directory, "texture-worker.mjs"), []byte("changed worker"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := installManagedHostBundleWithSDK(
		t.Context(),
		cache,
		"web",
		pin,
		bundleDownloader(data, &downloads),
		manifest,
	); err == nil ||
		downloads != 1 {
		t.Fatal("mutated codec worker accepted or cache silently replaced", err)
	}
}
