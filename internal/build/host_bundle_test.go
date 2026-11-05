package build

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/toolchain"
	"github.com/pelletier/go-toml/v2"
)

func TestWebGPUHostBundleStagesEngineBootstrapAndOfflineModules(t *testing.T) {
	t.Parallel()
	directory, bundle := webCodecBundleFixture(t)
	// Published schema-1 inputs may retain the old alias; new output omits it.
	addBundleFixtureFile(t, directory, bundle, "runtime.bin", []byte("\x00asm\x01\x00\x00\x00"))
	writeHostBundleFixture(t, directory, bundle)

	game := filepath.Join(t.TempDir(), "game.kart")
	if err := os.WriteFile(game, []byte("\x00asm\x01\x00\x00\x00"), 0o600); err != nil {
		t.Fatal(err)
	}

	distribution := t.TempDir()
	if err := os.MkdirAll(filepath.Join(distribution, "web"), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(distribution, "web", "runtime.bin"), []byte("previous output"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := stageRequestedTarget(t.Context(), preparedMaterialSDKFixture(), "A <game>", distribution, t.TempDir(), game, nil,
		Options{Host: directory, Target: "web"}); err != nil {
		t.Fatal(err)
	}

	web := filepath.Join(distribution, "web")
	assertWebGPUHostShell(t, web, bundle)

	root, err := os.OpenRoot(web)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	manifest := readWebReleaseManifest(t, web)
	assertWebBundleInManifest(t, manifest, bundle)
}

func assertWebGPUHostShell(t *testing.T, web string, bundle *hostBundle) {
	t.Helper()

	if _, err := os.Stat(filepath.Join(web, "runtime.bin")); !os.IsNotExist(err) {
		t.Fatalf("legacy runtime alias staged: %v", err)
	}

	contents, err := os.ReadFile(filepath.Join(web, "index.html"))
	if err != nil {
		t.Fatal(err)
	}

	text := string(contents)
	for _, expected := range []string{"A &lt;game&gt;", "data-wasm-sha256=\"" + bundle.Files[bundle.Entrypoint].SHA256, "bootstrap.mjs", "await KartyWebRuntime.ready"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("shell missing %q", expected)
		}
	}

	if _, err := os.Stat(filepath.Join(web, "karty.js")); !os.IsNotExist(err) {
		t.Fatalf("legacy launcher staged: %v", err)
	}
}

func readWebReleaseManifest(t *testing.T, web string) webReleaseManifest {
	t.Helper()

	pointerData, err := os.ReadFile(filepath.Join(web, "karty-latest.json"))
	if err != nil {
		t.Fatal(err)
	}

	var pointer webReleasePointer
	if err := json.Unmarshal(pointerData, &pointer); err != nil {
		t.Fatal(err)
	}

	manifestData, err := os.ReadFile(filepath.Join(web, filepath.FromSlash(pointer.Manifest)))
	if err != nil {
		t.Fatal(err)
	}

	var manifest webReleaseManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}

	return manifest
}

func assertWebBundleInManifest(t *testing.T, manifest webReleaseManifest, bundle *hostBundle) {
	t.Helper()

	for name := range bundle.Files {
		if name == "runtime.bin" {
			for _, entry := range manifest.Files {
				if entry.URL == name {
					t.Fatal("legacy runtime alias retained in release inventory")
				}
			}

			continue
		}

		found := false

		for _, entry := range manifest.Files {
			if entry.URL == name {
				found = entry.SHA256 == bundle.Files[name].SHA256 && entry.Role == "startup"
			}
		}

		if !found {
			t.Fatalf("bundle file %s missing from pinned offline startup", name)
		}
	}
}

func TestHostBundleRejectsInvalidInputsBeforeStaging(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*testing.T, string, *hostBundle){
		"checksum": func(t *testing.T, directory string, _ *hostBundle) {
			t.Helper()

			if err := os.WriteFile(filepath.Join(directory, "client.mjs"), []byte("changed"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"traversal": func(_ *testing.T, _ string, bundle *hostBundle) {
			bundle.Files["../escape"] = bundle.Files["client.mjs"]
		},
		"unknown-file": func(_ *testing.T, _ string, bundle *hostBundle) {
			bundle.Files["game.kart"] = bundle.Files["client.mjs"]
		},
		"missing-entry": func(_ *testing.T, _ string, bundle *hostBundle) { delete(bundle.Files, bundle.Entrypoint) },
		"alias": func(_ *testing.T, _ string, bundle *hostBundle) {
			bundle.Files["runtime.bin"] = bundle.Files["client.mjs"]
		},
		"size": func(_ *testing.T, _ string, bundle *hostBundle) {
			entry := bundle.Files["client.mjs"]
			entry.Size = hostBundleFileLimit + 1
			bundle.Files["client.mjs"] = entry
		},
		"renderer": func(_ *testing.T, _ string, bundle *hostBundle) { bundle.Renderer = "unknown" },
		"schema":   func(_ *testing.T, _ string, bundle *hostBundle) { bundle.Schema = 2 },
		"target":   func(_ *testing.T, _ string, bundle *hostBundle) { bundle.Target = "windows-amd64" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			directory, bundle := webHostBundleFixture(t)
			mutate(t, directory, bundle)
			writeHostBundleFixture(t, directory, bundle)
			distribution := t.TempDir()

			marker := filepath.Join(distribution, "previous")
			if err := os.WriteFile(marker, []byte("retained"), 0o600); err != nil {
				t.Fatal(err)
			}

			if err := stageRequestedTarget(t.Context(), preparedMaterialSDKFixture(), "test", distribution, "missing", "missing", nil,
				Options{Host: directory, Target: "web"}); err == nil {
				t.Fatal("invalid bundle accepted")
			}

			entries, err := os.ReadDir(distribution)
			if err != nil || len(entries) != 1 || entries[0].Name() != "previous" {
				t.Fatalf("invalid input changed distribution: %v, %v", entries, err)
			}
		})
	}
}

func TestHostBundleDescriptorAndFileSymlinksRejected(t *testing.T) {
	t.Parallel()

	for _, name := range []string{hostBundleName, "client.mjs"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			directory, _ := webHostBundleFixture(t)
			selected := filepath.Join(directory, name)

			outside := filepath.Join(t.TempDir(), name)
			if err := os.Rename(selected, outside); err != nil {
				t.Fatal(err)
			}

			if err := os.Symlink(outside, selected); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}

			if _, err := ValidateLocalHost(t.Context(), directory, "web", ""); !errors.Is(err, os.ErrInvalid) {
				t.Fatalf("symlink accepted: %v", err)
			}
		})
	}
}

func TestHostBundleRejectsUnknownAndDuplicateDescriptorFields(t *testing.T) {
	t.Parallel()

	for _, extra := range []string{"\nunknown = true\n", "\nsize = 123\n"} {
		t.Run(extra, func(t *testing.T) {
			t.Parallel()
			directory, _ := webHostBundleFixture(t)
			descriptor := filepath.Join(directory, hostBundleName)

			file, err := os.OpenFile(descriptor, os.O_APPEND|os.O_WRONLY, 0o600)
			if err != nil {
				t.Fatal(err)
			}

			_, writeErr := file.WriteString(extra)

			closeErr := file.Close()
			if writeErr != nil || closeErr != nil {
				t.Fatalf("write descriptor: %v, %v", writeErr, closeErr)
			}

			if _, err := ValidateLocalHost(t.Context(), directory, "web", ""); err == nil {
				t.Fatal("invalid descriptor field accepted")
			}
		})
	}
}

func TestNativeBundleCopiesLibrariesWithoutReplacingGameAssets(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()

	bundle := &hostBundle{
		Schema:     1,
		Renderer:   "webgpu",
		Target:     "linux-amd64",
		Entrypoint: "karty-host-webgpu",
		Files:      map[string]hostBundleFile{},
	}
	for name, data := range map[string][]byte{
		bundle.Entrypoint: nativeHeader(t, bundle.Target), "libkarty_webgpu.so": []byte("GPU facade"),
		"libwgpu_native.so": []byte("GPU driver"), "libSDL3.so.0": []byte("SDL"),
		"licenses/wgpu/LICENSE": []byte("license"),
	} {
		addBundleFixtureFile(t, directory, bundle, name, data)
	}

	writeHostBundleFixture(t, directory, bundle)

	loaded, err := readHostBundle(t.Context(), filepath.Join(directory, bundle.Entrypoint), "native", bundle.Target)
	if err != nil {
		t.Fatal(err)
	}

	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "game.kart"), []byte("my game"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := stageBundleFiles(t.Context(), loaded, target, bundle.Target); err != nil {
		t.Fatal(err)
	}

	resolved, bundled, err := ResolveLocalHostBundle(t.Context(), target, "native", bundle.Target)
	if err != nil || !bundled || resolved != filepath.Join(target, "karty-host") {
		t.Fatalf("staged descriptor lost the distribution entrypoint: %q, %v, %v", resolved, bundled, err)
	}

	for _, name := range []string{"karty-host", "libkarty_webgpu.so", "libwgpu_native.so", "libSDL3.so.0", "licenses/wgpu/LICENSE"} {
		if _, err := os.Stat(filepath.Join(target, name)); err != nil {
			t.Fatal(err)
		}
	}

	info, err := os.Stat(filepath.Join(target, "karty-host"))
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("native entrypoint not executable: %v", err)
	}

	contents, err := os.ReadFile(filepath.Join(target, "game.kart"))
	if err != nil || string(contents) != "my game" {
		t.Fatalf("game asset replaced: %q, %v", contents, err)
	}
}

func webHostBundleFixture(t *testing.T) (string, *hostBundle) {
	t.Helper()
	directory := t.TempDir()
	bundle := &hostBundle{Schema: 1, Renderer: "webgpu", Target: "web", Entrypoint: "karty-host.wasm", Files: map[string]hostBundleFile{}}

	for _, name := range webGPUHostFiles() {
		data := []byte("// " + name)
		if name == "karty-host.wasm" {
			data = []byte("\x00asm\x01\x00\x00\x00")
		}

		addBundleFixtureFile(t, directory, bundle, name, data)
	}

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

func addBundleFixtureFile(t *testing.T, directory string, bundle *hostBundle, name string, data []byte) {
	t.Helper()

	selected := filepath.Join(directory, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(selected), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(selected, data, 0o600); err != nil {
		t.Fatal(err)
	}

	digest := sha256.Sum256(data)
	bundle.Files[name] = hostBundleFile{Size: int64(len(data)), SHA256: hex.EncodeToString(digest[:])}
}

func writeHostBundleFixture(t *testing.T, directory string, bundle *hostBundle) {
	t.Helper()

	data, err := toml.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(directory, hostBundleName), data, 0o600); err != nil {
		t.Fatal(err)
	}
}
