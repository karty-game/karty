package levelbuild

import (
	"fmt"
	"slices"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/worldsky"
	"github.com/karty-game/karty/internal/assetpipeline"
	worldsource "github.com/karty-game/karty/internal/worldbuild/source"
)

func compileWorldSky(directory string, definition manifest, assets *assetBuild) ([]level.SourceEntry, []string, error) {
	expanded, err := worldsource.Load(directory, definition.World.Source)
	if err != nil {
		return nil, nil, err
	}

	if expanded.Sky == nil {
		return nil, nil, nil
	}

	if assets == nil || !slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.Capability(worldsky.Feature)) {
		return nil, nil, fmt.Errorf("sky requires SDK advertising %s: %w", worldsky.Feature, ErrManifest)
	}

	source := expanded.Sky

	document := worldsky.Document{Version: worldsky.Version, Settings: source.ResolvedSettings()}

	for _, texture := range []struct {
		name, path string
		target     **worldsky.Image
	}{
		{"cloud_sparse", source.CloudSparse, &document.CloudSparse},
		{"cloud_dense", source.CloudDense, &document.CloudDense},
		{"background", source.Background, &document.Background},
		{"horizon", source.Horizon, &document.Horizon},
	} {
		if texture.path == "" {
			continue
		}

		input, err := readConfinedFile(directory, texture.path, asset.MaxSourceAssetBytes)
		if err != nil {
			return nil, nil, fmt.Errorf("sky %s source: %w", texture.name, err)
		}

		*texture.target, err = assetpipeline.CompileWorldSkyImage(input)
		if err != nil {
			return nil, nil, fmt.Errorf("sky %s: %w", texture.name, err)
		}
	}

	encoded, err := worldsky.Encode(document)
	if err != nil {
		return nil, nil, err
	}

	return []level.SourceEntry{{Name: worldsky.EntryName, Kind: level.EntryData, Data: encoded}}, []string{worldsky.Feature}, nil
}
