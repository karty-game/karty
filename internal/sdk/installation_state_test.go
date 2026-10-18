package sdk

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAbsentSDKIsDistinctFromDamagedInstallation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("KARTY_HOME", home)

	const version = "99.0.0"
	if _, err := ResolveContext(t.Context(), version); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("absent SDK = %v", err)
	}

	directory := filepath.Join(home, "sdks", version)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(directory, "sdk.zip"), []byte("broken archive"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := ResolveContext(t.Context(), version); err == nil || errors.Is(err, ErrNotInstalled) {
		t.Fatalf("missing checksum treated as absent SDK: %v", err)
	}

	if err := os.WriteFile(filepath.Join(directory, "sha256"), []byte(strings.Repeat("0", 64)), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := ResolveContext(t.Context(), version); err == nil || errors.Is(err, ErrNotInstalled) {
		t.Fatalf("corrupt cache treated as absent SDK: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := ResolveContext(ctx, version); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled resolve = %v", err)
	}
}
