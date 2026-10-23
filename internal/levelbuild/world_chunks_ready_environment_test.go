package levelbuild

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

func TestReadyEnvironmentGeometryKeepsImageDependenciesIndependent(t *testing.T) {
	t.Parallel()

	input := "version: 11\nenvironment: {preset: daylight}\n" + strings.Replace(
		lightingRoomYAML,
		"    floor:",
		"    sky: true\n    floor:",
		1,
	)
	root := lightingLevelFixture(t, input)
	enableFixtureLightmap(
		t,
		root,
		"enabled = true\noffline = true\ndensity = 1\npage_size = 512\nbake_samples = 1\nbake_bounces = 0\nbake_denoise = 'off'\n[lightmap.environment]\ntimes = [0,15]\ndefault_time = 15\nweather_variants = true\n",
	)
	enableFixtureChunks(t, root, "")
	selected := directLightmapSDK(t)
	selected.Assets.Capabilities.Runtime = append(selected.Assets.Capabilities.Runtime,
		asset.Capability(worldchunk.ReadyFeature), asset.CapabilityWorldLightmapsEnvironmentVariantsV1)

	built, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
	if err != nil {
		t.Fatal(err)
	}

	if len(built) != 1 || len(built[0].Chunks) != 5 || !slices.Contains(built[0].Features, worldchunk.ReadyFeature) {
		t.Fatal("ready geometry or image dependency missing")
	}

	parent := decodePassiveEnvelope(t, built[0].Bytes)
	worldBytes, _, _ := parent.Read(world.EntryName, 0, world.MaxEncodedSize)

	document, err := world.Decode(worldBytes)
	if err != nil {
		t.Fatal(err)
	}

	child := decodePassiveEnvelope(t, built[0].Chunks[0].Bytes)
	encodedCatalog, _, _ := parent.Read(worldchunk.CatalogEntryName, 0, worldchunk.MaxEncodedSize)

	catalog, err := worldchunk.DecodeCatalog(encodedCatalog, &document)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, found := child.Read(worldchunk.MeshEntryName, 0, worldchunk.MaxEncodedSize); found || len(child.Entries) != 4 {
		t.Fatal("environment chunk shipped duplicate semantic geometry")
	}

	layoutBytes, _, _ := child.Read(worldlightmap.EntryName, 0, worldlightmap.MaxEncodedSize)

	layout, err := worldlightmap.Decode(layoutBytes, &document)
	if err != nil {
		t.Fatal(err)
	}

	mesh, err := worldchunk.CompileMesh(document, catalog.Chunks[0].Sectors, catalog.Chunks[0].Solids, layout)
	if err != nil {
		t.Fatal(err)
	}

	meshBytes, err := worldchunk.EncodeMesh(mesh, &document)
	if err != nil {
		t.Fatal(err)
	}

	encoded, _, found := child.Read(worldchunk.ReadyEntryName, 0, worldchunk.MaxReadyBytes)
	if !found {
		t.Fatal("environment geometry lacks prepared streams")
	}

	if _, err := worldchunk.DecodeReady(encoded, &mesh, meshBytes, &document, &layout, worldchunk.ReadyOptions{}); err != nil {
		t.Fatal(err)
	}

	validateReadyEnvironmentImageUnit(t, built[0].Chunks, child, &document, layout)
}

func validateReadyEnvironmentImageUnit(t *testing.T, children []ChunkArtifact, child level.Envelope,
	document *world.Document, layout worldlightmap.Layout,
) {
	t.Helper()

	encoded, _, found := child.Read(worldlightmap.EnvironmentVariantsEntryName, 0, worldlightmap.MaxEnvironmentCatalogSize)
	if !found {
		t.Fatal("initial chunk lost its condition catalog")
	}

	catalog, err := worldlightmap.DecodeEnvironmentVariants(encoded, layout, document)
	if err != nil || catalog.Default != 2 || len(catalog.Variants) != 4 {
		t.Fatal("non-first default condition was not preserved", err)
	}
	// The required default page is usable from the geometry module alone;
	// none of the optional condition modules participates in initial readiness.
	initial, _, found := child.Read(worldlightmap.EnvironmentImageEntryName, 0, worldlightmap.MaxPrebakeImageSize)
	if !found || worldlightmap.ValidateEnvironmentImage(catalog.Variants[catalog.Default], initial, layout) != nil {
		t.Fatal("initial chunk depends on an external default page")
	}

	for index, variant := range catalog.Variants {
		artifact := children[index+1]

		digest := sha256.Sum256(artifact.Bytes)
		if hex.EncodeToString(digest[:]) != variant.ContentSHA256 || uint64(len(artifact.Bytes)) != variant.Size {
			t.Fatal("optional condition module identity changed")
		}

		image := decodePassiveEnvelope(t, artifact.Bytes)

		metadata, err := level.DecodeMetadata(image.Metadata)
		if err != nil || chunkMetadataPresent(metadata, worldchunk.ReadyMetadataKey) {
			t.Fatal("image-only dependency incorrectly declares render geometry", err)
		}

		if len(image.Entries) != 1 || image.Entries[0].Name != worldlightmap.EnvironmentImageEntryName {
			t.Fatal("image-only dependency gained unrelated streams")
		}

		pixels, _, found := image.Read(worldlightmap.EnvironmentImageEntryName, 0, worldlightmap.MaxPrebakeImageSize)
		if !found || worldlightmap.ValidateEnvironmentImage(variant, pixels, layout) != nil {
			t.Fatal("optional condition image was not complete")
		}

		if index == catalog.Default && !bytes.Equal(pixels, initial) {
			t.Fatal("embedded default and later default reload disagree")
		}
	}
}
