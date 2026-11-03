package levelbuild

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"

	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldgpuchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
	"github.com/karty-game/karty/internal/assetpipeline"
)

func (build *gpuWorldBuild) compileCell(cell gpuCellPlan, span float64) (worldgpuchunk.GPUData, error) {
	encoded, err := gpuSourceMeshIdentity(&cell.chunk.Mesh, &build.document)
	if err != nil {
		return worldgpuchunk.GPUData{}, fmt.Errorf("identify GPU cell source mesh: %w", err)
	}

	var layout *worldlightmap.Layout
	if cell.chunk.Layout.Schema != "" {
		layout = &cell.chunk.Layout
	}

	ready, err := worldchunk.CompileReady(&cell.chunk.Mesh, encoded, &build.document, layout, build.ready)
	if err != nil {
		return worldgpuchunk.GPUData{}, fmt.Errorf("compile GPU cell ready mesh: %w", err)
	}

	renderable := false
	for _, batch := range ready.Batches {
		renderable = renderable || (!batch.Sky && !batch.Invisible && len(batch.Indices) > 0)
	}

	if !renderable {
		return worldgpuchunk.GPUData{}, nil
	}

	options := worldgpuchunk.CompileOptions{
		RGBMRange: span, CellSectors: cell.sectors, SolidSectors: build.solidSectors,
		SectorProbes: build.sectorProbes, SolidProbes: build.solidProbes,
	}
	if build.terrain != nil {
		for _, sector := range build.terrain.ExteriorSectors {
			options.SkipTerrainSectors = append(options.SkipTerrainSectors, uint32(sector))
		}
	}

	return worldgpuchunk.CompileReady(ready, &build.document, build.materials, options)
}

func (build *gpuWorldBuild) encodeCell(
	ctx context.Context,
	cell gpuCellPlan,
	cellID uint32,
	physical bool,
) (ChunkArtifact, worldgpuchunk.Cell, assetpipeline.Texture, error) {
	lighting, err := build.cellLighting(ctx, cell)
	if err != nil {
		return ChunkArtifact{}, worldgpuchunk.Cell{}, assetpipeline.Texture{}, err
	}

	rnm, cached, err := assetpipeline.EncodeCellRNM(
		ctx,
		build.assets.projectRoot,
		build.assets.manifest,
		lighting.width,
		lighting.height,
		lighting.pixels,
	)
	if err != nil {
		return ChunkArtifact{}, worldgpuchunk.Cell{}, assetpipeline.Texture{}, err
	}

	data, err := build.compileCell(cell, lighting.rangeValue)
	if err != nil {
		return ChunkArtifact{}, worldgpuchunk.Cell{}, assetpipeline.Texture{}, err
	}

	bounds := gpuDrawBounds(data.Draws)
	payload := worldgpuchunk.Payload{
		CellID:               cellID,
		Bounds:               bounds,
		MaterialLayoutSHA256: build.materialSHA,
		RGBMRange:            lighting.rangeValue,
		GPUData:              data,
	}

	ids := make([]uint32, len(build.materials.Materials))
	for index, material := range build.materials.Materials {
		ids[index] = material.MaterialID
	}

	if err := worldgpuchunk.ValidatePayload(payload, ids, len(build.document.Sectors), worldGPUSolidCount(&build.document)); err != nil {
		return ChunkArtifact{}, worldgpuchunk.Cell{}, assetpipeline.Texture{}, err
	}

	encoded, err := worldgpuchunk.EncodePayload(payload)
	if err != nil {
		return ChunkArtifact{}, worldgpuchunk.Cell{}, assetpipeline.Texture{}, err
	}

	entries := lighting.entries
	entries = append(
		entries,
		level.SourceEntry{Name: worldgpuchunk.PayloadEntry, Kind: level.EntryData, Data: encoded},
		level.SourceEntry{Name: worldgpuchunk.RNMEntry, Kind: level.EntryData, Data: rnm},
	)

	metadata, err := level.EncodeMetadata(
		level.Metadata{worldgpuchunk.CellMetadataKey: level.StringValue(worldgpuchunk.Schema), "cellID": level.UintValue(uint64(cellID))},
	)
	if err != nil {
		return ChunkArtifact{}, worldgpuchunk.Cell{}, assetpipeline.Texture{}, err
	}

	module, err := encodeLevelModule(build.definition.Level.Name, metadata, entries)
	if err != nil {
		return ChunkArtifact{}, worldgpuchunk.Cell{}, assetpipeline.Texture{}, err
	}

	digest := sha256.Sum256(module)

	descriptor := worldgpuchunk.Cell{
		ID:            cellID,
		SpaceID:       cell.space,
		Group:         cell.group,
		GridSpan:      cell.span,
		Grid:          cell.grid,
		Bounds:        bounds,
		Sectors:       cell.sectors,
		ContentSHA256: digest,
		Size:          uint64(len(module)),
		GPUBytes: uint64(
			len(data.Vertices) + len(data.Indices) + len(data.Addresses) + len(data.Records),
		),
		RNMBytes: uint64(lighting.width) * uint64(lighting.height) * cellRGBMAtlasPixelBytes,
	}
	if physical {
		descriptor.PhysicalIDs = cell.physical
	}

	check := worldgpuchunk.Catalog{MaterialLayoutSHA256: build.materialSHA, Cells: make([]worldgpuchunk.Cell, int(cellID)+1)}

	check.Cells[cellID] = descriptor
	if err := check.ValidateCell(cellID, payload, rnm); err != nil {
		return ChunkArtifact{}, worldgpuchunk.Cell{}, assetpipeline.Texture{}, err
	}

	imageDigest := sha256.Sum256(rnm)
	checksum := hex.EncodeToString(imageDigest[:])
	report := assetpipeline.Texture{Streamed: true,
		Kind:                  "world-cell-rnm",
		Name:                  fmt.Sprintf("%s#%d", worldgpuchunk.RNMEntry, cellID),
		Profile:               "world-cell-rnm",
		Source:                world.EntryName,
		Output:                checksum + ".ktx2",
		ContentSHA256:         checksum,
		OutputSHA256:          checksum,
		SourceSHA256:          cached.SourceDigest,
		CacheKey:              cached.CacheKey,
		CacheHit:              cached.CacheHit,
		Processor:             worldgpuchunk.Feature,
		Encoding:              "ktx2-rgba8-zstd",
		Width:                 int(lighting.width),
		Height:                int(lighting.height),
		OutputBytes:           int64(len(rnm)),
		EstimatedDecodedBytes: int64(descriptor.RNMBytes),
	}
	artifact := ChunkArtifact{
		ID:              len(build.physical.Chunks) + int(cellID),
		Bytes:           module,
		ContentSHA256:   hex.EncodeToString(digest[:]),
		EnvelopeVersion: level.EnvelopeVersion,
	}

	return artifact, descriptor, report, nil
}

func gpuDrawBounds(draws []worldgpuchunk.Draw) [6]float64 {
	bounds := [6]float64{math.Inf(1), math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1), math.Inf(-1)}

	for _, draw := range draws {
		for axis := range 3 {
			bounds[axis] = min(bounds[axis], draw.Bounds[axis])
			bounds[axis+3] = max(bounds[axis+3], draw.Bounds[axis+3])
		}
	}

	return bounds
}
