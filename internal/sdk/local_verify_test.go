package sdk

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/release"
)

func TestLocalManifestVerificationAcceptsPublishedSignature(t *testing.T) {
	t.Parallel()

	contents, err := os.ReadFile("testdata/published-sdk-0.0.19.toml")
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := os.ReadFile("testdata/published-sdk-0.0.19.signature.base64")
	if err != nil {
		t.Fatal(err)
	}

	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		t.Fatal(err)
	}

	manifest, err := VerifyPublishedManifest("0.0.19", contents, signature)
	if err != nil || manifest.Version != "0.0.19" {
		t.Fatal("published release did not authenticate through CLI trust", err)
	}

	contents[len(contents)-1] ^= 1
	if _, err = VerifyPublishedManifest("0.0.19", contents, signature); !errors.Is(err, errPublishedSDKSignature) {
		t.Fatal("tampered manifest authenticated", err)
	}

	contents[len(contents)-1] ^= 1
	if _, err = VerifyPublishedManifest("9.9.9", contents, signature); !errors.Is(err, errIncompleteSDK) {
		t.Fatal("wrong manifest version authenticated", err)
	}
}

func TestLocalManifestVerificationRejectsAnUntrustedSigningKey(t *testing.T) {
	t.Parallel()

	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	contents := []byte("version = '" + release.SDKVersion() + "'\n")
	signature := ed25519.Sign(private, contents)

	if !ed25519.Verify(public, contents, signature) {
		t.Fatal("fixture signature is invalid")
	}

	if _, err = VerifyPublishedManifest(release.SDKVersion(), contents, signature); !errors.Is(err, errPublishedSDKSignature) {
		t.Fatalf("release signer trusted a different key: %v", err)
	}

	if _, err = VerifyPublishedManifest(
		release.SDKVersion(),
		contents,
		signature[:len(signature)-1],
	); !errors.Is(
		err,
		errPublishedSDKSignature,
	) {
		t.Fatalf("release signer accepted a truncated signature: %v", err)
	}
}
