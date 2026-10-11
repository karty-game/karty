package levelbuild

import (
	"fmt"
	"slices"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

func worldLightingFeatures(assets *assetBuild, document world.Document) ([]string, error) {
	if document.Lighting == nil {
		return nil, nil
	}

	if assets == nil || !slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldLightingV1) {
		return nil, fmt.Errorf("requires an SDK advertising %s: %w", asset.CapabilityWorldLightingV1, ErrManifest)
	}

	return []string{world.FeatureLighting}, nil
}

// Authored sources can outnumber the runtime light table only after their
// contributions have been replaced by validated completed coefficient maps.
// Count actual packaged recipes, including all physical chunk components;
// a missing/stale development bake must never silently drop authored lights.
func validateWorldRuntimeLights(document *world.Document, result worldMaterialBuild) error {
	if document.Lighting == nil || len(document.Lighting.Lights) <= world.MaxRuntimeLights {
		return nil
	}

	baked, err := collectBakedLightIDs(document, result)
	if err != nil {
		return err
	}

	live := len(document.Lighting.Lights) - len(baked)
	if live > world.MaxRuntimeLights {
		return fmt.Errorf(
			"%d live authored lights exceed the runtime limit of %d; produce a complete offline bake before building or previewing this level: %w",
			live,
			world.MaxRuntimeLights,
			ErrManifest,
		)
	}

	return nil
}

func collectBakedLightIDs(document *world.Document, result worldMaterialBuild) (map[string]bool, error) {
	baked := make(map[string]bool)

	if !slices.Contains(result.features, worldlightmap.PrebakeFeature) {
		return baked, nil
	}

	for _, entry := range result.data {
		if entry.Name == worldlightmap.EntryName {
			if err := addBakedLightIDs(baked, document, entry.Data); err != nil {
				return nil, err
			}
		}
	}

	for _, chunk := range result.chunks {
		envelope, err := level.DecodePassiveModule(chunk.Bytes)
		if err != nil {
			return nil, err
		}

		encoded, _, found := envelope.Read(worldlightmap.EntryName, 0, worldlightmap.MaxEncodedSize)
		if !found {
			return nil, ErrManifest
		}

		if err := addBakedLightIDs(baked, document, encoded); err != nil {
			return nil, err
		}
	}

	return baked, nil
}

func addBakedLightIDs(baked map[string]bool, document *world.Document, encoded []byte) error {
	layout, err := worldlightmap.Decode(encoded, document)
	if err != nil {
		return err
	}

	if layout.RuntimeBake != nil && layout.RuntimeBake.Encoding == worldlightmap.DirectRNMEncoding {
		for _, id := range layout.RuntimeBake.LightIDs {
			baked[id] = true
		}
	}

	return nil
}
