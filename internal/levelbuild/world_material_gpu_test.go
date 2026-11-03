package levelbuild

import (
	"context"
	"image"
	"os"
	"slices"
	"testing"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty-sdk/format/worldmaterialgpu"
	"github.com/karty-game/karty-sdk/format/worldmaterialssbump"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/sdk"
)

func TestWorldMaterialGPUActualBuildReplacesRuntimeQOI(t *testing.T) {
	encoder := os.Getenv("KARTY_TEST_KTX")
	if encoder == "" {
		t.Skip("KARTY_TEST_KTX not selected")
	}

	t.Setenv("KARTY_KTX", encoder)
	base := fixtureWorldMaterialPair(t, []uint32{7, 9})

	reflectance := image.NewNRGBA(image.Rect(0, 0, base.Layout.Width, base.Layout.Height))
	for offset := 0; offset < len(reflectance.Pix); offset += 4 {
		copy(reflectance.Pix[offset:offset+4], []byte{42, 173, 0, 255})
	}

	var err error

	base.Reflectance, _, err = qoi.Encode(reflectance, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceLinear})
	if err != nil {
		t.Fatal(err)
	}

	manifest := sdk.Manifest{}
	manifest.Assets.Capabilities.Runtime = []asset.Capability{asset.CapabilityWorldMaterialAtlasV1, worldmaterialgpu.Feature}
	assets := &assetBuild{projectRoot: t.TempDir(), manifest: manifest,
		materials: func(context.Context, string, sdk.Manifest, world.Document, map[uint32][]byte,
			...map[uint32]assetpipeline.MaterializeOptions,
		) (worldmaterial.Pair, assetpipeline.Artifact, error) {
			return base, assetpipeline.Artifact{}, nil
		}}

	metadata, err := level.EncodeMetadata(level.Metadata{})
	if err != nil {
		t.Fatal(err)
	}

	document := world.Document{Sectors: []world.Sector{{FloorMaterial: 7, CeilingMaterial: 9}}}

	result, err := buildWorldMaterials(t.Context(), assets, document, nil, nil, metadata, nil)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Contains(result.features, worldmaterialgpu.Feature) || slices.Contains(result.features, worldmaterialssbump.Feature) ||
		slices.Contains(result.features, worldmaterial.FeatureReflectance) {
		t.Fatal("incorrect runtime features", result.features)
	}

	if len(result.data) != 5 || len(result.textures) != 3 {
		t.Fatal("incorrect GPU resources", len(result.data), len(result.textures))
	}

	for _, entry := range result.data {
		if legacyMaterialGPUImage(entry.Name) || ownedSSBumpEntry(entry.Name) {
			t.Fatal("runtime retained intermediate QOI", entry.Name)
		}
	}

	for _, report := range result.textures {
		if report.Encoding != "ktx2-uastc-zstd" || report.EstimatedDecodedBytes != 699048 {
			t.Fatal("runtime report excludes complete GPU mip chain", report)
		}
	}

	identity, err := level.DecodeMetadata(result.metadata)
	if err != nil {
		t.Fatal(err)
	}

	marker, valid := identity[worldmaterialgpu.MetadataKey].AsString()
	if !valid || marker != worldmaterialgpu.Feature {
		t.Fatal("missing GPU declaration")
	}

	if _, err := level.Encode(result.metadata, result.data); err != nil {
		t.Fatal("invalid complete runtime level", err)
	}
}
