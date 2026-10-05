package levelbuild

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"math"
	"os"
	"reflect"
	"strconv"
	"testing"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/pelletier/go-toml/v2"
)

func TestMaterialStrengthTOMLOmissionBoundsAndCapability(t *testing.T) {
	t.Parallel()

	var definition manifest
	if err := toml.Unmarshal([]byte(`
[world]
source="world.yaml"
[[textures]]
name="wall"
source="wall.png"
[textures.material_strengths]
normal=0
height=4
`), &definition); err != nil {
		t.Fatal(err)
	}

	got, err := definition.Textures[0].MaterialStrengths.resolved()
	if err != nil || got != (worldmaterial.Strengths{Normal: 0, Height: 4, AO: 1, Rim: 1}) {
		t.Fatalf("explicit zero or omitted default lost: %+v %v", got, err)
	}

	assets := &assetBuild{}

	assets.manifest.Assets.Capabilities.Runtime = []asset.Capability{asset.CapabilityWorldMaterialAtlasV1}
	if err := validateMaterialStrengths(definition, assets); err != nil {
		t.Fatal(err)
	}

	for _, unsupported := range []*assetBuild{nil, {}} {
		if err := validateMaterialStrengths(definition, unsupported); !errors.Is(err, ErrManifest) {
			t.Fatal("unsupported SDK silently ignored material controls")
		}
	}

	withoutWorld := definition

	withoutWorld.World.Source = ""
	if err := validateMaterialStrengths(withoutWorld, assets); !errors.Is(err, ErrManifest) {
		t.Fatal("worldless strengths silently ignored")
	}

	for _, invalid := range []float64{-0.001, 4.001, math.NaN(), math.Inf(1), math.Inf(-1)} {
		definition.Textures[0].MaterialStrengths.Rim = &invalid
		if err := validateMaterialStrengths(definition, assets); !errors.Is(err, ErrManifest) {
			t.Fatalf("invalid final control accepted: %v", invalid)
		}
	}

	definition.Textures[0].MaterialStrengths = nil
	if err := validateMaterialStrengths(definition, nil); err != nil {
		t.Fatal("omitted controls changed released SDK behavior")
	}
}

func TestMaterialStrengthIDMappingAndOwnership(t *testing.T) {
	t.Parallel()

	zero := float64(0)
	entries := []textureEntry{{Name: "wall", MaterialStrengths: &materialStrengths{Normal: &zero}}}
	document := world.Document{Sectors: []world.Sector{{FloorMaterial: 9, CeilingMaterial: 3}}}

	resolved, err := resolveMaterialStrengths(entries, []metadataTexture{{Name: "wall", ID: 9}}, document)
	if err != nil || resolved[9] == nil || *resolved[9] != (worldmaterial.Strengths{Height: 1, AO: 1, Rim: 1}) {
		t.Fatalf("material source-name mapping: %+v %v", resolved, err)
	}

	zero = 4

	if resolved[9].Normal != 0 {
		t.Fatal("compiled controls alias author configuration")
	}

	if _, err := resolveMaterialStrengths(entries, []metadataTexture{{Name: "wall", ID: 7}}, document); !errors.Is(err, ErrManifest) {
		t.Fatal("unused surface controls silently ignored")
	}

	if _, err := resolveMaterialStrengths(entries, nil, document); !errors.Is(err, ErrManifest) {
		t.Fatal("missing source texture controls silently ignored")
	}
}

func fixtureWorldMaterialMipPair(t *testing.T, ids []uint32) worldmaterial.Pair {
	t.Helper()

	pair := fixtureWorldMaterialPair(t, ids)

	layout, err := worldmaterial.NewMipLayout(pair.Layout)
	if err != nil {
		t.Fatal(err)
	}

	pair.Layout = layout
	width, height := layout.MipDimensions()

	pixels := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			pixels.SetNRGBA(x, y, color.NRGBA{R: 128, G: 128, B: 73, A: 255})
		}
	}

	pair.MipTail, _, err = qoi.Encode(pixels, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceLinear})
	if err != nil {
		t.Fatal(err)
	}

	return pair
}

func TestMaterialMipAndStrengthPackagedWASMAndReports(t *testing.T) {
	t.Parallel()

	root := lightingLevelFixture(t, "version: 1\n"+lightingRoomYAML)

	projectRoot, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer projectRoot.Close()

	definition, err := projectRoot.ReadFile("levels/lighting/level.toml")
	if err != nil {
		t.Fatal(err)
	}

	strengths := []byte("\n[textures.material_strengths]\nnormal=0\nheight=2\nao=0.75\nrim=4\n")
	if err := projectRoot.WriteFile("levels/lighting/level.toml", append(bytes.Clone(definition), strengths...), 0o600); err != nil {
		t.Fatal(err)
	}

	selected, err := sdk.Resolve("0.0.7")
	if err != nil {
		t.Fatal(err)
	}

	calls := 0

	var cachedPair worldmaterial.Pair

	assets := &assetBuild{
		projectRoot: root,
		manifest:    selected,
		materials: func(_ context.Context, _ string, _ sdk.Manifest, document world.Document, _ map[uint32][]byte) (worldmaterial.Pair, assetpipeline.Artifact, error) {
			calls++
			cachedPair = fixtureWorldMaterialMipPair(t, assetpipeline.WorldMaterialIDs(document))

			return cachedPair, assetpipeline.Artifact{CacheKey: "unchanged-image-recipe", CacheHit: true}, nil
		},
	}
	if _, err := buildAll(t.Context(), root, 4, "", assets); !errors.Is(err, ErrManifest) || calls != 0 {
		t.Fatal("unsupported controls reached generated-image processing")
	}

	assets.manifest.Version = "0.0.8"
	assets.manifest.Assets.Capabilities.Runtime = append(assets.manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialAtlasV1)

	first, err := buildAll(t.Context(), root, 4, "", assets)
	if err != nil || len(first) != 1 || calls != 1 {
		t.Fatalf("candidate controls+tail build: %v", err)
	}

	if cachedPair.Layout.Materials[0].Strengths != nil {
		t.Fatal("packaging mutated cached generated metadata")
	}

	expected := cachedPair
	expected.Layout, _ = worldmaterial.NewMipLayout(cachedPair.Layout)
	expected.Layout.Materials[0].Strengths = &worldmaterial.Strengths{Normal: 0, Height: 2, AO: 0.75, Rim: 4}
	checkWorldMaterialModule(t, first[0].Bytes, expected)

	reports := first[0].Textures
	if len(reports) != 4 {
		t.Fatalf("source + L0 pair + tail report count: %d", len(reports))
	}

	tailWidth, tailHeight := cachedPair.Layout.MipDimensions()

	last := reports[len(reports)-1]
	if last.Name != worldmaterial.MipTailEntry || last.Width != tailWidth || last.Height != tailHeight ||
		last.EstimatedDecodedBytes != int64(tailWidth)*int64(tailHeight)*4 ||
		!last.CacheHit {
		t.Fatalf("tail padded allocation/report lost: %+v", last)
	}

	strengths = bytes.Replace(strengths, []byte("height=2"), []byte("height=0.5"), 1)
	if err := projectRoot.WriteFile("levels/lighting/level.toml", append(bytes.Clone(definition), strengths...), 0o600); err != nil {
		t.Fatal(err)
	}

	second, err := buildAll(t.Context(), root, 4, "", assets)
	if err != nil || len(second) != 1 || bytes.Equal(first[0].Bytes, second[0].Bytes) {
		t.Fatal("changed strengths not recorded in level metadata")
	}

	if !reflect.DeepEqual(first[0].Textures[1:], second[0].Textures[1:]) {
		t.Fatal("changed controls altered generated texture hashes/recipes/reports")
	}

	broken := bytes.Replace(strengths, []byte("rim=4"), []byte("rim=4.001"), 1)
	if err := projectRoot.WriteFile("levels/lighting/level.toml", append(bytes.Clone(definition), broken...), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := buildAll(t.Context(), root, 4, "", assets); !errors.Is(err, ErrManifest) || calls != 2 {
		t.Fatal("invalid final control reached generation or partial publication")
	}
}

func TestMaterialStrengthEncodedLayoutBudgetBeforeImageGeneration(t *testing.T) {
	t.Parallel()

	precision := 1.123456789012345
	entries := make([]textureEntry, worldmaterial.MaxMaterials-1)
	metadata := make([]metadataTexture, len(entries))
	document := world.Document{Sectors: []world.Sector{{}}}

	for index := range entries {
		materialID := uint32(index + 1)
		name := strconv.Itoa(index)
		entries[index] = textureEntry{
			Name:              name,
			MaterialStrengths: &materialStrengths{Normal: &precision, Height: &precision, AO: &precision, Rim: &precision},
		}
		metadata[index] = metadataTexture{ID: materialID, Name: name}
		document.Sectors[0].Walls = append(document.Sectors[0].Walls, world.Wall{Material: materialID})
	}

	if _, err := resolveMaterialStrengths(entries, metadata, document); !errors.Is(err, ErrManifest) {
		t.Fatal("oversize complete control/layout metadata accepted before generation")
	}
}
