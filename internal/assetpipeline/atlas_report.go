package assetpipeline

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"slices"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/textureatlas"
	"github.com/karty-game/karty/internal/sdk"
)

// PrepareGameAtlas replaces runtime image payloads and costs while retaining
// logical source records for diagnostics and incremental build inspection.
func PrepareGameAtlas(ctx context.Context, directory string, manifest sdk.Manifest, report *Report) error {
	if len(report.Textures) == 0 || !slices.Contains(manifest.Assets.Capabilities.Runtime, asset.CapabilityTextureAtlasV1) {
		return nil
	}

	inputs := make([]AtlasImage, len(report.Textures))
	for index, texture := range report.Textures {
		if err := ctx.Err(); err != nil {
			return err
		}

		contents, err := os.ReadFile(texture.SourcePath)
		if err != nil {
			return err
		}

		if int64(len(contents)) != texture.OutputBytes || fmt.Sprintf("%x", sha256.Sum256(contents)) != texture.OutputSHA256 {
			return ErrCacheOutput
		}

		inputs[index] = AtlasImage{Name: texture.Name, Bytes: contents}
	}

	bundle, pages, err := ProcessTextureAtlas(ctx, directory, inputs, nil)
	if err != nil {
		return err
	}

	report.TextureSources, report.Textures, report.Atlas = report.Textures, pages, &bundle
	oldOutput := report.Summary.OutputBytes
	report.Summary.OutputBytes = 0

	report.Summary.EstimatedDecodedBytes = int64(bundle.Layout.DecodedBytes())
	for _, page := range pages {
		report.Summary.OutputBytes += page.OutputBytes
	}

	layout, err := textureatlas.EncodeLayout(bundle.Layout)
	if err != nil {
		return err
	}

	report.Summary.OutputBytes += int64(len(layout))
	report.Summary.AssetOutputBytes += report.Summary.OutputBytes - oldOutput
	report.Summary.TextureCount = len(pages)
	report.Features = append(report.Features, textureatlas.Feature)
	slices.Sort(report.Features)
	report.Features = slices.Compact(report.Features)

	return nil
}
