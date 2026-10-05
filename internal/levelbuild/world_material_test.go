package levelbuild

import (
	"bytes"
	"context"
	"encoding/json"
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
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/testfixture"
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
	metadata := []byte(`{"custom":9007199254740993,"name":"fixture"}`)

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

	var identity map[string]json.RawMessage

	if err := json.Unmarshal(envelope.Metadata, &identity); err != nil {
		t.Fatal(err)
	}

	if string(identity["custom"]) != "9007199254740993" || string(identity[worldmaterial.MetadataKey]) != `"`+worldmaterial.Schema+`"` {
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

func TestWorldMaterialCandidateSolidsBuildAndReleasedSDKRejects(t *testing.T) {
	t.Parallel()

	root := testfixture.WorldCameraGeometry(t, filepath.Join("..", "..", "samples", "world-camera"))

	manifest, err := sdk.Resolve("0.0.7")
	if err != nil {
		t.Fatal(err)
	}

	if manifest.Version != "0.0.7" || slices.Contains(manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialAtlasV1) {
		t.Fatal("released default gained atlas capability")
	}

	calls := 0

	var generated worldmaterial.Pair

	process := func(_ context.Context, _ string, _ sdk.Manifest, document world.Document, textures map[uint32][]byte) (worldmaterial.Pair, assetpipeline.Artifact, error) {
		calls++

		for _, id := range assetpipeline.WorldMaterialIDs(document) {
			if id != 0 {
				if _, err := qoi.Inspect(textures[id]); err != nil {
					return worldmaterial.Pair{}, assetpipeline.Artifact{}, err
				}
			}
		}

		generated = fixtureWorldMaterialPair(t, assetpipeline.WorldMaterialIDs(document))

		return generated, assetpipeline.Artifact{CacheKey: "fixture", CacheHit: true}, nil
	}
	assets := &assetBuild{projectRoot: root, manifest: manifest, materials: process}

	rejected, err := buildAll(context.Background(), root, 4, "", assets)
	if !errors.Is(err, ErrManifest) || len(rejected) != 0 || calls != 0 {
		t.Fatalf("released SDK accepted migrated candidate sample: %v", err)
	}

	assets.manifest.Version = "0.0.8"
	assets.manifest.Assets.Capabilities.Runtime = append(assets.manifest.Assets.Capabilities.Runtime,
		asset.CapabilityWorldMaterialMappingV1, asset.CapabilityWorldStaticSolidsV1)
	// The same migrated level remains buildable without atlas generation when the
	// candidate supports its mapping/solids capabilities.
	legacy, err := buildAll(context.Background(), root, 4, "", assets)
	if err != nil || len(legacy) != 1 || calls != 0 || slices.Contains(legacy[0].Features, worldmaterial.Feature) {
		t.Fatalf("candidate geometry-only build: %v", err)
	}

	assets.manifest.Assets.Capabilities.Runtime = append(assets.manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialAtlasV1)

	candidate, err := buildAll(context.Background(), root, 4, "", assets)
	if err != nil || len(candidate) != 1 || calls != 1 {
		t.Fatalf("candidate build: calls=%d err=%v", calls, err)
	}

	if !slices.Contains(candidate[0].Features, worldmaterial.Feature) || !slices.Contains(candidate[0].Features, world.Feature) {
		t.Fatalf("missing required features: %v", candidate[0].Features)
	}

	if len(candidate[0].Textures) != len(legacy[0].Textures)+2 {
		t.Fatal("generated pair missing from asset report/budget")
	}

	for _, texture := range candidate[0].Textures[len(legacy[0].Textures):] {
		if texture.Kind != "world-material" || texture.SourceBytes != 0 ||
			texture.EstimatedDecodedBytes != int64(generated.Layout.Width)*int64(generated.Layout.Height)*4 || !texture.CacheHit {
			t.Fatalf("atlas report: %+v", texture)
		}
	}

	checkWorldMaterialModule(t, candidate[0].Bytes, generated)

	sentinel := os.ErrPermission
	assets.materials = func(context.Context, string, sdk.Manifest, world.Document, map[uint32][]byte) (worldmaterial.Pair, assetpipeline.Artifact, error) {
		return worldmaterial.Pair{}, assetpipeline.Artifact{}, sentinel
	}

	if _, err := buildAll(context.Background(), root, 4, "", assets); !errors.Is(err, sentinel) {
		t.Fatalf("build did not fail closed: %v", err)
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
if (e.karty_level_abi_version() !== 1) throw new Error('level ABI');
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

	images := []struct {
		name string
		data []byte
	}{{worldmaterial.AlbedoEntry, generated.Albedo}, {worldmaterial.DataEntry, generated.Data}}
	if len(generated.MipTail) != 0 {
		images = append(images, struct {
			name string
			data []byte
		}{worldmaterial.MipTailEntry, generated.MipTail})
	}

	for _, test := range images {
		data, _, found := envelope.Read(test.name, 0, level.MaxEntrySize)
		if !found || !bytes.Equal(data, test.data) {
			t.Fatalf("packaged pair: %s", test.name)
		}
	}
}
