package sdk

import (
	"maps"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func ktxManifestFixture() Manifest {
	manifest := Manifest{}
	manifest.Tools.KTX, manifest.Tools.KTXRevision = "4.4.2", "4d6fc70eaf62ad0558e63e8d97eb9766118327a6"
	manifest.Artifacts.KTX = make(map[string]MaterialToolArtifact)

	for _, platform := range []struct{ name, asset, format string }{
		{"darwin-arm64", "Darwin-arm64.pkg", "pkg"},
		{"linux-amd64", "Linux-x86_64.tar.bz2", "tar.bz2"},
		{"linux-arm64", "Linux-arm64.tar.bz2", "tar.bz2"},
		{"windows-amd64", "Windows-x64.exe", "nsis"},
	} {
		executable := "bin/ktx"
		if strings.HasPrefix(platform.name, "windows-") {
			executable += ".exe"
		}

		manifest.Artifacts.KTX[platform.name] = MaterialToolArtifact{
			URL:    "https://github.com/KhronosGroup/KTX-Software/releases/download/v4.4.2/KTX-Software-4.4.2-" + platform.asset,
			SHA256: strings.Repeat("a", 64), Format: platform.format, Executable: executable}
	}

	return manifest
}

func TestKTXManifestRequiresCompletePinnedTool(t *testing.T) {
	t.Parallel()

	if ValidateKTX(Manifest{}) != nil {
		t.Fatal("historical SDK requires KTX")
	}

	valid := ktxManifestFixture()
	if err := ValidateKTX(valid); err != nil {
		t.Fatal(err)
	}

	for _, mutate := range []func(*Manifest){
		func(m *Manifest) { m.Tools.KTXRevision = "main" },
		func(m *Manifest) { delete(m.Artifacts.KTX, "linux-arm64") },
		func(m *Manifest) {
			entry := m.Artifacts.KTX["linux-amd64"]
			entry.Executable = "../ktx"
			m.Artifacts.KTX["linux-amd64"] = entry
		},
		func(m *Manifest) {
			entry := m.Artifacts.KTX["linux-amd64"]
			entry.URL = "https://example.com/unpinned.zip"
			m.Artifacts.KTX["linux-amd64"] = entry
		},
	} {
		changed := valid
		changed.Artifacts.KTX = maps.Clone(valid.Artifacts.KTX)
		mutate(&changed)

		if ValidateKTX(changed) == nil {
			t.Fatal("incomplete or untrusted tool pin accepted")
		}
	}

	encoded, err := toml.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}

	var decoded Manifest
	if err := toml.Unmarshal(encoded, &decoded); err != nil || ValidateKTX(decoded) != nil ||
		decoded.Tools != valid.Tools || !maps.Equal(decoded.Artifacts.KTX, valid.Artifacts.KTX) {
		t.Fatal("KTX pin lost in round trip", err)
	}
}
