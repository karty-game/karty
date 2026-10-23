package sdk

import (
	"archive/zip"
	"bytes"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/karty-game/karty/internal/release"
	"github.com/pelletier/go-toml/v2"
)

func TestBundleExposesDeclaredCompatibility(t *testing.T) {
	t.Parallel()

	data, err := currentTestBundle(t)
	if err != nil {
		t.Fatal(err)
	}

	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}

	metadata, err := fs.ReadFile(archive, "bundle.toml")
	if err != nil {
		t.Fatal(err)
	}

	var expected Compatibility

	if err := toml.Unmarshal(metadata, &expected); err != nil {
		t.Fatal(err)
	}

	manifest, err := readBundle(release.SDKVersion(), data)
	if err != nil {
		t.Fatal(err)
	}

	if manifest.Compatibility != expected {
		t.Fatalf("declared compatibility lost: got %+v, want %+v", manifest.Compatibility, expected)
	}

	if !manifest.SupportsAuthoredActions() {
		t.Fatal("current SDK must retain authored action support")
	}
}

func TestCandidateBundleRequiresStrictTOMLMetadata(t *testing.T) {
	t.Parallel()

	valid := "format = 2\nui_schema = 11\nproject_codegen = 2\n"
	for _, test := range []struct {
		name  string
		files fstest.MapFS
		valid bool
	}{
		{"candidate", fstest.MapFS{"bundle.toml": {Data: []byte(valid)}}, true},
		{"JSON only", fstest.MapFS{"bundle.json": {Data: []byte(`{"format":1,"ui_schema":11,"project_codegen":2}`)}}, false},
		{"JSON alongside TOML", fstest.MapFS{"bundle.toml": {Data: []byte(valid)}, "bundle.json": {Data: []byte(`{}`)}}, false},
		{"unknown field", fstest.MapFS{"bundle.toml": {Data: []byte(valid + "extra = true\n")}}, false},
		{"duplicate field", fstest.MapFS{"bundle.toml": {Data: []byte(valid + "format = 2\n")}}, false},
		{"legacy format", fstest.MapFS{"bundle.toml": {Data: []byte("format = 1\nui_schema = 11\nproject_codegen = 2\n")}}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// SDK 0.0.11 is the first release requiring strict TOML metadata.
			_, err := readCompatibility(test.files, "0.0.11")
			if (err == nil) != test.valid {
				t.Fatalf("unexpected metadata acceptance: %v", err)
			}
		})
	}

	legacy := fstest.MapFS{"bundle.json": {Data: []byte(`{"format":1,"ui_schema":9,"project_codegen":1}`)}}
	if metadata, err := readCompatibility(legacy, "0.0.10"); err != nil || metadata.Format != 1 {
		t.Fatal("released metadata compatibility changed", err)
	}
}

func TestLevelReferencesRequireCurrentAPI(t *testing.T) {
	t.Parallel()

	for _, version := range []string{"", "0.0.13", "0.0.14", "0.0.15"} {
		manifest := Manifest{}

		manifest.API.Version = version
		if got := manifest.SupportsLevelReferences(); got != (version == "0.0.14" || version == "0.0.15") {
			t.Fatalf("level reference support for API %q = %v", version, got)
		}
	}
}
