package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/toolchain"
)

func hostedSampleFixture(t *testing.T, site, name string) string {
	t.Helper()
	source := t.TempDir()
	files := writeWebGPUPreviewBundle(t, source)
	shell := `<html><script>const scripts = [["wasm_exec.js", false], ["bootstrap.mjs", true]];</script></html>`
	writeFixture(t, filepath.Join(source, "index.html"), shell)
	writeFixture(t, filepath.Join(source, "karty-sw.js"), "worker")
	writeFixture(t, filepath.Join(source, "content", "current.kld"), "current")
	writeFixture(t, filepath.Join(source, "content", "obsolete.kld"), "obsolete")

	entries := make([]hostedReleaseFile, 0, len(files)+5)

	files = append(files, "index.html", "game.kart", "karty-web.js", "karty-web-status.js", "content/current.kld")
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(source, filepath.FromSlash(file)))
		if err != nil {
			t.Fatal(err)
		}

		role := "startup"
		if file == "content/current.kld" {
			role = "background"
		}

		entries = append(
			entries,
			hostedReleaseFile{URL: file, SHA256: hashBytes(data), Size: int64(len(data)), Type: "application/octet-stream", Role: role},
		)
	}

	data, err := json.Marshal(hostedRelease{Version: 1, Files: entries})
	if err != nil {
		t.Fatal(err)
	}

	pointer := hostedPointer{Version: 1, Release: hashBytes(data), Manifest: "karty-releases/" + hashBytes(data) + ".json"}
	writeHostedReleaseFixture(t, source, pointer.Manifest, data)

	data, err = json.Marshal(pointer)
	if err != nil {
		t.Fatal(err)
	}

	writeFixture(t, filepath.Join(source, "karty-latest.json"), string(data))

	if err = os.CopyFS(filepath.Join(site, name), os.DirFS(source)); err != nil {
		t.Fatal(err)
	}

	return source
}

func TestHostedRuntimeSharesVerifiedBytesAndPreservesStandaloneBuilds(t *testing.T) {
	t.Parallel()

	if !slices.Contains(hostedRuntimeFiles(), "licenses/Karty-RUNTIME_LICENSE.md") {
		t.Fatal("shared runtime inventory must include the exact runtime license path")
	}

	site := t.TempDir()
	writeFixture(t, filepath.Join(site, "index.html"), "gallery")

	originals := make([]string, 0, 2)
	for _, name := range []string{"one", "two"} {
		originals = append(originals, hostedSampleFixture(t, site, name))
		if err := shareSampleRuntime(site, name, release.SDKVersion()); err != nil {
			t.Fatal(err)
		}

		assertHostedSample(t, site, name)
	}

	versions, err := os.ReadDir(filepath.Join(site, "runtime", release.SDKVersion()))
	if err != nil || len(versions) != 1 {
		t.Fatalf("shared runtime inventory = %v, %v", versions, err)
	}

	for _, source := range originals {
		if _, err := toolchain.ReadHostBundle(t.Context(), source, "web", ""); err != nil {
			t.Fatal("standalone source changed", err)
		}
	}

	if err := validateSamples(os.DirFS(site)); err != nil {
		t.Fatal(err)
	}
}

func assertHostedSample(t *testing.T, site, name string) {
	t.Helper()

	directory := filepath.Join(site, name)
	for _, absent := range []string{"runtime.bin", "karty-host.wasm", toolchain.HostBundleName, "content/obsolete.kld"} {
		if _, err := os.Stat(filepath.Join(directory, filepath.FromSlash(absent))); !os.IsNotExist(err) {
			t.Fatalf("unshared file %s: %v", absent, err)
		}
	}

	var pointer hostedPointer
	decodeHostedFixture(t, filepath.Join(directory, "karty-latest.json"), &pointer)

	var manifest hostedRelease
	decodeHostedFixture(t, filepath.Join(directory, filepath.FromSlash(pointer.Manifest)), &manifest)

	data, err := os.ReadFile(filepath.Join(directory, filepath.FromSlash(pointer.Manifest)))
	if err != nil || hashBytes(data) != pointer.Release {
		t.Fatalf("release hash %v", err)
	}

	shared := 0

	for _, entry := range manifest.Files {
		bytes, err := os.ReadFile(filepath.Join(directory, filepath.FromSlash(entry.URL)))
		if err != nil || hashBytes(bytes) != entry.SHA256 || int64(len(bytes)) != entry.Size {
			t.Fatalf("release integrity %s: %v", entry.URL, err)
		}

		if strings.HasPrefix(entry.URL, "../runtime/") {
			shared++

			if entry.Role != "startup" {
				t.Fatal("shared background file")
			}
		}
	}

	if shared != len(hostedRuntimeFiles()) {
		t.Fatalf("shared startup count %d", shared)
	}

	shell, err := os.ReadFile(filepath.Join(directory, "index.html"))
	if err != nil || !strings.Contains(string(shell), `[["../runtime/`) {
		t.Fatalf("shared shell %q, %v", shell, err)
	}
}

func decodeHostedFixture(t *testing.T, name string, target any) {
	t.Helper()

	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}

	if err = json.Unmarshal(data, target); err != nil {
		t.Fatal(err)
	}
}

func TestHostedRuntimeRejectsIncompleteChangedAndExtraFiles(t *testing.T) {
	t.Parallel()

	for _, change := range []string{
		"missing source", "changed runtime", "missing runtime", "changed license", "missing license", "extra alias", "changed inventory",
	} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()
			site := t.TempDir()
			hostedSampleFixture(t, site, "one")

			if change == "missing source" {
				if err := os.Remove(filepath.Join(site, "one", "texture-worker.mjs")); err != nil {
					t.Fatal(err)
				}

				if err := shareSampleRuntime(site, "one", release.SDKVersion()); err == nil {
					t.Fatal("incomplete original accepted")
				}

				return
			}

			if err := shareSampleRuntime(site, "one", release.SDKVersion()); err != nil {
				t.Fatal(err)
			}

			versions, err := os.ReadDir(filepath.Join(site, "runtime", release.SDKVersion()))
			if err != nil {
				t.Fatal(err)
			}

			shared := filepath.Join(site, "runtime", release.SDKVersion(), versions[0].Name())

			corruptHostedRuntimeFixture(t, shared, change)

			if err := validateRuntime(os.DirFS(site)); err == nil {
				t.Fatal("invalid hosted runtime accepted")
			}
		})
	}
}

func corruptHostedRuntimeFixture(t *testing.T, shared, change string) {
	t.Helper()

	switch change {
	case "changed runtime":
		writeFixture(t, filepath.Join(shared, "client.mjs"), "changed")
	case "missing runtime":
		if err := os.Remove(filepath.Join(shared, "client.mjs")); err != nil {
			t.Fatal(err)
		}
	case "changed license":
		const name = "licenses/Karty-RUNTIME_LICENSE.md"
		writeFixture(t, filepath.Join(shared, filepath.FromSlash(name)), strings.Repeat("x", len("bundle "+name)))
	case "missing license":
		if err := os.Remove(filepath.Join(shared, "licenses", "Karty-RUNTIME_LICENSE.md")); err != nil {
			t.Fatal(err)
		}
	case "extra alias":
		writeFixture(t, filepath.Join(shared, "runtime.bin"), "alias")
	case "changed inventory":
		writeFixture(t, filepath.Join(shared, hostedRuntimeName), "{}")
	}
}

func TestHostedSampleRejectsMissingRuntimeStartupEntry(t *testing.T) {
	t.Parallel()
	site := t.TempDir()
	hostedSampleFixture(t, site, "one")
	directory := filepath.Join(site, "one")

	var pointer hostedPointer
	decodeHostedFixture(t, filepath.Join(directory, "karty-latest.json"), &pointer)

	var manifest hostedRelease
	decodeHostedFixture(t, filepath.Join(directory, filepath.FromSlash(pointer.Manifest)), &manifest)

	for i, entry := range manifest.Files {
		if entry.URL == "client.mjs" {
			manifest.Files = append(manifest.Files[:i], manifest.Files[i+1:]...)

			break
		}
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	pointer.Release = hashBytes(data)
	pointer.Manifest = "karty-releases/" + pointer.Release + ".json"
	writeHostedReleaseFixture(t, directory, pointer.Manifest, data)

	data, err = json.Marshal(pointer)
	if err != nil {
		t.Fatal(err)
	}

	writeFixture(t, filepath.Join(directory, "karty-latest.json"), string(data))

	if err := shareSampleRuntime(site, "one", release.SDKVersion()); err == nil {
		t.Fatal("incomplete runtime startup manifest accepted")
	}
}

func TestHostedRuntimeRejectsDifferentBundleForSameSDK(t *testing.T) {
	t.Parallel()
	site := t.TempDir()
	hostedSampleFixture(t, site, "one")
	hostedSampleFixture(t, site, "two")

	if err := shareSampleRuntime(site, "one", release.SDKVersion()); err != nil {
		t.Fatal(err)
	}

	descriptor := filepath.Join(site, "two", toolchain.HostBundleName)

	original, err := os.ReadFile(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	// A different complete source descriptor is a distinct pinned release even
	// when the current fixture payloads happen to have the same bytes.
	writeFixture(t, descriptor, string(original)+"\n")

	if err := shareSampleRuntime(site, "two", release.SDKVersion()); err == nil {
		t.Fatal("distinct bundle silently merged")
	}
}

func writeHostedReleaseFixture(t *testing.T, directory, name string, data []byte) {
	t.Helper()

	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	if err = root.MkdirAll("karty-releases", 0o700); err != nil {
		t.Fatal(err)
	}

	if err = root.WriteFile(name, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCompactCandidateNoticesRequireCompleteRegularInputs(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	writeFixture(t, filepath.Join(directory, "RUNTIME_LICENSE.md"), "candidate runtime license")

	if _, err := readCompactNotices(directory); err == nil {
		t.Fatal("missing SDK notice accepted")
	}

	writeFixture(t, filepath.Join(directory, "SDK_LICENSE.md"), "candidate SDK license")

	notices, err := readCompactNotices(directory)
	if err != nil || string(notices["SDK_LICENSE.md"]) != "candidate SDK license" {
		t.Fatalf("candidate notices %v, %v", notices, err)
	}

	if err := os.Truncate(filepath.Join(directory, "SDK_LICENSE.md"), maxNoticeBytes+1); err != nil {
		t.Fatal(err)
	}

	if _, err := readCompactNotices(directory); err == nil {
		t.Fatal("oversized notice accepted")
	}
}

func TestPublisherRejectsChangedHostedRuntimeBeforeReplacingPreview(t *testing.T) {
	t.Parallel()

	state, source := sampleFixture(t)
	if err := assemble(state, source, "main", false, nil); err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(filepath.Join(source, "ui-demo")); err != nil {
		t.Fatal(err)
	}

	hostedSampleFixture(t, source, "ui-demo")

	if err := shareSampleRuntime(source, "ui-demo", release.SDKVersion()); err != nil {
		t.Fatal(err)
	}

	versions, err := os.ReadDir(filepath.Join(source, "runtime", release.SDKVersion()))
	if err != nil || len(versions) != 1 {
		t.Fatalf("hosted runtime versions: %v, %v", versions, err)
	}

	shared := filepath.Join(source, "runtime", release.SDKVersion(), versions[0].Name())
	corruptHostedRuntimeFixture(t, shared, "changed runtime")

	if err := assemble(state, source, "main", false, nil); err == nil {
		t.Fatal("changed runtime accepted")
	}

	if err := requireFile(filepath.Join(state, "main", "ui-demo", "index.html")); err != nil {
		t.Fatal("working preview replaced before runtime validation", err)
	}
}
