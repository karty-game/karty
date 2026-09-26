package sdk

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
)

func TestParsePublishedManifestVerifiesSignatureAndVersion(t *testing.T) {
	t.Parallel()

	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	contents := []byte(
		"version = '0.0.2'\n[api]\nversion = '0.0.2'\n[host]\nversion = '0.0.2'\n[templates]\ngame = '0.0.2'\n[assets.texture-profiles.sprite]\nprocessor = 'copy-png@1'\n[tools]\nair = '1'\ngo = '1'\ntinygo = '1'\nwasm-tools = '1'\n",
	)
	signature := ed25519.Sign(private, contents)

	manifest, err := parsePublishedManifest("0.0.2", contents, signature, public)
	if err != nil {
		t.Fatal(err)
	}

	if manifest.Version != "0.0.2" || manifest.Host.Version != "0.0.2" {
		t.Fatalf("published manifest = %+v", manifest)
	}

	if _, err := parsePublishedManifest("0.0.2", append(contents, 'x'), signature, public); !errors.Is(err, errPublishedSDKSignature) {
		t.Fatalf("tampered SDK error = %v, want signature error", err)
	}

	if _, err := parsePublishedManifest("0.0.1", contents, signature, public); !errors.Is(err, errIncompleteSDK) {
		t.Fatalf("wrong SDK version error = %v, want incomplete SDK error", err)
	}
}
