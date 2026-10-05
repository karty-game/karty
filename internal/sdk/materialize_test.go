package sdk

import (
	"archive/zip"
	"bytes"
	"io/fs"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty/internal/release"
	"github.com/pelletier/go-toml/v2"
)

func materializeManifest() Manifest {
	manifest := Manifest{Version: "0.0.8"}
	manifest.Tools.Materialize = "2.0.0"
	manifest.Tools.MaterializeRevision = "3ad39f1308e3b2b62da81f557adc6d32697b616a"
	manifest.Artifacts.Materialize = map[string]MaterialToolArtifact{}

	for _, platform := range []string{"darwin-arm64", "linux-amd64", "linux-arm64", "windows-amd64"} {
		executable := "bin/materialize-cli"
		if platform == "windows-amd64" {
			executable += ".exe"
		}

		manifest.Artifacts.Materialize[platform] = MaterialToolArtifact{
			URL: "https://github.com/karty-game/karty-tools/releases/download/materialize-v2.0.0/" +
				"materialize-2.0.0-" + platform + ".zip",
			SHA256: strings.Repeat("a", 64), Format: "zip", Executable: executable,
		}
	}

	return manifest
}

func TestMaterializeSchemaRoundTrip(t *testing.T) {
	t.Parallel()

	want := materializeManifest()

	data, err := toml.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"materialize-revision", "[artifacts.materialize.linux-amd64]", "executable", "format"} {
		if !bytes.Contains(data, []byte(key)) {
			t.Fatalf("missing wire key %q", key)
		}
	}

	var got Manifest
	if err := toml.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}

	if got.Version != want.Version || got.Tools != want.Tools ||
		!reflect.DeepEqual(want.Artifacts.Materialize, got.Artifacts.Materialize) {
		t.Fatal("Materialize metadata lost during TOML round trip")
	}

	if err := ValidateMaterialize(got); err != nil {
		t.Fatal(err)
	}

	if err := ValidateMaterialize(Manifest{}); err != nil {
		t.Fatalf("older SDK rejected: %v", err)
	}
}

func TestMaterializeRejectsPartialAndMalformedPins(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		edit func(*Manifest)
	}{
		{"version absent", func(m *Manifest) { m.Tools.Materialize = "" }},
		{"version malformed", func(m *Manifest) { m.Tools.Materialize = "2.0.0-beta" }},
		{"revision absent", func(m *Manifest) { m.Tools.MaterializeRevision = "" }},
		{"revision malformed", func(m *Manifest) { m.Tools.MaterializeRevision = "main" }},
		{"revision uppercase", func(m *Manifest) { m.Tools.MaterializeRevision = strings.Repeat("A", 40) }},
		{"artifacts absent", func(m *Manifest) { m.Artifacts.Materialize = nil }},
		{"platform missing", func(m *Manifest) { delete(m.Artifacts.Materialize, "linux-arm64") }},
		{"platform extra", func(m *Manifest) { m.Artifacts.Materialize["darwin-amd64"] = MaterialToolArtifact{} }},
		{"platform substituted", func(m *Manifest) {
			delete(m.Artifacts.Materialize, "linux-arm64")
			m.Artifacts.Materialize["darwin-amd64"] = MaterialToolArtifact{}
		}},
		{"URL", func(m *Manifest) { editMaterialArtifact(m, func(a *MaterialToolArtifact) { a.URL += "?other" }) }},
		{"digest absent", func(m *Manifest) { editMaterialArtifact(m, func(a *MaterialToolArtifact) { a.SHA256 = "" }) }},
		{"digest uppercase", func(m *Manifest) {
			editMaterialArtifact(m, func(a *MaterialToolArtifact) { a.SHA256 = strings.Repeat("A", 64) })
		}},
		{"format", func(m *Manifest) { editMaterialArtifact(m, func(a *MaterialToolArtifact) { a.Format = "tar.gz" }) }},
		{"executable traversal", func(m *Manifest) {
			editMaterialArtifact(m, func(a *MaterialToolArtifact) { a.Executable = "../materialize-cli" })
		}},
		{"Windows executable", func(m *Manifest) {
			a := m.Artifacts.Materialize["windows-amd64"]
			a.Executable = "bin/materialize-cli"
			m.Artifacts.Materialize["windows-amd64"] = a
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			manifest := materializeManifest()
			test.edit(&manifest)

			if err := ValidateMaterialize(manifest); err == nil {
				t.Fatal("invalid pin accepted")
			}
		})
	}
}

func editMaterialArtifact(manifest *Manifest, edit func(*MaterialToolArtifact)) {
	artifact := manifest.Artifacts.Materialize["linux-amd64"]
	edit(&artifact)
	manifest.Artifacts.Materialize["linux-amd64"] = artifact
}

func TestWorldMaterialCapabilityRequiresPinnedTools(t *testing.T) {
	t.Parallel()

	released, err := Resolve("0.0.7")
	if err != nil {
		t.Fatal(err)
	}

	for _, pinned := range []bool{false, true} {
		manifest := released
		manifest.Version = "0.0.8"

		manifest.Assets.Capabilities.Runtime = append(slices.Clone(released.Assets.Capabilities.Runtime),
			asset.CapabilityWorldMaterialAtlasV1)

		if pinned {
			pin := materializeManifest()
			manifest.Tools.Materialize = pin.Tools.Materialize
			manifest.Tools.MaterializeRevision = pin.Tools.MaterializeRevision
			manifest.Artifacts.Materialize = pin.Artifacts.Materialize
		}

		err := validateManifest(manifest.Version, manifest)
		if pinned && err != nil {
			t.Fatalf("complete atlas SDK rejected: %v", err)
		}

		if !pinned && err == nil {
			t.Fatal("atlas SDK without managed Materialize pins accepted")
		}
	}

	if err := validateManifest(released.Version, released); err != nil {
		t.Fatalf("released SDK without atlas capability changed: %v", err)
	}
}

func TestBundleDecodesOptionalMaterializeAndRejectsPartialPin(t *testing.T) {
	t.Parallel()

	original, err := currentTestBundle(t)
	if err != nil {
		t.Fatal(err)
	}

	archive, err := zip.NewReader(bytes.NewReader(original), int64(len(original)))
	if err != nil {
		t.Fatal(err)
	}

	for _, partial := range []bool{false, true} {
		data := materializeCandidateBundle(t, archive, partial)

		manifest, err := readBundle("0.0.8", data)
		if partial {
			if err == nil || !strings.Contains(err.Error(), "Materialize") {
				t.Fatalf("partial bundle pin accepted: %v", err)
			}
		} else if err != nil || !reflect.DeepEqual(manifest.Artifacts.Materialize, materializeManifest().Artifacts.Materialize) {
			t.Fatalf("candidate metadata lost: %v", err)
		}
	}

	manifest, err := readBundle(release.SDKVersion(), original)
	if err != nil || manifest.Tools.Materialize != "2.0.0" || len(manifest.Artifacts.Materialize) != 4 {
		t.Fatalf("released default must retain its Materialize pin: %v", err)
	}
}

func materializeCandidateBundle(t *testing.T, archive *zip.Reader, partial bool) []byte {
	t.Helper()

	var buffer bytes.Buffer

	out := zip.NewWriter(&buffer)

	for _, file := range archive.File {
		data, err := fs.ReadFile(archive, file.Name)
		if err != nil {
			t.Fatal(err)
		}

		if file.Name == "manifest.toml" {
			var manifest Manifest
			if err := toml.Unmarshal(data, &manifest); err != nil {
				t.Fatal(err)
			}

			pin := materializeManifest()
			manifest.Version = pin.Version
			manifest.Tools.Materialize = pin.Tools.Materialize
			manifest.Tools.MaterializeRevision = pin.Tools.MaterializeRevision

			manifest.Artifacts.Materialize = pin.Artifacts.Materialize
			if partial {
				manifest.Tools.MaterializeRevision = ""
			}

			data, err = toml.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
		}

		writer, err := out.Create(file.Name)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := writer.Write(data); err != nil {
			t.Fatal(err)
		}
	}

	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	return buffer.Bytes()
}
