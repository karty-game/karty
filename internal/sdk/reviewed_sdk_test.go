package sdk

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/release"
)

func TestReviewedSDKAcceptsProjectAdapterThree(t *testing.T) {
	t.Parallel()

	data, err := currentTestBundle(t)
	if err != nil {
		t.Fatal(err)
	}

	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer

	writer := zip.NewWriter(&output)

	for _, entry := range archive.File {
		source, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}

		contents, err := io.ReadAll(source)
		if closeErr := source.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}

		if err != nil {
			t.Fatal(err)
		}

		if entry.Name == "bundle.toml" {
			contents = []byte(strings.Replace(string(contents), "project_codegen = 2", "project_codegen = 3", 1))
		}

		target, err := writer.CreateHeader(&entry.FileHeader)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := target.Write(contents); err != nil {
			t.Fatal(err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	manifest, err := readBundle(release.SDKVersion(), output.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	if manifest.Compatibility.ProjectCodegen != 3 || !manifest.SupportsAuthoredActions() {
		t.Fatal("reviewed SDK adapter lost authored actions")
	}
}
