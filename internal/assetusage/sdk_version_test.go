package assetusage_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

// A local patch SDK retains the current candidate API and SFC templates.
func TestLocalUIScreenReachabilityPatchSDK(t *testing.T) {
	if os.Getenv("KARTY_TEST_SDK") != release.SampleSDK {
		t.Skip("requires candidate SFC templates; set KARTY_TEST_SDK=0.0.9")
	}

	manifest, err := sdk.Resolve(release.SampleSDK)
	if err != nil {
		t.Fatal(err)
	}

	resources, err := sdk.Resources(manifest)
	if err != nil {
		t.Fatal(err)
	}

	reader, ok := resources.(*zip.Reader)
	if !ok {
		t.Fatal("SDK resources are not a ZIP archive")
	}

	t.Setenv("KARTY_HOME", t.TempDir())

	var output bytes.Buffer

	writer := zip.NewWriter(&output)

	for _, file := range reader.File {
		stream, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}

		data, err := io.ReadAll(stream)
		if closeErr := stream.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}

		if err != nil {
			t.Fatal(err)
		}

		if file.Name == "manifest.toml" {
			data = bytes.Replace(data, []byte("version = '"+release.SampleSDK+"'"), []byte("version = '9.9.9'"), 1)
		}

		entry, err := writer.Create(file.Name)
		if err != nil {
			t.Fatal(err)
		}

		if _, err = entry.Write(data); err != nil {
			t.Fatal(err)
		}
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
