package levelbuild_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	sdkActions "github.com/karty-game/karty-sdk/format/actions"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty/internal/levelbuild"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

func TestAuthoredActionsSDKCompatibility(t *testing.T) {
	t.Parallel()

	for _, supported := range []bool{false, true} {
		name := "without-contract"
		if supported {
			name = "with-contract"
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeLevel(t, root, "scripted", "levels.scripted", "map")

			actions := []byte("version: 1\nsequences:\n  - name: welcome\n    steps:\n      - waitFrames: 1\n")
			if err := os.WriteFile(filepath.Join(root, "levels", "scripted", "actions.yaml"), actions, 0o600); err != nil {
				t.Fatal(err)
			}

			selected, err := sdk.Resolve(release.SDKVersion())
			if err != nil {
				t.Fatal(err)
			}

			selected.API.Version = "future-api"

			if !supported {
				selected.Compatibility.ProjectCodegen = 1
			}

			artifacts, err := levelbuild.BuildAllWithAssets(t.Context(), root, 2, "", selected)
			if !supported {
				if !errors.Is(err, levelbuild.ErrManifest) {
					t.Fatalf("SDK without authored action contract accepted: %v", err)
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			assertPackagedActions(t, artifacts)
		})
	}
}

func assertPackagedActions(t *testing.T, artifacts []levelbuild.Artifact) {
	t.Helper()

	if len(artifacts) != 1 {
		t.Fatalf("expected one scripted level, got %d", len(artifacts))
	}

	decoded, err := level.Decode(unwrapLevelModule(t, artifacts[0].Bytes))
	if err != nil {
		t.Fatal(err)
	}

	packaged, _, found := decoded.Read("karty/actions@1", 0, level.MaxEntrySize)

	document, decodeErr := sdkActions.Decode(packaged)
	if !found || decodeErr != nil || len(document.Sequences) != 1 || document.Sequences[0].Name != "welcome" ||
		document.Sequences[0].Steps[0].WaitFrames != 1 {
		t.Fatal("authored actions did not become bounded binary runtime data")
	}
}
