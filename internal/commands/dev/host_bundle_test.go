package dev

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/toolchain"
)

func TestDevelopmentAcceptsVerifiedWebGPUBundle(t *testing.T) {
	t.Parallel()
	directory := developmentBundleFixture(t)

	for _, selected := range []string{directory, filepath.Join(directory, "karty-host.wasm")} {
		if host, err := devHost(t.Context(), selected); err != nil || host != selected {
			t.Fatalf("verified local host rejected: %q, %v", host, err)
		}
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := devHost(ctx, directory); !errors.Is(err, context.Canceled) {
		t.Fatal("host validation discarded development cancellation", err)
	}

	if err := os.WriteFile(filepath.Join(directory, "bootstrap.mjs"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := devHost(t.Context(), directory); err == nil {
		t.Fatal("changed bundle accepted")
	}
}

func developmentBundleFixture(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()

	var descriptor strings.Builder
	descriptor.WriteString("schema=1\nrenderer='webgpu'\ntarget='web'\nentrypoint='karty-host.wasm'\n")

	for _, name := range append(append(toolchain.WebGPUHostFiles(), toolchain.WebGPUTextureCodecFiles()...), toolchain.WebGPUTextureCodecLicenses()...) {
		data := []byte("fixture")
		if name == "karty-host.wasm" || name == "runtime.bin" {
			data = []byte("\x00asm\x01\x00\x00\x00")
		}

		digest := sha256.Sum256(data)
		fmt.Fprintf(&descriptor, "\n[files.%q]\nsize=%d\nsha256=%q\n", name, len(data), hex.EncodeToString(digest[:]))

		if err := os.MkdirAll(filepath.Dir(filepath.Join(directory, name)), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(directory, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(directory, "host-bundle.toml"), []byte(descriptor.String()), 0o600); err != nil {
		t.Fatal(err)
	}

	return directory
}
