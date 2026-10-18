package assetusage_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"testing"

	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

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
