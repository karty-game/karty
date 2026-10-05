package sdk

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io/fs"
	"testing"

	"github.com/karty-game/karty/internal/release"
)

func TestParsePublishedManifestVerifiesSignatureAndVersion(t *testing.T) {
	t.Parallel()

	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	data, err := currentTestBundle(t)
	if err != nil {
		t.Fatal(err)
	}

	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}

	contents, err := fs.ReadFile(archive, "manifest.toml")
	if err != nil {
		t.Fatal(err)
	}

	signature := ed25519.Sign(private, contents)

	manifest, err := parsePublishedManifest(release.SDKVersion(), contents, signature, public)
	if err != nil {
		t.Fatal(err)
	}

	if manifest.Version != release.SDKVersion() || manifest.Host.Version != release.SDKVersion() {
		t.Fatalf("published manifest = %+v", manifest)
	}

	if _, err := parsePublishedManifest(
		release.SDKVersion(), append(contents, 'x'), signature, public,
	); !errors.Is(err, errPublishedSDKSignature) {
		t.Fatalf("tampered SDK error = %v, want signature error", err)
	}

	if _, err := parsePublishedManifest("9.9.9", contents, signature, public); !errors.Is(err, errIncompleteSDK) {
		t.Fatalf("wrong SDK version error = %v, want incomplete SDK error", err)
	}
}
