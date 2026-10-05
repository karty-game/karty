package sdk

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty/internal/release"
)

func currentTestBundle(t *testing.T) ([]byte, error) {
	t.Helper()

	cache, err := sdkCache()
	if err != nil {
		return nil, err
	}

	data, err := readBundleFile(filepath.Join(cache, release.SDKVersion(), "sdk.zip"))
	if err != nil {
		return nil, err
	}

	hash, err := os.ReadFile(filepath.Join(cache, release.SDKVersion(), "sha256"))
	if err != nil {
		return nil, err
	}

	if err := verifyBundleHash(data, string(hash)); err != nil {
		return nil, err
	}

	return data, nil
}
