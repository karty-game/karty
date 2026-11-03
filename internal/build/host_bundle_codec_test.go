package build

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/worldmaterialgpu"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/toolchain"
)

func webCodecBundleFixture(t *testing.T) (string, *hostBundle) {
	t.Helper()
	directory, bundle := webHostBundleFixture(t)

	for _, name := range append(toolchain.WebGPUTextureCodecFiles(), toolchain.WebGPUTextureCodecLicenses()...) {
		data := []byte("fixture " + name)
		if name == "transcoder.wasm" {
			data = []byte("\x00asm\x01\x00\x00\x00")
		}

		addBundleFixtureFile(t, directory, bundle, name, data)
	}

	writeHostBundleFixture(t, directory, bundle)

	return directory, bundle
}

func preparedMaterialSDKFixture() sdk.Manifest {
	manifest := sdk.Manifest{Version: "0.1.0"}
	manifest.Host.Version, manifest.Host.Format, manifest.Host.Renderer = manifest.Version, sdk.HostBundleFormat, sdk.HostBundleRenderer
	manifest.Assets.Capabilities.Runtime = []asset.Capability{worldmaterialgpu.Feature}

	return manifest
}

func TestWebCodecBundleStagesCompletePinnedOfflineInventory(t *testing.T) {
	t.Parallel()
	directory, bundle := webCodecBundleFixture(t)
	manifest := preparedMaterialSDKFixture()

	game := filepath.Join(t.TempDir(), "game.kart")
	if err := os.WriteFile(game, []byte("\x00asm\x01\x00\x00\x00"), 0o600); err != nil {
		t.Fatal(err)
	}

	distribution := t.TempDir()
	stage := func() {
		t.Helper()

		if err := stageRequestedTarget(t.Context(), manifest, "Codec", distribution, t.TempDir(), game, nil,
			Options{Host: directory, Target: "web"}); err != nil {
			t.Fatal(err)
		}
	}
	stage()

	web := filepath.Join(distribution, "web")
	first, release, original := readWebReleaseTest(t, web)

	for _, name := range append(toolchain.WebGPUTextureCodecFiles(), toolchain.WebGPUTextureCodecLicenses()...) {
		found := false

		for _, entry := range release.Files {
			if entry.URL == name {
				found = entry.Role == "startup" && entry.SHA256 == bundle.Files[name].SHA256
				if filepath.Ext(name) == ".mjs" && entry.Type != "text/javascript; charset=utf-8" {
					t.Fatal("codec JavaScript content type lost", entry)
				}

				if filepath.Ext(name) == ".wasm" && entry.Type != "application/wasm" {
					t.Fatal("codec WASM content type lost", entry)
				}
			}
		}

		if !found {
			t.Fatal("codec/notices absent from pinned offline startup inventory", name)
		}
	}

	staged, err := readHostBundle(t.Context(), web, "web", "")
	if err != nil || toolchain.ValidateWebHostSDK(manifest, staged) != nil {
		t.Fatal("staged descriptor did not retain complete codec delivery", err)
	}

	stage()

	repeated, _, content := readWebReleaseTest(t, web)
	if repeated != first || !bytes.Equal(content, original) {
		t.Fatal("identical codec staging changed immutable release identity")
	}

	addBundleFixtureFile(t, directory, bundle, "texture-worker.mjs", []byte("updated codec worker"))
	writeHostBundleFixture(t, directory, bundle)
	stage()

	updated, _, _ := readWebReleaseTest(t, web)
	if updated.Release == first.Release {
		t.Fatal("codec worker change did not change release/cache identity")
	}

	previous, err := os.ReadFile(filepath.Join(web, filepath.FromSlash(first.Manifest)))
	if err != nil || !bytes.Equal(previous, original) {
		t.Fatal("previous immutable release inventory was overwritten", err)
	}
}

func TestCurrentSDKRejectsIncompleteAndRawWebHostBeforeStaging(t *testing.T) {
	t.Parallel()

	directory, incomplete := webHostBundleFixture(t)
	for _, name := range toolchain.WebGPUTextureCodecFiles() {
		delete(incomplete.Files, name)
	}

	writeHostBundleFixture(t, directory, incomplete)

	manifest := preparedMaterialSDKFixture()

	for _, selected := range []string{directory, filepath.Join(directory, "karty-host.wasm")} {
		distribution := t.TempDir()
		if err := stageRequestedTarget(t.Context(), manifest, "Codec", distribution, "missing", "missing", nil,
			Options{Host: selected, Target: "web"}); err == nil {
			t.Fatal("prepared SDK accepted incomplete web bundle")
		}

		entries, err := os.ReadDir(distribution)
		if err != nil || len(entries) != 0 {
			t.Fatal("incompatible host mutated output before rejection", err)
		}
	}

	raw := filepath.Join(t.TempDir(), "raw.wasm")
	if err := os.WriteFile(raw, []byte("\x00asm\x01\x00\x00\x00"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := resolveManagedHost(t.Context(), manifest, &Options{Host: raw, Target: "web"}); err == nil {
		t.Fatal("prepared SDK selected raw host before level compilation")
	}

	_, complete := webCodecBundleFixture(t)
	if err := resolveManagedHost(t.Context(), manifest, &Options{hostBundle: complete, Target: "web"}); err != nil {
		t.Fatal("prepared SDK rejected verified complete host", err)
	}
}
