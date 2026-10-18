package sdk

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/release"
	"github.com/pelletier/go-toml/v2"
)

func TestSDKAcceptsAirFreeAndHistoricalSignedMetadata(t *testing.T) {
	t.Parallel()

	original, err := currentTestBundle(t)
	if err != nil {
		t.Fatal(err)
	}

	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	for _, historical := range []bool{false, true} {
		t.Run(map[bool]string{false: "without Air", true: "historical Air"}[historical], func(t *testing.T) {
			t.Parallel()

			checkAirManifestRoundTrip(t, original, public, private, historical)
		})
	}
}

func checkAirManifestRoundTrip(t *testing.T, original []byte, public ed25519.PublicKey, private ed25519.PrivateKey, historical bool) {
	t.Helper()

	manifest, err := readBundle(release.SDKVersion(), original)
	if err != nil {
		t.Fatal(err)
	}

	manifest.Tools.Air = ""
	manifest.Artifacts.Air = nil

	if historical {
		manifest.Tools.Air = "1.62.0"
		manifest.Artifacts.Air = map[string]ToolArtifact{
			"linux-amd64": {URL: "https://example.com/air", SHA256: strings.Repeat("a", 64)},
		}
	}

	contents, err := toml.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	if !historical && (bytes.Contains(contents, []byte("air =")) || bytes.Contains(contents, []byte("[artifacts.air"))) {
		t.Fatal("new manifest serialized obsolete Air metadata")
	}

	signature := ed25519.Sign(private, contents)

	published, err := parsePublishedManifest(manifest.Version, contents, signature, public)
	if err != nil {
		t.Fatal(err)
	}

	bundled, err := readBundle(manifest.Version, replaceBundleManifest(t, original, contents))
	if err != nil {
		t.Fatal(err)
	}

	if bundled.Tools != published.Tools || !reflect.DeepEqual(bundled.Artifacts, published.Artifacts) {
		t.Fatal("signed manifest and bundled metadata differ")
	}

	if published.Tools.Air != manifest.Tools.Air || !reflect.DeepEqual(published.Artifacts.Air, manifest.Artifacts.Air) {
		t.Fatal("historical signed Air metadata lost")
	}

	if _, err := parsePublishedManifest(
		manifest.Version, append(contents, '\n'), signature, public,
	); !errors.Is(err, errPublishedSDKSignature) {
		t.Fatalf("tampered raw manifest signature error = %v", err)
	}
}

func replaceBundleManifest(t *testing.T, original, contents []byte) []byte {
	t.Helper()

	archive, err := zip.NewReader(bytes.NewReader(original), int64(len(original)))
	if err != nil {
		t.Fatal(err)
	}

	var buffer bytes.Buffer

	writer := zip.NewWriter(&buffer)

	for _, file := range archive.File {
		data := contents
		if file.Name != "manifest.toml" {
			data, err = fs.ReadFile(archive, file.Name)
			if err != nil {
				t.Fatal(err)
			}
		}

		entry, err := writer.Create(file.Name)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := entry.Write(data); err != nil {
			t.Fatal(err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	return buffer.Bytes()
}
