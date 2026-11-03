package toolchain

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"debug/elf"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/sdk"
	"github.com/pelletier/go-toml/v2"
)

func managedBundleFixture(t *testing.T, target string, edit func(*HostBundle, map[string][]byte)) ([]byte, sdk.HostBundleArtifact) {
	t.Helper()

	bundle := &HostBundle{Schema: 1, Renderer: "webgpu", Target: target, Entrypoint: "karty-host.wasm", Files: map[string]HostBundleFile{}}
	files := map[string][]byte{}

	if target == "web" {
		for _, name := range WebGPUHostFiles() {
			files[name] = []byte("module " + name)
		}

		files[bundle.Entrypoint] = []byte("\x00asm\x01\x00\x00\x00")
		files["runtime.bin"] = files[bundle.Entrypoint]
	} else {
		bundle.Entrypoint = "karty-host-webgpu"

		var (
			header bytes.Buffer
			ident  [16]byte
		)
		copy(ident[:], "\x7fELF\x02\x01\x01")

		if err := binary.Write(
			&header,
			binary.LittleEndian,
			elf.Header64{Ident: ident, Type: uint16(elf.ET_EXEC), Machine: uint16(elf.EM_X86_64), Version: 1, Ehsize: 64},
		); err != nil {
			t.Fatal(err)
		}

		files[bundle.Entrypoint] = header.Bytes()
		for _, name := range []string{"libkarty_webgpu.so", "libwgpu_native.so", "libSDL3.so.0", "licenses/Karty-RUNTIME_LICENSE.md", "licenses/SDL-LICENSE.txt", "licenses/wgpu-native-LICENSE-APACHE.txt", "licenses/wgpu-native-LICENSE-MIT.txt", "licenses/Go-WebGPU-LICENSE.txt", "licenses/go-sdl3-LICENSE.txt"} {
			files[name] = []byte(name)
		}
	}

	for name, payload := range files {
		bundle.Files[name] = HostBundleFile{Size: int64(len(payload)), SHA256: fmt.Sprintf("%x", sha256.Sum256(payload))}
	}

	if target == "web" {
		addWebCodecFixture(bundle, files)
	}

	if edit != nil {
		edit(bundle, files)
	}

	descriptor, err := toml.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}

	files[HostBundleName] = descriptor

	var output bytes.Buffer

	archive := zip.NewWriter(&output)

	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		f, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := f.Write(files[name]); err != nil {
			t.Fatal(err)
		}
	}

	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}

	encoded := output.Bytes()

	return encoded, sdk.HostBundleArtifact{
		URL:    "https://example.invalid/complete.zip",
		SHA256: fmt.Sprintf("%x", sha256.Sum256(encoded)),
		Format: "zip",
	}
}

func bundleDownloader(data []byte, count *int) func(context.Context, string) (io.ReadCloser, error) {
	return func(context.Context, string) (io.ReadCloser, error) {
		*count++

		return io.NopCloser(bytes.NewReader(data)), nil
	}
}

func TestManagedHostBundleAtomicInstallAndVerifiedWarmCache(t *testing.T) {
	t.Parallel()

	for _, target := range []string{"web", "linux-amd64"} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			data, pin := managedBundleFixture(t, target, nil)
			cache := t.TempDir()
			downloads := 0

			bundle, err := installManagedHostBundle(t.Context(), cache, "0.0.21", target, pin, bundleDownloader(data, &downloads))
			if err != nil {
				t.Fatal(err)
			}

			if _, err := os.Stat(filepath.Join(bundle.Directory, bundle.Entrypoint)); err != nil {
				t.Fatal(err)
			}

			if _, err := installManagedHostBundle(
				t.Context(),
				cache,
				"0.0.21",
				target,
				pin,
				bundleDownloader(data, &downloads),
			); err != nil {
				t.Fatal(err)
			}

			if downloads != 1 {
				t.Fatal("warm cache downloaded again", downloads)
			}

			if err := os.WriteFile(filepath.Join(bundle.Directory, bundle.Entrypoint), []byte("tampered"), 0o600); err != nil {
				t.Fatal(err)
			}

			if _, err := installManagedHostBundle(
				t.Context(),
				cache,
				"0.0.21",
				target,
				pin,
				bundleDownloader(data, &downloads),
			); err == nil {
				t.Fatal("corrupt cache trusted")
			}

			if downloads != 1 {
				t.Fatal("corrupt cache silently overwritten")
			}
		})
	}
}

func TestManagedHostBundleRejectsMalformedWithoutPublication(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"traversal", "undeclared", "case-collision", "missing", "hash", "alias", "renderer", "target", "descriptor-rewrite"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			data, pin := managedBundleFixture(t, "web", func(bundle *HostBundle, files map[string][]byte) {
				switch kind {
				case "traversal":
					files["../escape"] = []byte("escape")
				case "undeclared":
					files["untracked.txt"] = []byte("extra")
				case "case-collision":
					files["CLIENT.MJS"] = files["client.mjs"]
				case "missing":
					delete(files, "runtime.mjs")
				case "hash":
					files["runtime.mjs"] = []byte("bad bytes")
				case "alias":
					bundle.Files["runtime.bin"] = bundle.Files["client.mjs"]
					files["runtime.bin"] = files["client.mjs"]
				case "renderer":
					bundle.Renderer = "unknown"
				case "target":
					bundle.Target = "linux-amd64"
				case "descriptor-rewrite":
					bundle.Schema = 2
				}
			})
			cache := t.TempDir()

			downloads := 0
			if _, err := installManagedHostBundle(
				t.Context(),
				cache,
				"0.0.21",
				"web",
				pin,
				bundleDownloader(data, &downloads),
			); err == nil {
				t.Fatal("malformed bundle accepted", kind)
			}

			entries, _ := filepath.Glob(filepath.Join(cache, "hosts", "0.0.21", "web", "*"))
			if len(entries) != 0 {
				t.Fatal("failed bundle left publication", entries)
			}
		})
	}
}

func TestManagedHostBundleCacheDescriptorIsBoundToSignedArchive(t *testing.T) {
	t.Parallel()

	data, pin := managedBundleFixture(t, "web", nil)
	cache := t.TempDir()
	downloads := 0

	bundle, err := installManagedHostBundle(t.Context(), cache, "0.0.21", "web", pin, bundleDownloader(data, &downloads))
	if err != nil {
		t.Fatal(err)
	}

	changed := []byte("modified but internally consistent")
	if err := os.WriteFile(filepath.Join(bundle.Directory, "runtime.mjs"), changed, 0o600); err != nil {
		t.Fatal(err)
	}

	bundle.Files["runtime.mjs"] = HostBundleFile{Size: int64(len(changed)), SHA256: fmt.Sprintf("%x", sha256.Sum256(changed))}

	encoded, err := toml.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(bundle.Directory, HostBundleName), encoded, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := installManagedHostBundle(t.Context(), cache, "0.0.21", "web", pin, bundleDownloader(data, &downloads)); err == nil {
		t.Fatal("rewritten cache descriptor became its own trust anchor")
	}
}

func TestManagedHostBundleBoundsAndCancellation(t *testing.T) {
	t.Parallel()

	data, pin := managedBundleFixture(t, "web", nil)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	downloads := 0
	if _, err := installManagedHostBundle(
		ctx,
		t.TempDir(),
		"0.0.21",
		"web",
		pin,
		bundleDownloader(data, &downloads),
	); err == nil ||
		downloads != 0 {
		t.Fatal("canceled install downloaded/published")
	}

	wrong := pin

	wrong.SHA256 = strings.Repeat("0", 64)
	if _, err := installManagedHostBundle(
		t.Context(),
		t.TempDir(),
		"0.0.21",
		"web",
		wrong,
		bundleDownloader(data, &downloads),
	); err == nil {
		t.Fatal("archive checksum mismatch accepted")
	}
	// Corrupt only the central-directory decoded length: a tiny ZIP must not
	// allocate the claimed huge image/file before checking its explicit bound.
	index := bytes.Index(data, []byte("PK\x01\x02"))
	if index < 0 {
		t.Fatal("ZIP directory missing")
	}

	for _, kind := range []string{"decoded-size", "symlink"} {
		bad := bytes.Clone(data)
		if kind == "decoded-size" {
			binary.LittleEndian.PutUint32(bad[index+24:], HostBundleFileLimit+1)
		} else {
			binary.LittleEndian.PutUint16(bad[index+4:], 3<<8|20)
			binary.LittleEndian.PutUint32(bad[index+38:], (0o120000|0o777)<<16)
		}

		pin.SHA256 = fmt.Sprintf("%x", sha256.Sum256(bad))
		if _, err := installManagedHostBundle(
			t.Context(),
			t.TempDir(),
			"0.0.21",
			"web",
			pin,
			bundleDownloader(bad, &downloads),
		); err == nil {
			t.Fatal("invalid ZIP member accepted", kind)
		}
	}
}
