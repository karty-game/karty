package levelbuild

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty/internal/assetpipeline"
	worldgeometry "github.com/karty-game/karty/internal/worldbuild/geometry"
	worldsource "github.com/karty-game/karty/internal/worldbuild/source"
)

type worldMaterialBuild struct {
	data     []level.SourceEntry
	metadata []byte
	textures []assetpipeline.Texture
	features []string
	chunks   []ChunkArtifact
	sources  []level.SourceEntry
}

func compileMaterialWorld(
	directory string,
	definition manifest,
	textures []metadataTexture,
	sources []level.SourceEntry,
) (world.Document, []byte, error) {
	expanded, err := worldsource.Load(directory, definition.World.Source)
	if err != nil {
		return world.Document{}, nil, fmt.Errorf("compile level %q world source: %w", definition.Level.Name, err)
	}

	materials := make(map[string]uint32, len(textures))
	for _, texture := range textures {
		materials[texture.Name] = texture.ID
	}

	compiled, err := worldgeometry.Compile(expanded, materials)
	if err != nil {
		return world.Document{}, nil, fmt.Errorf("compile level %q world geometry: %w", definition.Level.Name, err)
	}

	if err := classifyBandSources(compiled, textures, sources); err != nil {
		return world.Document{}, nil, err
	}

	if err := validateAnimationFrames(compiled, textures); err != nil {
		return world.Document{}, nil, err
	}

	classifyCompiledFrames(&compiled, textures)

	if err := compileMaterialEmission(&compiled, definition.Textures, textures); err != nil {
		return world.Document{}, nil, err
	}

	encoded, err := world.Encode(compiled)
	if err != nil {
		return world.Document{}, nil, fmt.Errorf("encode level %q world: %w", definition.Level.Name, err)
	}

	return compiled, encoded, nil
}

//nolint:gocognit // Coverage proofs walk the complete compiled partition before packaging.
func classifyCompiledFrames(document *world.Document, textures []metadataTexture) {
	classes := make(map[uint32]string, len(textures))
	for _, texture := range textures {
		classes[texture.ID] = texture.Coverage
	}

	for si := range document.Sectors {
		sector := &document.Sectors[si]
		for wi := range sector.Walls {
			wall := &sector.Walls[wi]
			for ri := range wall.FrameRegions {
				region := &wall.FrameRegions[ri]
				switch classes[region.Material] {
				case "empty":
					region.Material, region.Coverage, region.UV = wall.Material, world.FrameCoverageMain, nil
					region.RepeatU, region.RepeatV = false, false
				case "opaque":
					if region.UV == nil {
						continue
					}

					projection := region.UV.Projections[0]
					covered := true

					for _, vertex := range region.Vertices {
						x := wall.Start.X + (wall.End.X-wall.Start.X)*vertex.X
						y := wall.Start.Y + (wall.End.Y-wall.Start.Y)*vertex.X
						u := projection.U.X*x + projection.U.Y*y + projection.U.Z*vertex.Y + projection.U.Offset

						v := projection.V.X*x + projection.V.Y*y + projection.V.Z*vertex.Y + projection.V.Offset
						if (!region.RepeatU && (u < 0 || u > 1)) || (!region.RepeatV && (v < 0 || v > 1)) {
							covered = false

							break
						}
					}

					if covered {
						region.Coverage = world.FrameCoverageOpaque
					}
				}
			}
		}
	}
}

func compileWorldAssets(ctx context.Context, directory string, definition manifest, textureMetadata []metadataTexture,
	textures, data []level.SourceEntry, metadata []byte, assets *assetBuild) (worldMaterialBuild, error) {
	if err := validateMaterializeSettings(definition, assets); err != nil {
		return worldMaterialBuild{}, err
	}

	if err := validateMaterialEmission(definition, assets); err != nil {
		return worldMaterialBuild{}, err
	}

	compiled, encoded, err := compileMaterialWorld(directory, definition, textureMetadata, textures)
	if err != nil {
		return worldMaterialBuild{}, err
	}

	if compiled.MaterialLayers != nil && (assets == nil ||
		!slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialLayersV1) ||
		!slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialAtlasV2)) {
		return worldMaterialBuild{}, fmt.Errorf("advanced wall materials require %s and %s: %w",
			asset.CapabilityWorldMaterialLayersV1, asset.CapabilityWorldMaterialAtlasV2, ErrManifest)
	}

	particleEntries, particleFeatures, err := compileWorldParticles(directory, definition, &compiled, textureMetadata, assets)
	if err != nil {
		return worldMaterialBuild{}, fmt.Errorf("level %q particles: %w", definition.Level.Name, err)
	}

	data = append(data, particleEntries...)

	vegetationEntries, vegetationFeatures, err := compileWorldVegetation(directory, definition, &compiled, textureMetadata, assets)
	if err != nil {
		return worldMaterialBuild{}, fmt.Errorf("level %q vegetation: %w", definition.Level.Name, err)
	}

	data = append(data, vegetationEntries...)

	outdoorEntries, outdoorFeatures, err := compileWorldSkyAndTerrain(directory, definition, &compiled, assets)
	if err != nil {
		return worldMaterialBuild{}, fmt.Errorf("level %q outdoor resources: %w", definition.Level.Name, err)
	}

	data = append(data, outdoorEntries...)

	animations, err := worldAnimationFeatures(assets, compiled)
	if err != nil {
		return worldMaterialBuild{}, err
	}

	environment, err := worldEnvironmentFeatures(assets, compiled)
	if err != nil {
		return worldMaterialBuild{}, fmt.Errorf("level %q environment: %w", definition.Level.Name, err)
	}

	lighting, err := worldLightingFeatures(assets, compiled)
	if err != nil {
		return worldMaterialBuild{}, fmt.Errorf("level %q world lighting: %w", definition.Level.Name, err)
	}

	mapping, err := worldMappingFeatures(assets, compiled)
	if err != nil {
		return worldMaterialBuild{}, fmt.Errorf("level %q material mapping: %w", definition.Level.Name, err)
	}

	solids, err := worldSolidsFeatures(assets, compiled)
	if err != nil {
		return worldMaterialBuild{}, fmt.Errorf("level %q static solids: %w", definition.Level.Name, err)
	}

	strengths, err := resolveMaterialStrengths(definition.Textures, textureMetadata, compiled)
	if err != nil {
		return worldMaterialBuild{}, err
	}

	materialize, err := resolveMaterializeSettings(definition.Textures, textureMetadata, compiled)
	if err != nil {
		return worldMaterialBuild{}, err
	}

	data = append(data, level.SourceEntry{Name: world.EntryName, Kind: level.EntryData, Data: encoded})

	definition, chunkCache, cachedChunks, err := prepareWorldChunkCache(ctx, directory, definition, compiled, assets)
	if err != nil {
		return worldMaterialBuild{}, err
	}

	var projectDirectory string
	if assets != nil {
		projectDirectory = assets.projectRoot
	}

	var lightmaps []string
	if definition.Chunks == nil || !definition.Chunks.Enabled {
		data, metadata, lightmaps, err = buildWorldLightmaps(
			projectDirectory,
			directory,
			definition,
			compiled,
			data,
			metadata,
			assets != nil && assets.development,
		)
		if err != nil {
			return worldMaterialBuild{}, fmt.Errorf("level %q lightmaps: %w", definition.Level.Name, err)
		}
	}

	atlas, err := buildWorldMaterials(ctx, assets, compiled, textures, data, metadata, strengths, materialize)
	if err != nil {
		return worldMaterialBuild{}, fmt.Errorf("level %q material atlas: %w", definition.Level.Name, err)
	}

	if compiled.Emission != nil {
		atlas.features = append(atlas.features, world.FeatureEmission)
	}

	atlas.features = append(atlas.features, particleFeatures...)
	atlas.features = append(atlas.features, vegetationFeatures...)
	atlas.features = append(atlas.features, outdoorFeatures...)
	atlas.features = append(atlas.features, animations...)
	atlas.features = append(atlas.features, environment...)
	atlas.features = append(atlas.features, lighting...)
	atlas.features = append(atlas.features, mapping...)
	atlas.features = append(atlas.features, solids...)
	atlas.features = append(atlas.features, lightmaps...)
	atlas.features = append(atlas.features, world.Feature)

	atlas.sources = textures
	if definition.Chunks != nil && definition.Chunks.Enabled {
		atlas, err = finishWorldChunks(ctx, directory, definition, compiled, atlas, assets, chunkCache, cachedChunks)
		if err != nil {
			return worldMaterialBuild{}, fmt.Errorf("level %q chunks: %w", definition.Level.Name, err)
		}
	}

	if err := validateWorldRuntimeLights(&compiled, atlas); err != nil {
		return worldMaterialBuild{}, fmt.Errorf("level %q world lighting: %w", definition.Level.Name, err)
	}

	return atlas, nil
}

func buildWorldMaterials(
	ctx context.Context,
	assets *assetBuild,
	document world.Document,
	textures, data []level.SourceEntry,
	metadata []byte,
	strengths map[uint32]*worldmaterial.Strengths,
	options ...map[uint32]assetpipeline.MaterializeOptions,
) (worldMaterialBuild, error) {
	result := worldMaterialBuild{data: data, metadata: metadata}
	if assets == nil || (!slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialAtlasV1) &&
		!slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialAtlasV2)) {
		return result, nil
	}

	if err := ctx.Err(); err != nil {
		return worldMaterialBuild{}, err
	}

	textureBytes := make(map[uint32][]byte, len(textures))
	for _, entry := range textures {
		id, valid := level.TextureAssetID(entry.Name)
		if !valid {
			return worldMaterialBuild{}, ErrSource
		}

		textureBytes[id] = entry.Data
	}

	process := assets.materials
	if process == nil {
		process = assetpipeline.ProcessWorldMaterials
	}

	pair, cached, err := process(ctx, assets.projectRoot, assets.manifest, document, textureBytes, options...)
	if err != nil {
		return worldMaterialBuild{}, err
	}

	if err := ctx.Err(); err != nil {
		return worldMaterialBuild{}, err
	}

	if len(pair.Reflectance) != 0 &&
		!slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialReflectanceV1) {
		return worldMaterialBuild{}, fmt.Errorf("reflectance requires %s: %w", worldmaterial.FeatureReflectance, ErrManifest)
	}

	if document.MaterialLayers != nil && pair.Layout.Schema != worldmaterial.SchemaV2 {
		return worldMaterialBuild{}, worldmaterial.ErrAtlas
	}

	pair.Layout = applyMaterialStrengths(pair.Layout, strengths)
	if err := worldmaterial.ValidateAnimationCoverage(&document, pair.Layout); err != nil {
		return worldMaterialBuild{}, err
	}

	result.data, result.metadata, err = packageWorldMaterials(data, metadata, document, pair)
	if err != nil {
		return worldMaterialBuild{}, err
	}

	result.textures = describeWorldMaterials(pair, cached)

	result.features = []string{worldmaterial.Feature}
	if pair.Layout.Schema == worldmaterial.SchemaV2 {
		result.features = []string{worldmaterial.FeatureV2, world.FeatureMaterialLayers}
	}

	if len(pair.Reflectance) != 0 {
		result.features = append(result.features, worldmaterial.FeatureReflectance)
	}

	return result, nil
}

// Include all generated QOI entries in the level report and decoded budget.
// Authored source costs stay on original textures to avoid double charging.
func describeWorldMaterials(pair worldmaterial.Pair, cached assetpipeline.Artifact) []assetpipeline.Texture {
	const pixelBytes = 4

	result := make([]assetpipeline.Texture, 0, 5)

	images := []struct {
		name        string
		bytes       []byte
		tail        bool
		reflectance bool
	}{
		{worldmaterial.AlbedoEntry, pair.Albedo, false, false},
		{worldmaterial.DataEntry, pair.Data, false, false},
		{worldmaterial.MipTailEntry, pair.MipTail, true, false},
		{worldmaterial.ReflectanceEntry, pair.Reflectance, false, true},
		{worldmaterial.ReflectanceMipTailEntry, pair.ReflectanceMipTail, true, true},
	}
	for _, entry := range images {
		if len(entry.bytes) == 0 {
			continue
		}

		name, encoded := entry.name, entry.bytes

		width, height := pair.Layout.Width, pair.Layout.Height
		if entry.tail {
			width, height = pair.Layout.MipDimensions()
		}

		processor := materialAtlasFeature(pair.Layout)
		if entry.reflectance {
			processor = worldmaterial.FeatureReflectance
		}

		digest := sha256.Sum256(encoded)
		checksum := hex.EncodeToString(digest[:])
		result = append(result, assetpipeline.Texture{
			Kind: "world-material", Name: name, Profile: "world-material", Source: world.EntryName,
			Output: checksum + ".qoi", ContentSHA256: checksum, OutputSHA256: checksum,
			SourceSHA256: cached.SourceDigest, CacheKey: cached.CacheKey, CacheHit: cached.CacheHit,
			Processor: processor, Encoding: "qoi", Width: width, Height: height,
			OutputBytes:           int64(len(encoded)),
			EstimatedDecodedBytes: int64(width) * int64(height) * pixelBytes,
		})
	}

	return result
}

func materialAtlasFeature(layout worldmaterial.Layout) string {
	if layout.Schema == worldmaterial.SchemaV2 {
		return worldmaterial.FeatureV2
	}

	return worldmaterial.Feature
}

func packageWorldMaterials(data []level.SourceEntry, metadata []byte,
	document world.Document, pair worldmaterial.Pair) ([]level.SourceEntry, []byte, error) {
	if err := pair.Validate(); err != nil {
		return nil, nil, err
	}

	ids := assetpipeline.WorldMaterialIDs(document)
	if len(ids) != len(pair.Layout.Materials) {
		return nil, nil, worldmaterial.ErrAtlas
	}

	for i, id := range ids {
		if pair.Layout.Materials[i].MaterialID != id {
			return nil, nil, worldmaterial.ErrAtlas
		}
	}

	for _, entry := range data {
		if entry.Name == worldmaterial.LayoutEntry || entry.Name == worldmaterial.AlbedoEntry || entry.Name == worldmaterial.DataEntry ||
			entry.Name == worldmaterial.MipTailEntry || entry.Name == worldmaterial.ReflectanceEntry ||
			entry.Name == worldmaterial.ReflectanceMipTailEntry {
			return nil, nil, ErrSource
		}
	}

	layoutJSON, err := worldmaterial.EncodeLayout(pair.Layout)
	if err != nil {
		return nil, nil, err
	}

	identity, err := level.DecodeMetadata(metadata)
	if err != nil {
		return nil, nil, ErrManifest
	}

	identity[worldmaterial.MetadataKey] = level.StringValue(pair.Layout.Schema)

	metadata, err = level.EncodeMetadata(identity)
	if err != nil {
		return nil, nil, err
	}

	data = append(data,
		level.SourceEntry{Name: worldmaterial.LayoutEntry, Kind: level.EntryData, Data: layoutJSON},
		level.SourceEntry{Name: worldmaterial.AlbedoEntry, Kind: level.EntryData, Data: pair.Albedo},
		level.SourceEntry{Name: worldmaterial.DataEntry, Kind: level.EntryData, Data: pair.Data})

	if len(pair.MipTail) != 0 {
		data = append(data, level.SourceEntry{Name: worldmaterial.MipTailEntry, Kind: level.EntryData, Data: pair.MipTail})
	}

	if len(pair.Reflectance) != 0 {
		data = append(data, level.SourceEntry{Name: worldmaterial.ReflectanceEntry, Kind: level.EntryData, Data: pair.Reflectance})
	}

	if len(pair.ReflectanceMipTail) != 0 {
		data = append(
			data,
			level.SourceEntry{Name: worldmaterial.ReflectanceMipTailEntry, Kind: level.EntryData, Data: pair.ReflectanceMipTail},
		)
	}

	return data, metadata, nil
}
