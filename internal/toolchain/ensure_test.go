package toolchain_test

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/toolchain"
)

func TestEnsureRejectsMissingPlatformArtifactBeforeDownload(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve("0.0.1")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	manifest.Artifacts.TinyGo = nil

	_, err = toolchain.Ensure(context.Background(), manifest, toolchain.EnsureOptions{NeedTinyGo: true})
	if err == nil {
		t.Fatal("Ensure() error = nil, want missing platform artifact")
	}

	want := "TinyGo for " + runtime.GOOS + "-" + runtime.GOARCH
	if !strings.Contains(err.Error(), want) {
		t.Errorf("Ensure() error = %q, want %q", err, want)
	}
}
