package assetusage_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty/internal/assetusage"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

func TestSelectedSDKTextureSetterRetainsReferences(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	for _, compiler := range []string{"go", "tinygo"} {
		t.Run(compiler, func(t *testing.T) {
			t.Parallel()

			files, err := sdk.ClientFiles(manifest, compiler)
			if err != nil {
				t.Fatal(err)
			}

			directory := usageProject(t, `package main
import "example.com/game/.karty/engine"
func main() {
    sprite := new(engine.Game).NewSprite2D("sprites.unused", engine.Vec2{})
    sprite.SetTexture("sprites.player")
}
`)
			if err := os.Remove(filepath.Join(directory, ".karty", "engine", "engine.go")); err != nil {
				t.Fatal(err)
			}

			for name, data := range files {
				path := filepath.Join(directory, ".karty", filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
					t.Fatal(err)
				}

				if err := os.WriteFile(path, data, 0o600); err != nil {
					t.Fatal(err)
				}
			}

			result := assetusage.AnalyzeWithOptions(
				directory, "example.com/game", []string{"sprites.player", "sprites.unused", "sprites.dead"}, nil,
				assetusage.Options{Compiler: compiler},
			)
			if result.KeepAll || result.Live["sprites.player"] == "" || result.Live["sprites.unused"] == "" ||
				result.Live["sprites.dead"] != "" {
				t.Fatalf("selected SDK texture references: %+v", result)
			}
		})
	}
}

// A locally repackaged SDK retains the latest API and SFC templates.
func TestLocalUIScreenReachabilityPatchSDK(t *testing.T) {
	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	resources, err := sdk.ResourcesContext(t.Context(), manifest)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("KARTY_HOME", t.TempDir())

	var output bytes.Buffer

	writer := zip.NewWriter(&output)

	err = fs.WalkDir(resources, ".", func(name string, file fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if file.IsDir() {
			return nil
		}

		data, err := fs.ReadFile(resources, name)
		if err != nil {
			return err
		}

		if name == "manifest.toml" {
			data = bytes.Replace(data, []byte("version = '"+release.SDKVersion()+"'"), []byte("version = '9.9.9'"), 1)
		}

		entry, err := writer.Create(name)
		if err != nil {
			return err
		}

		_, err = entry.Write(data)

		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	data := output.Bytes()
	if err := sdk.InstallBundle("9.9.9", data, fmt.Sprintf("%x", sha256.Sum256(data))); err != nil {
		t.Fatal(err)
	}

	testLocalUIScreenReachability(t, "9.9.9", ".kui")
}
