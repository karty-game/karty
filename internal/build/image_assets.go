package build

import (
	"context"
	"fmt"

	"github.com/karty-game/karty-sdk/format/cartridge"
	"github.com/karty-game/karty-sdk/format/textureatlas"
	"github.com/karty-game/karty/internal/assetpipeline"
)

func projectImageAssets(ctx context.Context, textures []assetpipeline.Texture, prepared []*textureatlas.Bundle) ([]cartridge.Asset, error) {
	assets := make([]cartridge.Asset, 0, len(textures))
	if len(prepared) > 0 && prepared[0] != nil {
		entries, err := prepared[0].Entries()
		if err != nil {
			return nil, err
		}

		for _, entry := range entries {
			assets = append(assets, cartridge.Asset{Name: entry.Name, Bytes: entry.Bytes})
		}
	} else {
		for _, texture := range textures {
			contents, err := readVerifiedProcessedAsset(ctx, texture.SourcePath, texture.OutputSHA256, texture.OutputBytes)
			if err != nil {
				return nil, fmt.Errorf("read texture %q for cartridge: %w", texture.Name, err)
			}

			assets = append(assets, cartridge.Asset{Name: texture.Name, Bytes: contents})
		}
	}

	return assets, nil
}
