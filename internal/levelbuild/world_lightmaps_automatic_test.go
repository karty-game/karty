package levelbuild

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

const automaticBakeSettings = "enabled = true\noffline = true\npage_size = 512\ndensity = 1\nbake_samples = 1\nbake_bounces = 0\nbake_denoise = 'off'\n"

func automaticLightingYAML(count int, dynamic bool) (string, []string) {
	var source strings.Builder
	source.WriteString("lighting:\n  version: 1\n  ambient: {x: 0.1, y: 0.1, z: 0.1}\n  lights:\n")

	ids := make([]string, count)
	for index := range count {
		ids[index] = fmt.Sprintf("steady-%d", index)
		fmt.Fprintf(&source, "    - {id: %s, position: {x: 1, y: 1, z: 2}, color: {x: 0.001, y: 0.002, z: 0.001}, radius: 6}\n", ids[index])
	}

	if dynamic {
		source.WriteString(
			"    - {id: moving, position: {x: 1, y: 1, z: 2}, color: {x: 1, y: 0, z: 0}, radius: 6, motion: {version: 1, offset: {x: 0.1}, period_seconds: 2}}\n",
		)
		source.WriteString("    - {id: pulsing, position: {x: 1, y: 1, z: 2}, color: {x: 0, y: 1, z: 0}, radius: 6}\n")
	}

	return source.String(), ids
}

func addAutomaticPulse(t *testing.T, root string) {
	t.Helper()

	path := filepath.Join(root, "levels", "lighting", "level.toml")

	definition, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	writePrebakeFixture(
		t,
		path,
		append(
			definition,
			[]byte(
				"\n[textures.emission]\nintensity = 0.5\nlights = ['pulsing']\n[textures.emission.pulse]\ndepth = 0.3\nperiod_seconds = 3\n",
			)...),
	)
}

func automaticLayout(t *testing.T, envelope level.Envelope, document *world.Document) worldlightmap.Layout {
	t.Helper()

	encoded, _, found := envelope.Read(worldlightmap.EntryName, 0, worldlightmap.MaxEncodedSize)
	if !found {
		t.Fatal("missing automatic directional recipe")
	}

	layout, err := worldlightmap.Decode(encoded, document)
	if err != nil {
		t.Fatal(err)
	}

	return layout
}

func TestOfflineChunkBakeAutomaticallyIncludesSteadyLightsAndInvalidatesCache(t *testing.T) {
	t.Parallel()

	lighting, wantIDs := automaticLightingYAML(24, true)
	root := lightingLevelFixture(t, "version: 9\n"+chunkRoomsYAML+lighting)
	addAutomaticPulse(t, root)
	enableFixtureLightmap(t, root, automaticBakeSettings)
	enableFixtureChunks(t, root, "max_sectors = 1\n")
	selected := directLightmapSDK(t)
	selected.Assets.Capabilities.Runtime = append(selected.Assets.Capabilities.Runtime, asset.Capability(worldchunk.Feature))
	build := func(development bool) Artifact {
		t.Helper()

		artifacts, err := BuildAllWithOptions(t.Context(), root, 4, "", selected, Options{Development: development})
		if err != nil || len(artifacts) != 1 {
			t.Fatalf("automatic chunk build: %v", err)
		}

		return artifacts[0]
	}

	first := build(false)
	if len(first.Chunks) != 2 {
		t.Fatalf("chunks=%d", len(first.Chunks))
	}

	envelope := executeLightmapEnvelope(t, first.Bytes)
	worldBytes, _, _ := envelope.Read(world.EntryName, 0, world.MaxEncodedSize)

	document, err := world.Decode(worldBytes)
	if err != nil || document.Lighting == nil || len(document.Lighting.Lights) != 26 {
		t.Fatalf("authored source data changed: %v", err)
	}

	catalogBytes, _, _ := envelope.Read(worldchunk.CatalogEntryName, 0, worldchunk.MaxEncodedSize)

	catalog, err := worldchunk.DecodeCatalog(catalogBytes, &document)
	if err != nil || len(catalog.BakedLights) != len(wantIDs) {
		t.Fatalf("complete catalog bake classification: %+v, %v", catalog.BakedLights, err)
	}

	for index, slot := range catalog.BakedLights {
		if slot != index {
			t.Fatalf("baked authored order at %d=%d", index, slot)
		}
	}

	for _, chunk := range first.Chunks {
		child := executeLightmapEnvelope(t, chunk.Bytes)

		layout := automaticLayout(t, child, &document)
		if layout.RuntimeBake == nil || layout.RuntimeBake.Encoding != worldlightmap.DirectRNMEncoding ||
			!slices.Equal(layout.RuntimeBake.LightIDs, wantIDs) {
			t.Fatalf("automatic sources=%+v", layout.RuntimeBake)
		}

		manifest, _, _ := child.Read(worldlightmap.PrebakeEntryName, 0, worldlightmap.MaxPrebakeManifestSize)

		image, _, _ := child.Read(worldlightmap.PrebakeImageEntryName, 0, worldlightmap.MaxPrebakeImageSize)
		if _, err := worldlightmap.DecodePrebake(manifest, image, layout, &document); err != nil {
			t.Fatal(err)
		}
	}

	warm := build(true)
	if !bytes.Equal(warm.Bytes, first.Bytes) || !bytes.Equal(warm.Chunks[0].Bytes, first.Chunks[0].Bytes) {
		t.Fatal("development did not reuse complete automatic bake")
	}

	path := filepath.Join(root, "levels", "lighting", "world.yaml")

	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Edit the final automatically selected source, beyond the former eight-light
	// allowlist. Every receiver's transport and immutable digest must change.
	changed := strings.Replace(
		string(original),
		"id: steady-23, position: {x: 1, y: 1, z: 2}",
		"id: steady-23, position: {x: 1.5, y: 1, z: 2}",
		1,
	)
	if changed == string(original) {
		t.Fatal("fixture source edit did not match")
	}

	writePrebakeFixture(t, path, []byte(changed))

	cold := build(true)
	if slices.Contains(cold.Features, worldlightmap.PrebakeFeature) {
		t.Fatal("development reused stale automatically selected light")
	}

	rebuilt := build(false)
	for index := range first.Chunks {
		if bytes.Equal(first.Chunks[index].Bytes, rebuilt.Chunks[index].Bytes) {
			t.Fatalf("chunk %d ignored edited static source", index)
		}
	}

	writePrebakeFixture(t, path, original)

	if restored := build(true); !bytes.Equal(restored.Bytes, first.Bytes) {
		t.Fatal("matching immutable automatic bake was lost")
	}
}

func TestOfflineBakePackagesThousandSourcesAndRejectsMissingLiveFallback(t *testing.T) {
	t.Parallel()

	lighting, ids := automaticLightingYAML(1000, false)
	root := lightingLevelFixture(t, "version: 9\n"+lightingRoomYAML+lighting)
	enableFixtureLightmap(t, root, automaticBakeSettings)

	selected := directLightmapSDK(t)
	if artifacts, err := BuildAllWithAssets(t.Context(), root, 4, "", selected); !errors.Is(err, ErrManifest) || artifacts != nil {
		t.Fatalf("unbaked 1000 sources were silently admitted: %v", err)
	}

	reports, err := BakeAll(t.Context(), root, BakeOptions{Workers: 1}, nil)
	if err != nil || len(reports) != 1 {
		t.Fatalf("1000-source offline bake: %v", err)
	}

	artifacts, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
	if err != nil || len(artifacts) != 1 || !slices.Contains(artifacts[0].Features, worldlightmap.PrebakeFeature) {
		t.Fatalf("1000-source completed bake not packaged: %v", err)
	}

	envelope := executeLightmapEnvelope(t, artifacts[0].Bytes)
	worldBytes, _, _ := envelope.Read(world.EntryName, 0, world.MaxEncodedSize)

	document, err := world.Decode(worldBytes)
	if err != nil {
		t.Fatal(err)
	}

	layout := automaticLayout(t, envelope, &document)
	if !slices.Equal(layout.RuntimeBake.LightIDs, ids) || len(layout.Pages) != 1 {
		t.Fatal("large source set changed recipe or atlas page count")
	}

	writePrebakeFixture(t, reports[0].Image, []byte("corrupt generated QOI"))

	if artifacts, err := BuildAllWithOptions(
		t.Context(),
		root,
		4,
		"",
		selected,
		Options{Development: true},
	); !errors.Is(err, ErrManifest) ||
		artifacts != nil {
		t.Fatalf("missing large prebake silently dropped sources in development: %v", err)
	}
}

func TestOfflineBakeAutomaticEmptyStaticSetKeepsDynamicSources(t *testing.T) {
	t.Parallel()

	lighting, _ := automaticLightingYAML(0, true)
	root := lightingLevelFixture(t, "version: 9\n"+lightingRoomYAML+lighting)
	addAutomaticPulse(t, root)
	enableFixtureLightmap(t, root, automaticBakeSettings)

	if _, err := BakeAll(t.Context(), root, BakeOptions{Workers: 1}, nil); err != nil {
		t.Fatal(err)
	}

	artifacts, err := BuildAllWithAssets(t.Context(), root, 4, "", directLightmapSDK(t))
	if err != nil {
		t.Fatal(err)
	}

	envelope := executeLightmapEnvelope(t, artifacts[0].Bytes)
	worldBytes, _, _ := envelope.Read(world.EntryName, 0, world.MaxEncodedSize)

	document, err := world.Decode(worldBytes)
	if err != nil {
		t.Fatal(err)
	}

	layout := automaticLayout(t, envelope, &document)
	if len(document.Lighting.Lights) != 2 || layout.RuntimeBake == nil || layout.RuntimeBake.Encoding != worldlightmap.DirectRNMEncoding ||
		len(layout.RuntimeBake.LightIDs) != 0 {
		t.Fatalf("empty static set changed dynamic sources or recipe: %+v", layout.RuntimeBake)
	}
}
