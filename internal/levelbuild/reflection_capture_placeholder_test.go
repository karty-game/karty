package levelbuild

import (
	"bytes"
	"image"
	"os"
	"slices"
	"testing"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/worldreflection"
)

func TestGPUReflectionCapturePlaceholderPreservesSourceAndCellIdentity(t *testing.T) {
	encoder := os.Getenv("KARTY_TEST_KTX")
	if encoder == "" {
		t.Skip("KARTY_TEST_KTX not selected")
	}

	t.Setenv("KARTY_KTX", encoder)
	root, assets := gpuActualProfileFixture(t, "fixed")
	assets.manifest.Assets.Capabilities.Runtime = append(assets.manifest.Assets.Capabilities.Runtime,
		asset.Capability(worldreflection.Feature), asset.Capability(worldreflection.GPUFeature))

	normal, err := buildAll(t.Context(), root, 4, "", assets)
	if err != nil {
		t.Fatal(err)
	}

	base := decodePassiveEnvelope(t, normal[0].Bytes)
	if _, _, found := base.Read(worldreflection.EntryName, 0, level.MaxEntrySize); found {
		t.Fatal("ordinary build published capture-only placeholder")
	}

	assets.reflectionCapture = true

	capture, err := buildAll(t.Context(), root, 4, "", assets)
	if err != nil {
		t.Fatal(err)
	}

	placeholderRoot := decodePassiveEnvelope(t, capture[0].Bytes)
	data, _, found := placeholderRoot.Read(worldreflection.EntryName, 0, level.MaxEntrySize)

	placeholder, err := worldreflection.DecodePrepared(data)
	if !found || err != nil || len(placeholder.Metadata.Probes) == 0 {
		t.Fatal("capture lacks complete prepared probe resources", err)
	}

	if worldreflection.InputDigest(base) != worldreflection.InputDigest(placeholderRoot) {
		t.Fatal("placeholder changed capture source digest")
	}

	assertReflectionCellsUnchanged(t, normal[0], capture[0])

	actual := placeholder.Metadata
	actual.Producer = worldreflection.Producer
	actual.Probes = slices.Clone(actual.Probes)
	actual.Sectors = slices.Clone(actual.Sectors)

	pixels := image.NewNRGBA(image.Rect(0, 0, worldreflection.FaceSize*worldreflection.Faces, worldreflection.FaceSize))
	for offset := 0; offset < len(pixels.Pix); offset += 4 {
		copy(pixels.Pix[offset:offset+4], []byte{70, 120, 210, 255})
	}

	encoded, _, err := qoi.Encode(pixels, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceSRGB})
	if err != nil {
		t.Fatal(err)
	}

	for index := range actual.Probes {
		actual.Probes[index].Image = encoded
	}

	encodedCapture, err := worldreflection.Encode(actual)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := SaveReflectionCapture(root, capture[0], encodedCapture); err != nil {
		t.Fatal(err)
	}

	assets.reflectionCapture = false

	final, err := buildAll(t.Context(), root, 4, "", assets)
	if err != nil {
		t.Fatal(err)
	}

	finalRoot := decodePassiveEnvelope(t, final[0].Bytes)
	data, _, found = finalRoot.Read(worldreflection.EntryName, 0, level.MaxEntrySize)

	prepared, err := worldreflection.DecodePrepared(data)
	if !found || err != nil || bytes.Equal(prepared.Texture, placeholder.Texture) {
		t.Fatal("ordinary build did not replace placeholder with saved capture", err)
	}

	if worldreflection.InputDigest(finalRoot) != worldreflection.InputDigest(placeholderRoot) {
		t.Fatal("captured pixels changed source digest")
	}

	assertReflectionCellsUnchanged(t, capture[0], final[0])

	actual.Sectors[0] = -1

	bad, err := worldreflection.Encode(actual)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := SaveReflectionCapture(root, capture[0], bad); err != nil {
		t.Fatal(err)
	}

	if _, err := buildAll(t.Context(), root, 4, "", assets); err == nil {
		t.Fatal("capture owner bindings different from immutable record slots accepted")
	}
}

func assertReflectionCellsUnchanged(t *testing.T, before, after Artifact) {
	t.Helper()

	if len(before.Chunks) != len(after.Chunks) {
		t.Fatal("capture changed child count")
	}

	for index, child := range before.Chunks {
		if child.ContentSHA256 != after.Chunks[index].ContentSHA256 || !bytes.Equal(child.Bytes, after.Chunks[index].Bytes) {
			t.Fatal("capture rewrote child GPU records, geometry or RNM", index)
		}
	}
}
