package levelbuild

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldgpuchunk"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty-sdk/format/worldmaterialgpu"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

// Synthetic maps here are fixtures only. Production has no replacement for a
// missing/unusable native Materialize executable or failed GPU generation.
func fixtureWorldMaterialPair(t *testing.T, ids []uint32) worldmaterial.Pair {
	t.Helper()

	layout, err := worldmaterial.NewLayout(ids)
	if err != nil {
		t.Fatal(err)
	}

	encode := func(pixel color.NRGBA, colorspace uint8) []byte {
		img := image.NewNRGBA(image.Rect(0, 0, layout.Width, layout.Height))
		for y := range layout.Height {
			for x := range layout.Width {
				img.SetNRGBA(x, y, pixel)
			}
		}

		encoded, _, err := qoi.Encode(img, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: colorspace})
		if err != nil {
			t.Fatal(err)
		}

		return encoded
	}

	return worldmaterial.Pair{Layout: layout,
		Albedo: encode(color.NRGBA{R: 192, A: 255}, qoi.ColorspaceSRGB),
		Data:   encode(color.NRGBA{R: 143, G: 102, B: 41, A: 87}, qoi.ColorspaceLinear),
	}
}

func TestPackageWorldMaterialLayoutAndPair(t *testing.T) {
	t.Parallel()

	document := world.Document{
		Sectors: []world.Sector{{FloorMaterial: 4, CeilingMaterial: 1, Walls: []world.Wall{{Material: 8}, {Material: 0}}}},
	}
	pair := fixtureWorldMaterialPair(t, []uint32{4, 1, 8, 0})
	metadata := binaryMetadata(t, level.Metadata{"custom": level.UintValue(9007199254740993), "name": level.StringValue("fixture")})

	entries, metadata, err := packageWorldMaterials(nil, metadata, document, pair)
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := level.Encode(metadata, entries)
	if err != nil {
		t.Fatal(err)
	}

	envelope, err := level.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	identity, err := level.DecodeMetadata(envelope.Metadata)
	if err != nil {
		t.Fatal(err)
	}

	custom, _ := identity.Uint("custom")

	marker, _ := identity.String(worldmaterial.MetadataKey)
	if custom != 9007199254740993 || marker != worldmaterial.Schema {
		t.Fatalf("metadata: %s", envelope.Metadata)
	}

	layoutBytes, _, found := envelope.Read(worldmaterial.LayoutEntry, 0, worldmaterial.MaxLayoutBytes)
	if !found {
		t.Fatal("missing layout")
	}

	layout, err := worldmaterial.DecodeLayout(layoutBytes)
	if err != nil || !reflect.DeepEqual(layout, pair.Layout) {
		t.Fatalf("layout: %v", err)
	}

	for _, test := range []struct {
		name string
		want []byte
	}{{worldmaterial.AlbedoEntry, pair.Albedo}, {worldmaterial.DataEntry, pair.Data}} {
		got, _, found := envelope.Read(test.name, 0, level.MaxEntrySize)
		if !found || !bytes.Equal(got, test.want) {
			t.Fatalf("missing/replaced %s", test.name)
		}

		for _, entry := range envelope.Entries {
			if entry.Name == test.name && entry.Kind != level.EntryData {
				t.Fatal("atlas pair must be data, not a level texture ID")
			}
		}
	}

	wrong := fixtureWorldMaterialPair(t, []uint32{1, 4, 8, 0})
	if _, _, err := packageWorldMaterials(nil, metadata, document, wrong); err == nil {
		t.Fatal("geometry/layout order mismatch accepted")
	}

	wrong = pair
	wrong.Data = nil

	if _, _, err := packageWorldMaterials(nil, metadata, document, wrong); err == nil {
		t.Fatal("partial pair accepted")
	}

	for _, name := range []string{worldmaterial.LayoutEntry, worldmaterial.AlbedoEntry, worldmaterial.DataEntry, worldmaterial.MipTailEntry} {
		entries := []level.SourceEntry{{Name: name, Kind: level.EntryData, Data: []byte("authored")}}
		if _, _, err := packageWorldMaterials(entries, metadata, document, pair); !errors.Is(err, ErrSource) {
			t.Fatalf("reserved material entry accepted: %s, %v", name, err)
		}
	}

	for _, metadata := range [][]byte{[]byte("null"), []byte("[]"), []byte("invalid")} {
		if _, _, err := packageWorldMaterials(nil, metadata, document, pair); !errors.Is(err, ErrManifest) {
			t.Fatalf("invalid metadata accepted: %v", err)
		}
	}
}

func TestAuthoredDataCannotClaimGeneratedWorldMaterials(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "authored.bin"), []byte("authored"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Reject reserved entries even without a world or an atlas-capable SDK:
	// otherwise the level could contain a partial, undeclared packaged pair.
	for _, name := range []string{worldmaterial.LayoutEntry, worldmaterial.AlbedoEntry, worldmaterial.DataEntry, worldmaterial.MipTailEntry} {
		_, err := loadData(directory, []dataEntry{{Name: name, Source: "authored.bin"}})
		if !errors.Is(err, ErrSource) {
			t.Fatalf("authored generated entry %q accepted: %v", name, err)
		}
	}

	entries, err := loadData(directory, []dataEntry{{Name: "@world/custom", Source: "authored.bin"}})
	if err != nil || len(entries) != 1 || string(entries[0].Data) != "authored" {
		t.Fatalf("unrelated authored data changed: %v", err)
	}
}

func TestWorldMaterialSolidsBuildAndCapabilityGate(t *testing.T) {
	t.Parallel()
	root := lightingLevelFixture(t, "version: 6\n"+lightingRoomYAML+levelSolidYAML)

	selected, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	selected = withoutRuntimeCapabilities(
		currentWorldSDK(selected),
		asset.CapabilityWorldMaterialMappingV1,
		asset.CapabilityWorldStaticSolidsV1,
	)
	calls := 0

	var generated worldmaterial.Pair

	assets := &assetBuild{
		projectRoot: root,
		manifest:    selected,
		materials: func(_ context.Context, _ string, _ sdk.Manifest, document world.Document, _ map[uint32][]byte, _ ...map[uint32]assetpipeline.MaterializeOptions) (worldmaterial.Pair, assetpipeline.Artifact, error) {
			calls++
			generated = fixtureWorldMaterialPair(t, assetpipeline.WorldMaterialIDs(document))

			return generated, assetpipeline.Artifact{CacheKey: "fixture", CacheHit: true}, nil
		},
	}
	if rejected, err := buildAll(t.Context(), root, 4, "", assets); !errors.Is(err, ErrManifest) || len(rejected) != 0 {
		t.Fatal("missing geometry capability published resources", err)
	}

	assets.manifest.Assets.Capabilities.Runtime = append(
		assets.manifest.Assets.Capabilities.Runtime,
		asset.CapabilityWorldMaterialMappingV1,
		asset.CapabilityWorldStaticSolidsV1,
	)
	calls = 0

	candidate, err := buildAll(t.Context(), root, 4, "", assets)
	if err != nil || len(candidate) != 1 || calls != 1 {
		t.Fatal("current prepared material build", calls, err)
	}

	if !slices.Contains(candidate[0].Features, worldgpuchunk.Feature) || !slices.Contains(candidate[0].Features, worldmaterialgpu.Feature) {
		t.Fatal("missing prepared material/cell contracts")
	}

	document := executePackagedLightingWorld(t, candidate[0].Bytes)
	if document.StaticSolids == nil || len(document.StaticSolids.Items) != 1 || !document.StaticSolids.Items[0].Collision {
		t.Fatal("solid geometry lost")
	}

	reports := 0

	for _, texture := range candidate[0].Textures {
		if texture.Kind == "world-material" {
			reports++

			if texture.SourceBytes != 0 || texture.EstimatedDecodedBytes <= 0 {
				t.Fatal("invalid prepared material report")
			}
		}
	}

	if reports != 3 {
		t.Fatal("prepared material array report count", reports)
	}

	checkWorldMaterialModule(t, candidate[0].Bytes, generated)

	sentinel := os.ErrPermission

	assets.materials = func(context.Context, string, sdk.Manifest, world.Document, map[uint32][]byte, ...map[uint32]assetpipeline.MaterializeOptions) (worldmaterial.Pair, assetpipeline.Artifact, error) {
		return worldmaterial.Pair{}, assetpipeline.Artifact{}, sentinel
	}
	if _, err = buildAll(t.Context(), root, 4, "", assets); !errors.Is(err, sentinel) {
		t.Fatal("source preparation did not fail closed", err)
	}
}

func checkWorldMaterialModule(t *testing.T, module []byte, generated worldmaterial.Pair) {
	t.Helper()

	// Execute the packaged passive WASM to retrieve its actual envelope. This
	// checks WASM level packaging, not the renderer or Materialize GPU.
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("passive WASM execution needs Node (provided by root mise tasks)")
	}

	modulePath := filepath.Join(t.TempDir(), "fixture.kld")
	if err := os.WriteFile(modulePath, module, 0o600); err != nil {
		t.Fatal(err)
	}

	const script = `import fs from 'node:fs';
const {instance} = await WebAssembly.instantiate(fs.readFileSync(process.argv[1]));
const e = instance.exports;
if (e.karty_level_abi_version() !== 2) throw new Error('level ABI');
process.stdout.write(Buffer.from(e.memory.buffer, e.karty_level_data_pointer(), e.karty_level_data_length()));`

	encoded, err := exec.CommandContext(t.Context(), node, "--input-type=module", "-e", script, modulePath).Output()
	if err != nil {
		t.Fatal(err)
	}

	envelope, err := level.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	layoutBytes, _, found := envelope.Read(worldmaterial.LayoutEntry, 0, worldmaterial.MaxLayoutBytes)
	if !found {
		t.Fatal("candidate has no packaged layout")
	}

	layout, err := worldmaterial.DecodeLayout(layoutBytes)
	if err != nil || !reflect.DeepEqual(layout, generated.Layout) {
		t.Fatalf("packaged layout: %v", err)
	}

	descriptorBytes, _, found := envelope.Read(worldmaterialgpu.DescriptorEntry, 0, level.MaxEntrySize)
	if !found {
		t.Fatal("missing prepared material descriptor")
	}

	descriptor, err := worldmaterialgpu.DecodeDescriptor(descriptorBytes)
	if err != nil {
		t.Fatal(err)
	}

	pair := worldmaterialgpu.Pair{Descriptor: descriptor}
	pair.Albedo, _, _ = envelope.Read(worldmaterialgpu.AlbedoEntry, 0, level.MaxEntrySize)
	pair.SSBump, _, _ = envelope.Read(worldmaterialgpu.SSBumpEntry, 0, level.MaxEntrySize)

	pair.Detail, _, _ = envelope.Read(worldmaterialgpu.DetailEntry, 0, level.MaxEntrySize)
	if err = pair.Validate(layout); err != nil {
		t.Fatal("prepared material arrays lost layout binding", err)
	}

	for _, name := range []string{worldmaterial.AlbedoEntry, worldmaterial.DataEntry, worldmaterial.MipTailEntry} {
		if _, _, found = envelope.Read(name, 0, level.MaxEntrySize); found {
			t.Fatal("root retained retired raw material payload", name)
		}
	}
}
