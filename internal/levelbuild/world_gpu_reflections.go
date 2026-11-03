package levelbuild

import (
	"slices"

	"github.com/karty-game/karty-sdk/bake/reflectionplan"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldreflection"
)

// Probe placement depends only on immutable world geometry. Resolve these slots
// before cell encoding so capture and final publication retain identical cells.
func plannedGPUCellProbes(document *world.Document, assets *assetBuild, fixed bool) ([]uint32, []uint32, error) {
	if !fixed || !preparedGPUReflectionEnabled(assets) {
		return nil, nil, nil
	}

	planned, err := reflectionplan.Plan(document, "")
	if err != nil {
		return nil, nil, err
	}

	slots := make(map[int]uint32, len(planned.Probes))
	for index, probe := range planned.Probes {
		slots[probe.ID] = uint32(index + 1)
	}

	sectors, err := gpuOwnerProbeSlots(planned.Sectors, slots)
	if err != nil {
		return nil, nil, err
	}

	solids, err := gpuOwnerProbeSlots(planned.Solids, slots)

	return sectors, solids, err
}

func gpuOwnerProbeSlots(owners []int, slots map[int]uint32) ([]uint32, error) {
	result := make([]uint32, len(owners))
	for owner, probe := range owners {
		if probe == -1 {
			continue
		}

		slot, found := slots[probe]
		if !found {
			return nil, worldreflection.ErrCubemaps
		}

		result[owner] = slot
	}

	return result, nil
}

func preparedGPUReflectionEnabled(assets *assetBuild) bool {
	return assets != nil &&
		slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.Capability(worldreflection.GPUFeature))
}
