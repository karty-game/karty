package toolchain

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestManagedWebRuntimeLicensePreservedAndVerified(t *testing.T) {
	t.Parallel()

	const name = "licenses/Karty-RUNTIME_LICENSE.md"

	license := []byte("fixture runtime license")
	data, pin := managedBundleFixture(t, "web", func(bundle *HostBundle, files map[string][]byte) {
		files[name] = license
		bundle.Files[name] = HostBundleFile{Size: int64(len(license)), SHA256: fmt.Sprintf("%x", sha256.Sum256(license))}
	})
	cache := t.TempDir()
	downloads := 0

	bundle, err := installManagedHostBundle(t.Context(), cache, "0.0.21", "web", pin, bundleDownloader(data, &downloads))
	if err != nil {
		t.Fatal(err)
	}

	filename := filepath.Join(bundle.Directory, filepath.FromSlash(name))

	// #nosec G304 -- The managed fixture is installed inside t.TempDir and the license path is a fixed constant.
	got, err := os.ReadFile(filename)
	if err != nil || string(got) != string(license) {
		t.Fatal("managed web bundle lost its runtime license", err)
	}

	if err = os.WriteFile(filename, []byte("changed runtime license"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err = installManagedHostBundle(t.Context(), cache, "0.0.21", "web", pin, bundleDownloader(data, &downloads)); err == nil {
		t.Fatal("modified runtime license bypassed managed-cache checksum verification")
	}

	if downloads != 1 {
		t.Fatal("modified runtime license silently replaced its immutable cache", downloads)
	}
}

func TestWebRuntimeLicenseAllowlistStaysExact(t *testing.T) {
	t.Parallel()

	bundle := &HostBundle{Target: "web"}
	if !bundle.allowedFile("licenses/Karty-RUNTIME_LICENSE.md") {
		t.Fatal("required web runtime license rejected")
	}

	for _, name := range []string{
		"licenses/other.md", "licenses/karty-runtime_license.md", "../licenses/Karty-RUNTIME_LICENSE.md",
		"licenses/../Karty-RUNTIME_LICENSE.md", "/licenses/Karty-RUNTIME_LICENSE.md",
		"licenses\\Karty-RUNTIME_LICENSE.md", "licenses/Karty-RUNTIME_LICENSE.md/extra",
	} {
		if bundle.allowedFile(name) {
			t.Fatal("runtime license exception widened the web file allowlist", name)
		}
	}
}
