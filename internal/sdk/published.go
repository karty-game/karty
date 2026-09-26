// Package sdk resolves versioned Karty SDK manifests.
package sdk

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/pelletier/go-toml/v2"
)

const (
	publishedSDKBaseURL = "https://github.com/karty-game/karty-sdk/releases/download/sdk-v"
	// publishedSDKPublicKey is the DER-encoded Ed25519 verifier for release SDK
	// manifests. The matching private key exists only as a GitHub Actions secret.
	publishedSDKPublicKey = "MCowBQYDK2VwAyEARR6tPyxEr03kZisMLdioCbhZWrx+Oryo42P8n6OHL/k="
)

var (
	errPublishedSDKDownload  = errors.New("download published SDK")
	errPublishedSDKSignature = errors.New("published SDK signature is invalid")
)

// ResolvePublished downloads and verifies the SDK manifest attached to the
// matching Karty release. Callers must use its exact project-pinned version.
func ResolvePublished(ctx context.Context, version string) (Manifest, error) {
	if !versionPattern.MatchString(version) {
		return Manifest{}, fmt.Errorf("invalid SDK version: %w", errIncompleteSDK)
	}

	manifestURL := publishedSDKBaseURL + version + "/sdk-" + version + ".toml"
	signatureURL := manifestURL + ".sig"

	manifest, err := downloadPublishedSDK(ctx, manifestURL)
	if err != nil {
		return Manifest{}, err
	}

	signature, err := downloadPublishedSDK(ctx, signatureURL)
	if err != nil {
		return Manifest{}, err
	}

	key, err := publishedVerifier()
	if err != nil {
		return Manifest{}, err
	}

	return parsePublishedManifest(version, manifest, signature, key)
}

func downloadPublishedSDK(ctx context.Context, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create published SDK request: %w", err)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errPublishedSDKDownload, err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %s", errPublishedSDKDownload, response.Status)
	}

	contents, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read published SDK: %w", err)
	}

	return contents, nil
}

func publishedVerifier() (ed25519.PublicKey, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(publishedSDKPublicKey)
	if err != nil {
		return nil, fmt.Errorf("decode published SDK verifier: %w", err)
	}

	parsed, err := x509.ParsePKIXPublicKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("parse published SDK verifier: %w", err)
	}

	key, ok := parsed.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("parse published SDK verifier: invalid key type: %w", errPublishedSDKSignature)
	}

	return key, nil
}

func parsePublishedManifest(version string, contents, signature []byte, key ed25519.PublicKey) (Manifest, error) {
	if !ed25519.Verify(key, contents, signature) {
		return Manifest{}, errPublishedSDKSignature
	}

	var manifest Manifest
	if err := toml.Unmarshal(contents, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse published SDK %s: %w", version, err)
	}

	if err := validateManifest(version, manifest); err != nil {
		return Manifest{}, fmt.Errorf("published %w", err)
	}

	return manifest, nil
}
