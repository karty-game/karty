package levelbuild

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"slices"

	worldlightmapbake "github.com/karty-game/karty-sdk/bake/worldlightmap"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldgpuchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty-sdk/format/worldterrain"
)

const (
	gpuDefaultCellSize    = 16
	gpuMaxSourceMeshBytes = 64 << 20
)

type gpuChunkSettings struct {
	CellSize       float64    `toml:"cell_size"`
	LiquidEdgeStep float64    `toml:"liquid_edge_step"`
	RoomGroups     [][]string `toml:"room_groups"`
}

type gpuWorldBuild struct {
	assets                    *assetBuild
	document                  world.Document
	definition                manifest
	materials                 worldmaterial.Layout
	materialSHA               [32]byte
	physical                  worldchunk.PlanResult
	terrain                   *worldterrain.Document
	cache                     *chunkPixelCache
	bakeMaterials             []worldlightmapbake.Material
	lighting                  worldlightmap.Options
	bake                      worldlightmapbake.Options
	ready                     worldchunk.ReadyOptions
	solidSectors              [][]uint32
	sectorProbes, solidProbes []uint32
}

func worldGPUChunksEnabled(assets *assetBuild) bool {
	return assets != nil && slices.Contains(assets.manifest.Assets.Capabilities.Runtime, worldgpuchunk.Feature)
}

func buildWorldGPUChunks(
	ctx context.Context,
	directory string,
	definition manifest,
	document world.Document,
	result worldMaterialBuild,
	assets *assetBuild,
) (worldMaterialBuild, error) {
	build, cells, err := prepareWorldGPUChunks(ctx, directory, definition, document, &result, assets)
	if err != nil {
		return worldMaterialBuild{}, err
	}

	var release func()

	build.cache, release = prepareChunkBaker(build.cache, document, build.bakeMaterials, build.lighting.PageSize > 0)
	defer release()

	physical, err := build.buildPhysics(ctx, &result)
	if err != nil {
		return worldMaterialBuild{}, fmt.Errorf("GPU physical chunks: %w", err)
	}

	catalog := worldgpuchunk.Catalog{MaterialLayoutSHA256: build.materialSHA, CellSize: gpuWorldSettings(definition).CellSize}

	geometry, err := worldchunk.GeometryDigest(&document)
	if err != nil {
		return worldMaterialBuild{}, err
	}

	if _, err := hex.Decode(catalog.GeometrySHA256[:], []byte(geometry)); err != nil {
		return worldMaterialBuild{}, err
	}

	for _, cell := range cells {
		artifact, descriptor, report, err := build.encodeCell(ctx, cell, uint32(len(catalog.Cells)), physical)
		if err != nil {
			return worldMaterialBuild{}, fmt.Errorf("encode GPU cell %v: %w", cell.grid, err)
		}

		result.chunks = append(result.chunks, artifact)
		result.textures = append(result.textures, report)
		catalog.Cells = append(catalog.Cells, descriptor)
	}

	catalog.BakedLights = gpuCellBakedLights(&document, cells)

	encoded, err := worldgpuchunk.EncodeCatalog(catalog)
	if err != nil {
		return worldMaterialBuild{}, err
	}

	result.data = append(result.data, level.SourceEntry{Name: worldgpuchunk.CatalogEntry, Kind: level.EntryData, Data: encoded})

	identity, err := level.DecodeMetadata(result.metadata)
	if err != nil {
		return worldMaterialBuild{}, err
	}

	identity[worldgpuchunk.MetadataKey] = level.StringValue(worldgpuchunk.Schema)

	result.metadata, err = level.EncodeMetadata(identity)
	if err != nil {
		return worldMaterialBuild{}, err
	}

	result.features = append(result.features, worldgpuchunk.Feature)
	if build.lighting.PageSize > 0 {
		result.features = append(result.features, worldlightmap.Feature)
		if definition.Lightmap.Fixed {
			result.features = append(result.features, worldlightmap.FixedFeature)
		} else {
			result.features = append(result.features, worldlightmap.PrebakeFeature)
		}
	}

	result.data = slices.DeleteFunc(result.data, func(entry level.SourceEntry) bool {
		return entry.Name == worldlightmap.PrebakeImageEntryName || entry.Name == worldlightmap.FixedImageEntryName
	})

	return result, nil
}

func prepareWorldGPUChunks(
	ctx context.Context,
	directory string,
	definition manifest,
	document world.Document,
	result *worldMaterialBuild,
	assets *assetBuild,
) (gpuWorldBuild, []gpuCellPlan, error) {
	build := gpuWorldBuild{assets: assets, document: document, definition: definition}

	settings, err := build.prepareInputs(result)
	if err != nil {
		return build, nil, err
	}

	fixed := definition.Lightmap != nil && definition.Lightmap.Fixed

	build.sectorProbes, build.solidProbes, err = plannedGPUCellProbes(&document, assets, fixed)
	if err != nil {
		return build, nil, err
	}

	policy := gpuCellPolicy{roomGroups: settings.RoomGroups}
	if definition.Lightmap != nil && definition.Lightmap.Enabled && !definition.Lightmap.Fixed {
		policy.minimumPage = 512
	}

	if definition.Chunks != nil && definition.Chunks.Spatial != nil {
		policy.spatial = definition.Chunks.Spatial

		policy.options, err = gpuLoadingChunkOptions(definition, &document)
		if err != nil {
			return build, nil, err
		}
	}

	cells, err := planGPUCells(
		ctx,
		&document,
		build.physical,
		build.terrain,
		settings.CellSize,
		build.lighting,
		settings.LiquidEdgeStep,
		policy,
	)
	if err != nil {
		return build, nil, err
	}

	cells, err = build.preflightCells(cells, result.ready)
	if err != nil {
		return build, nil, err
	}

	if build.lighting.PageSize > 0 {
		build.bakeMaterials, err = loadChunkBakeMaterials(directory, definition, &document, result.sources)
		if err != nil {
			return build, nil, err
		}

		build.cache, err = prepareChunkPixelCache(assets.projectRoot, directory)
		if err != nil {
			return build, nil, err
		}

		build.bake.Terrain = build.terrain

		build.bake.Vegetation, err = prepareFixedVegetationBake(*result)
		if err != nil {
			return build, nil, err
		}
	}

	return build, cells, nil
}

func gpuWorldSettings(definition manifest) gpuChunkSettings {
	settings := gpuChunkSettings{CellSize: gpuDefaultCellSize}

	if definition.Chunks != nil {
		if definition.Chunks.Spatial != nil {
			settings.CellSize = definition.Chunks.Spatial.TileSize
			settings.LiquidEdgeStep = definition.Chunks.Spatial.LiquidEdgeStep
		}

		if definition.Chunks.GPU != nil {
			settings = *definition.Chunks.GPU
			if settings.CellSize == 0 {
				settings.CellSize = gpuDefaultCellSize
			}
		}
	}

	return settings
}

func validGPUSettings(settings gpuChunkSettings) bool {
	return !math.IsNaN(settings.CellSize) && !math.IsInf(settings.CellSize, 0) && settings.CellSize >= 1 && settings.CellSize <= 1024 &&
		!math.IsNaN(
			settings.LiquidEdgeStep,
		) && !math.IsInf(settings.LiquidEdgeStep, 0) && settings.LiquidEdgeStep >= 0 && settings.LiquidEdgeStep <= 64 &&
		(settings.LiquidEdgeStep == 0 || math.Mod(settings.CellSize, settings.LiquidEdgeStep) == 0)
}

func worldGPUSolidCount(document *world.Document) int {
	if document.StaticSolids == nil {
		return 0
	}

	return len(document.StaticSolids.Items)
}
func gpuWorldEntry(entries []level.SourceEntry, name string) []byte {
	for _, entry := range entries {
		if entry.Name == name {
			return entry.Data
		}
	}

	return nil
}

func (build *gpuWorldBuild) prepareInputs(result *worldMaterialBuild) (gpuChunkSettings, error) {
	document, definition, assets := build.document, build.definition, build.assets

	var settings gpuChunkSettings

	if !worldMaterialGPUEnabled(assets) {
		return gpuChunkSettings{}, fmt.Errorf("GPU cells require prepared shared material arrays: %w", ErrManifest)
	}

	materialBytes := gpuWorldEntry(result.data, worldmaterial.LayoutEntry)

	var err error

	build.materials, err = worldmaterial.DecodeLayout(materialBytes)
	if err != nil {
		return gpuChunkSettings{}, err
	}

	build.materialSHA = sha256.Sum256(materialBytes)

	settings = gpuWorldSettings(definition)
	if !validGPUSettings(settings) {
		return gpuChunkSettings{}, fmt.Errorf("GPU cell settings: %w", ErrManifest)
	}

	maxSectors := 8
	if definition.Chunks != nil && definition.Chunks.MaxSectors != nil {
		maxSectors = *definition.Chunks.MaxSectors
	}

	var roomGroups []int
	if (definition.Chunks == nil || definition.Chunks.Spatial == nil) &&
		result.terrain == nil && len(gpuWorldEntry(result.data, worldterrain.EntryName)) == 0 {
		roomGroups, _, err = gpuRoomSectorGroups(&document, settings.RoomGroups)
		if err != nil {
			return gpuChunkSettings{}, err
		}

		// Indoor collision follows the same loading rooms as rendering. Convex
		// decomposition must not create additional physical loading boundaries.
		maxSectors = world.MaxSectors
	}

	build.physical, err = worldchunk.Plan(
		document,
		worldchunk.Options{
			NoLightmap:      true,
			MaxSectors:      maxSectors,
			MaxMeshBytes:    gpuMaxSourceMeshBytes,
			MaxEncodedBytes: worldchunk.MaxEncodedBytes,
			SectorGroups:    roomGroups,
		},
	)
	if err != nil {
		return gpuChunkSettings{}, err
	}

	if result.terrain != nil {
		build.terrain = result.terrain
	} else if data := gpuWorldEntry(result.data, worldterrain.EntryName); len(data) > 0 {
		decoded, err := worldterrain.Decode(data)
		if err != nil {
			return gpuChunkSettings{}, err
		}

		build.terrain = &decoded
	}

	build.lighting, build.bake, err = gpuLightingOptions(definition, &document, result.data)
	if err != nil {
		return gpuChunkSettings{}, err
	}

	return settings, nil
}

func (build *gpuWorldBuild) preflightCells(cells []gpuCellPlan, source *worldchunk.ReadyOptions) ([]gpuCellPlan, error) {
	var err error

	if source != nil {
		build.ready = *source
	} else {
		build.ready.Masked = make([]bool, len(build.materials.Materials))
		for index, m := range build.materials.Materials {
			build.ready.Masked[index] = m.Coverage == worldmaterial.CoverageMasked
		}
	}

	meshes := make([]worldchunk.Mesh, len(cells))
	for index := range cells {
		meshes[index] = cells[index].chunk.Mesh
	}

	build.ready.Surfaces, err = worldchunk.ReadySurfaces(meshes, &build.document)
	if err != nil {
		return nil, err
	}

	owners, err := worldchunk.SolidOwners(&build.document)
	if err != nil {
		return nil, err
	}

	build.solidSectors = make([][]uint32, len(owners))
	for solid, owner := range owners {
		build.solidSectors[solid] = []uint32{uint32(owner)}
	}

	if build.terrain != nil {
		for _, chunk := range build.physical.Chunks {
			for _, solid := range chunk.Solids {
				build.solidSectors[solid] = gpuIDs(chunk.Sectors)
			}
		}
	}

	kept := cells[:0]
	for _, cell := range cells {
		data, err := build.compileCell(cell, 1)
		if err != nil {
			return nil, fmt.Errorf("GPU cell preflight %v: %w", cell.grid, err)
		}

		if len(data.Draws) > 0 {
			kept = append(kept, cell)
		}
	}

	cells = kept

	return cells, nil
}
