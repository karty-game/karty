package levelbuild

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"slices"
	"testing"

	"github.com/karty-game/karty-sdk/bake/reflectionplan"
	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/worldreflection"
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

func TestRequiredReflectionsRejectStructurallyValidPartialCapture(t *testing.T) {
	t.Parallel()

	document, _ := gpuHangarRoomTestSource(t)
	envelope := level.Envelope{}

	complete, err := reflectionplan.Plan(&document, worldreflection.InputDigest(envelope))
	if err != nil {
		t.Fatal(err)
	}

	strip := image.NewNRGBA(image.Rect(0, 0, worldreflection.FaceSize*worldreflection.Faces, worldreflection.FaceSize))
	for index := 3; index < len(strip.Pix); index += 4 {
		strip.Pix[index] = 255
	}

	encoded, _, err := qoi.Encode(strip, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceSRGB})
	if err != nil {
		t.Fatal(err)
	}

	for index := range complete.Probes {
		complete.Probes[index].Image = encoded
	}

	if err := validateRequiredReflectionCoverage(complete, &document); err != nil {
		t.Fatal("complete capture rejected", err)
	}

	partial := complete
	partial.Probes = slices.Clone(complete.Probes[:1])

	partial.Sectors, partial.Solids = slices.Clone(complete.Sectors), slices.Clone(complete.Solids)
	for _, owners := range [][]int{partial.Sectors, partial.Solids} {
		for index, id := range owners {
			if id != partial.Probes[0].ID {
				owners[index] = -1
			}
		}
	}

	if err := partial.ValidateSource(envelope, &document); err != nil {
		t.Fatal("fixture must demonstrate a structurally valid partial capture", err)
	}

	if err := validateRequiredReflectionCoverage(partial, &document); !errors.Is(err, ErrManifest) {
		t.Fatal("one of 26 environments accepted as complete", err)
	}

	moved := complete
	moved.Probes = slices.Clone(complete.Probes)

	moved.Probes[0].Origin[0] += 0.01
	if err := validateRequiredReflectionCoverage(moved, &document); !errors.Is(err, ErrManifest) {
		t.Fatal("changed capture placement accepted", err)
	}

	wrongOwners := complete
	wrongOwners.Sectors = slices.Clone(complete.Sectors)

	for index, id := range wrongOwners.Sectors {
		if id != -1 {
			wrongOwners.Sectors[index] = -1

			break
		}
	}

	if err := validateRequiredReflectionCoverage(wrongOwners, &document); !errors.Is(err, ErrManifest) {
		t.Fatal("missing reflective owner accepted", err)
	}
}
