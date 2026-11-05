package levelbuild

import (
	"bytes"
	"errors"
	"image"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

	markReflectionCaptureRequired(t, root, assets)

	assets.reflectionCapture = true
	capture := buildReflectionPlaceholder(t, root, assets)
	placeholderRoot := decodePassiveEnvelope(t, capture.Bytes)
	data, _, found := placeholderRoot.Read(worldreflection.EntryName, 0, level.MaxEntrySize)

	placeholder, err := worldreflection.DecodePrepared(data)
	if !found || err != nil || len(placeholder.Metadata.Probes) == 0 {
		t.Fatal("capture lacks complete prepared probe resources", err)
	}

	if worldreflection.InputDigest(base) != worldreflection.InputDigest(placeholderRoot) {
		t.Fatal("placeholder changed capture source digest")
	}

	assertReflectionCellsUnchanged(t, normal[0], capture)

	actual := placeholder.Metadata
	actual.Producer = worldreflection.Producer
	actual.Probes = slices.Clone(actual.Probes)
	actual.Sectors = slices.Clone(actual.Sectors)
	encodedCapture := encodeReflectionFixture(t, &actual)

	capturePath, err := SaveReflectionCapture(root, capture, encodedCapture)
	if err != nil {
		t.Fatal(err)
	}

	assets.reflectionCapture = false
	final := buildReflectionArtifact(t, root, assets)
	assertCapturedReflectionReplacesPlaceholder(t, placeholderRoot, capture, final, placeholder.Texture)
	assertInvalidRequiredReflectionsRejected(t, root, capture, capturePath, actual, assets)
}

func markReflectionCaptureRequired(t *testing.T, root string, assets *assetBuild) {
	t.Helper()

	project, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer project.Close()

	manifest, err := project.OpenFile(filepath.Join("levels", "lighting", "level.toml"), os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer manifest.Close()

	if _, err := manifest.WriteString("\n[reflections]\nrequired = true\n"); err != nil {
		t.Fatal(err)
	}

	if _, err := buildAll(
		t.Context(),
		root,
		4,
		"",
		assets,
	); !errors.Is(err, ErrManifest) ||
		!strings.Contains(err.Error(), "required reflection capture") {
		t.Fatal("required reflections disappeared from successful build", err)
	}

	// Capture preparation must create its fixed lighting from a clean checkout,
	// without depending on an optional CI bake-cache restore.
	if err := os.RemoveAll(filepath.Join(root, ".karty", "bakes")); err != nil {
		t.Fatal(err)
	}
}

func buildReflectionPlaceholder(t *testing.T, root string, assets *assetBuild) Artifact {
	t.Helper()

	capture, err := buildAll(t.Context(), root, 4, "", assets)
	if err != nil {
		t.Fatal(err)
	}

	return capture[0]
}

func encodeReflectionFixture(t *testing.T, actual *worldreflection.Bundle) []byte {
	t.Helper()

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

	data, err := worldreflection.Encode(*actual)
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func buildReflectionArtifact(t *testing.T, root string, assets *assetBuild) Artifact {
	t.Helper()

	final, err := buildAll(t.Context(), root, 4, "", assets)
	if err != nil {
		t.Fatal(err)
	}

	return final[0]
}

func assertCapturedReflectionReplacesPlaceholder(
	t *testing.T,
	placeholderRoot level.Envelope,
	capture, final Artifact,
	placeholderTexture []byte,
) {
	t.Helper()

	finalRoot := decodePassiveEnvelope(t, final.Bytes)
	data, _, found := finalRoot.Read(worldreflection.EntryName, 0, level.MaxEntrySize)

	prepared, err := worldreflection.DecodePrepared(data)
	if !found || err != nil || bytes.Equal(prepared.Texture, placeholderTexture) {
		t.Fatal("ordinary build did not replace placeholder with saved capture", err)
	}

	if worldreflection.InputDigest(finalRoot) != worldreflection.InputDigest(placeholderRoot) {
		t.Fatal("captured pixels changed source digest")
	}

	assertReflectionCellsUnchanged(t, capture, final)
}

func assertInvalidRequiredReflectionsRejected(
	t *testing.T,
	root string,
	capture Artifact,
	capturePath string,
	actual worldreflection.Bundle,
	assets *assetBuild,
) {
	t.Helper()

	empty := actual
	empty.Probes = nil

	empty.Sectors, empty.Solids = slices.Clone(actual.Sectors), slices.Clone(actual.Solids)
	for index := range empty.Sectors {
		empty.Sectors[index] = -1
	}

	for index := range empty.Solids {
		empty.Solids[index] = -1
	}

	emptyData, err := worldreflection.Encode(empty)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := SaveReflectionCapture(root, capture, emptyData); err != nil {
		t.Fatal(err)
	}

	if _, err := buildAll(
		t.Context(),
		root,
		4,
		"",
		assets,
	); !errors.Is(err, ErrManifest) ||
		!strings.Contains(err.Error(), "no environments") {
		t.Fatal("empty required capture silently omitted", err)
	}

	if err := os.WriteFile(capturePath, []byte("invalid capture"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := buildAll(
		t.Context(),
		root,
		4,
		"",
		assets,
	); !errors.Is(err, ErrManifest) ||
		!strings.Contains(err.Error(), "required reflection capture") {
		t.Fatal("invalid required capture silently omitted", err)
	}

	actual.Sectors[0] = -1

	bad, err := worldreflection.Encode(actual)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := SaveReflectionCapture(root, capture, bad); err != nil {
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
