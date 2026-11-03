package build

import (
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/levelbuild"
)

func TestSpatialTextureReportKeepsTotalWithoutGlobalPreloadBudget(t *testing.T) {
	t.Parallel()

	const (
		shared = 64 << 20
		cell   = 16 << 20
	)

	textures := make([]assetpipeline.Texture, 1, 33)

	textures[0] = assetpipeline.Texture{Name: "shared-materials", EstimatedDecodedBytes: shared}
	for range 32 {
		textures = append(textures, assetpipeline.Texture{Name: "cell-rnm", Streamed: true, EstimatedDecodedBytes: cell})
	}

	report := assetpipeline.Report{}
	if err := includeLevelAssets(&report, []levelbuild.Artifact{{Name: "large-spatial-world", Textures: textures}}); err != nil {
		t.Fatal("streaming world was charged as a global preload", err)
	}

	if report.Summary.EstimatedDecodedLevelBytes != shared+32*cell || report.Summary.EstimatedStreamedLevelBytes != 32*cell {
		t.Fatal("report concealed total streamed memory", report.Summary)
	}

	for _, oversized := range []assetpipeline.Texture{{Name: "cell-too-large", Streamed: true, EstimatedDecodedBytes: asset.MaxDecodedTextures + 1}, {Name: "shared-too-large", EstimatedDecodedBytes: asset.MaxDecodedTextures + 1}} {
		if err := includeLevelAssets(
			&assetpipeline.Report{},
			[]levelbuild.Artifact{{Name: "bad", Textures: []assetpipeline.Texture{oversized}}},
		); err == nil {
			t.Fatal("individual resource budget bypassed", oversized.Name)
		}
	}
}
