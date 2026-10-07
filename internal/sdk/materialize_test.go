package sdk

import (
	"archive/zip"
	"bytes"
	"io/fs"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty/internal/release"
	"github.com/pelletier/go-toml/v2"
)

func materializeManifest(t *testing.T) Manifest {
	t.Helper()

	manifest, err := Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	manifest.Artifacts.Materialize = maps.Clone(manifest.Artifacts.Materialize)

	return manifest
}

func TestMaterializeSchemaRoundTrip(t *testing.T) {
	t.Parallel()

	want := materializeManifest(t)

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

	optional := want
	optional.Tools.Materialize = ""
	optional.Tools.MaterializeRevision = ""
	optional.Artifacts.Materialize = nil

	if err := ValidateMaterialize(optional); err != nil {
		t.Fatalf("absent optional Materialize metadata rejected: %v", err)
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

			manifest := materializeManifest(t)
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

	current := materializeManifest(t)

	for _, pinned := range []bool{false, true} {
		manifest := current
		if !pinned {
			manifest.Tools.Materialize = ""
			manifest.Tools.MaterializeRevision = ""
			manifest.Artifacts.Materialize = nil
		}

		err := validateManifest(manifest.Version, manifest)
		if pinned && err != nil {
			t.Fatalf("complete atlas SDK rejected: %v", err)
		}

		if !pinned && err == nil {
			t.Fatal("atlas SDK without managed Materialize pins accepted")
		}
	}

	optional := current
	optional.Assets.Capabilities.Runtime = slices.DeleteFunc(
		slices.Clone(current.Assets.Capabilities.Runtime),
		func(capability asset.Capability) bool {
			return capability == asset.CapabilityWorldMaterialAtlasV1
		},
	)
	optional.Tools.Materialize = ""
	optional.Tools.MaterializeRevision = ""
	optional.Artifacts.Materialize = nil

	if err := validateManifest(optional.Version, optional); err != nil {
		t.Fatalf("SDK without atlas capability rejected: %v", err)
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

	want := materializeManifest(t)

	for _, partial := range []bool{false, true} {
		data := materializeCandidateBundle(t, archive, partial)

		manifest, err := readBundle(release.SDKVersion(), data)
		if partial {
			if err == nil || !strings.Contains(err.Error(), "Materialize") {
				t.Fatalf("partial bundle pin accepted: %v", err)
			}
		} else if err != nil || !reflect.DeepEqual(manifest.Artifacts.Materialize, want.Artifacts.Materialize) {
			t.Fatalf("candidate metadata lost: %v", err)
		}
	}

	manifest, err := readBundle(release.SDKVersion(), original)
	if err != nil || manifest.Tools.Materialize != want.Tools.Materialize ||
		!reflect.DeepEqual(manifest.Artifacts.Materialize, want.Artifacts.Materialize) {
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

			pin := materializeManifest(t)
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
