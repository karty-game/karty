package sdk_test

import (
	"testing"

	"github.com/karty-game/karty/internal/sdk"
)

func TestResolve(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve("0.0.1")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if manifest.Templates.Game != "0.0.1" {
		t.Errorf("game template = %q, want %q", manifest.Templates.Game, "0.0.1")
	}

	for _, profile := range []string{"sprite", "interface", "environment"} {
		if processor := manifest.Assets.TextureProfiles[profile].Processor; processor != "copy-png@1" {
			t.Errorf("texture profile %q processor = %q, want copy-png@1", profile, processor)
		}
	}
}

func TestResolveControlTransitionSDK(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve("0.0.1")
	if err != nil {
		t.Fatal(err)
	}

	if manifest.API.Version != "0.0.1" || manifest.Host.Version != "0.0.1" {
		t.Fatalf("transition SDK resolved incompatible versions: %+v", manifest)
	}
}

func TestResolveTextAlignmentSDK(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve("0.0.1")
	if err != nil {
		t.Fatal(err)
	}

	if manifest.API.Version != "0.0.1" || manifest.Host.Version != "0.0.1" {
		t.Fatalf("visual-hierarchy SDK resolved incompatible versions: %+v", manifest)
	}
}

func TestResolveInteractionPolishSDK(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve("0.0.1")
	if err != nil {
		t.Fatal(err)
	}

	if manifest.API.Version != "0.0.1" || manifest.Host.Version != "0.0.1" || manifest.Docs.Version != "0.0.1" {
		t.Fatalf("interaction-polish SDK resolved incompatible versions: %+v", manifest)
	}
}

func TestResolveRejectsUnknownVersion(t *testing.T) {
	t.Parallel()

	if _, err := sdk.Resolve("9.9.9"); err == nil {
		t.Fatal("Resolve() error = nil, want an unknown SDK error")
	}
}
