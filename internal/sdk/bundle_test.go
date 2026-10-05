package sdk

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/release"
)

func TestBundleInstallIsImmutableAndOffline(t *testing.T) {
	data, err := currentTestBundle(t)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("KARTY_HOME", t.TempDir())

	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	if err = InstallBundle(release.SDKVersion(), data, hash); err != nil {
		t.Fatal(err)
	}

	if err = InstallBundle(release.SDKVersion(), data, hash); err != nil {
		t.Fatal(err)
	}

	manifest, err := Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	for _, compiler := range []string{"go", "tinygo"} {
		files, err := ClientFiles(manifest, compiler)
		if err != nil || len(files) != 4 {
			t.Fatalf("bindings: %v", err)
		}
	}

	if err = InstallBundle(release.SDKVersion(), data, "0000000000000000000000000000000000000000000000000000000000000000"); err == nil {
		t.Fatal("checksum mismatch accepted")
	}

	cache, _ := sdkCache()
	if err = os.WriteFile(filepath.Join(cache, release.SDKVersion(), "sdk.zip"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err = Resolve(release.SDKVersion()); err == nil {
		t.Fatal("corrupted cache accepted")
	}
}

//nolint:gocognit // Table exercises independent malformed archive variants.
func TestBundleRejectsUnsafeEntriesAndIncompatibleMetadata(t *testing.T) {
	t.Parallel()

	original, err := currentTestBundle(t)
	if err != nil {
		t.Fatal(err)
	}

	archive, err := zip.NewReader(bytes.NewReader(original), int64(len(original)))
	if err != nil {
		t.Fatal(err)
	}

	for _, kind := range []string{"traversal", "absolute", "backslash", "duplicate", "symlink", "format", "schema", "generator", "version", "missing"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			var buffer bytes.Buffer

			out := zip.NewWriter(&buffer)

			for _, file := range archive.File {
				if kind == "missing" && file.Name == "bindings/go/engine/game.go" {
					continue
				}

				data, _ := fs.ReadFile(archive, file.Name)
				if file.Name == "bundle.json" {
					switch kind {
					case "format":
						data = []byte(`{"format":2,"ui_schema":9,"project_codegen":1}`)
					case "schema":
						data = []byte(`{"format":1,"ui_schema":99,"project_codegen":1}`)
					case "generator":
						data = []byte(`{"format":1,"ui_schema":9,"project_codegen":99}`)
					}
				}

				if file.Name == "manifest.toml" && kind == "version" {
					data = bytes.Replace(data, []byte("version = '"+release.SDKVersion()+"'"), []byte("version = '9.9.9'"), 1)
				}

				writer, _ := out.Create(file.Name)
				if _, err := writer.Write(data); err != nil {
					t.Fatal(err)
				}
			}

			names := map[string]string{
				"traversal": "../escape",
				"absolute":  "/escape",
				"backslash": "bad\\entry",
				"duplicate": "manifest.toml",
				"symlink":   "link",
			}
			if name, ok := names[kind]; ok {
				header := &zip.FileHeader{Name: name}
				if kind == "symlink" {
					header.SetMode(os.ModeSymlink | 0777)
				}

				writer, _ := out.CreateHeader(header)
				if _, err := writer.Write([]byte("bad")); err != nil {
					t.Fatal(err)
				}
			}

			out.Close()

			if _, err := readBundle(release.SDKVersion(), buffer.Bytes()); err == nil {
				t.Fatal("invalid bundle accepted")
			}
		})
	}
}
func TestBundleVersionConfinement(t *testing.T) {
	t.Parallel()

	for _, version := range []string{"../escape", "/tmp/escape", "1.0.0/../../escape", "", "1.0.0?x"} {
		if _, err := Resolve(version); err == nil {
			t.Fatalf("accepted %q", version)
		}
	}
}

func TestResolveRequiresInstalledSDK(t *testing.T) {
	t.Setenv("KARTY_HOME", t.TempDir())

	_, err := Resolve(release.SDKVersion())
	if !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "karty sdk install "+release.SDKVersion()) {
		t.Fatalf("missing SDK error=%v, want installation instructions", err)
	}
}
