package levelbuild

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/karty-game/karty-sdk/bake/reflectionplan"
)

// The golden comes from the verified, saved 26-probe Hangar capture. Only the
// authored source is read here; private bake caches and engine files are not.
func TestHangarSharedReflectionPlanPreservesSavedCaptureMetadata(t *testing.T) {
	t.Parallel()
	document, _ := gpuHangarRoomTestSource(t)

	const input = "d0fde4fd00fc05c4e40281524a88d7304f8565f0923f88dab5b57c6094912351"

	bundle, err := reflectionplan.Plan(&document, input)
	if err != nil {
		t.Fatal(err)
	}

	if len(bundle.Probes) != 26 || len(bundle.Sectors) != 133 || len(bundle.Solids) != 413 {
		t.Fatal("saved capture metadata cardinality changed")
	}

	//nolint:musttag // The golden covers every binary-only SDK field and its default JSON name/order.
	data, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}

	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != "8c69856320ab9e33bfc3a19fc9534cc1877529fe7f3b30b92a3edb493c43bda5" {
		t.Fatal("shared planner changed saved probe IDs, placement, bounds or sector/solid bindings")
	}
}
