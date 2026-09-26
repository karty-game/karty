package assetusage_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/karty-game/karty/internal/sdk"
)

// A patch SDK can keep its API and templates unchanged. Test this offline with
// the bootstrap bundle so the regression does not require a published release.
func TestLocalUIScreenReachabilityPatchSDK(t *testing.T) {
	t.Setenv("KARTY_HOME", t.TempDir())

	original, err := os.ReadFile("../sdk/bootstrap/sdk-0.0.1.zip")
	if err != nil {
		t.Fatal(err)
	}

	reader, err := zip.NewReader(bytes.NewReader(original), int64(len(original)))
	if err != nil {
		t.Fatal(err)
	}

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
			data = bytes.Replace(data, []byte("version = '0.0.1'"), []byte("version = '9.9.9'"), 1)
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

	testLocalUIScreenReachability(t, "9.9.9")
}
