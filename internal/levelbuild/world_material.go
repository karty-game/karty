package levelbuild

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
}

func compileMaterialWorld(directory string, definition manifest, textures []metadataTexture) (world.Document, []byte, error) {
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

	encoded, err := world.Encode(compiled)
	if err != nil {
		return world.Document{}, nil, fmt.Errorf("encode level %q world: %w", definition.Level.Name, err)
	}

	return compiled, encoded, nil
}

func compileWorldAssets(ctx context.Context, directory string, definition manifest, textureMetadata []metadataTexture,
	textures, data []level.SourceEntry, metadata []byte, assets *assetBuild) (worldMaterialBuild, error) {
	compiled, encoded, err := compileMaterialWorld(directory, definition, textureMetadata)
	if err != nil {
		return worldMaterialBuild{}, err
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

	data = append(data, level.SourceEntry{Name: world.EntryName, Kind: level.EntryData, Data: encoded})

	var projectDirectory string
	if assets != nil {
		projectDirectory = assets.projectRoot
	}

	data, metadata, lightmaps, err := buildWorldLightmaps(projectDirectory, directory, definition, compiled, data, metadata)
	if err != nil {
		return worldMaterialBuild{}, fmt.Errorf("level %q lightmaps: %w", definition.Level.Name, err)
	}

	atlas, err := buildWorldMaterials(ctx, assets, compiled, textures, data, metadata, strengths)
	if err != nil {
		return worldMaterialBuild{}, fmt.Errorf("level %q material atlas: %w", definition.Level.Name, err)
	}

	atlas.features = append(atlas.features, lighting...)
	atlas.features = append(atlas.features, mapping...)
	atlas.features = append(atlas.features, solids...)
	atlas.features = append(atlas.features, lightmaps...)
	atlas.features = append(atlas.features, world.Feature)

	return atlas, nil
}

func buildWorldMaterials(ctx context.Context, assets *assetBuild, document world.Document,
	textures, data []level.SourceEntry, metadata []byte, strengths map[uint32]*worldmaterial.Strengths) (worldMaterialBuild, error) {
	result := worldMaterialBuild{data: data, metadata: metadata}
	if assets == nil || !slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialAtlasV1) {
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

	pair, cached, err := process(ctx, assets.projectRoot, assets.manifest, document, textureBytes)
	if err != nil {
		return worldMaterialBuild{}, err
	}

	if err := ctx.Err(); err != nil {
		return worldMaterialBuild{}, err
	}

	pair.Layout = applyMaterialStrengths(pair.Layout, strengths)

	result.data, result.metadata, err = packageWorldMaterials(data, metadata, document, pair)
	if err != nil {
		return worldMaterialBuild{}, err
	}

	result.textures = describeWorldMaterials(pair, cached)
	result.features = []string{worldmaterial.Feature}

	return result, nil
}

// Include all generated QOI entries in the level report and decoded budget.
// Authored source costs stay on original textures to avoid double charging.
func describeWorldMaterials(pair worldmaterial.Pair, cached assetpipeline.Artifact) []assetpipeline.Texture {
	const pixelBytes = 4

	result := make([]assetpipeline.Texture, 0, 3)

	images := [][]byte{pair.Albedo, pair.Data}
	if len(pair.MipTail) != 0 {
		images = append(images, pair.MipTail)
	}

	for index, encoded := range images {
		name := worldmaterial.AlbedoEntry
		if index == 1 {
			name = worldmaterial.DataEntry
		}

		width, height := pair.Layout.Width, pair.Layout.Height
		if index == len(images)-1 && len(pair.MipTail) != 0 {
			name = worldmaterial.MipTailEntry
			width, height = pair.Layout.MipDimensions()
		}

		digest := sha256.Sum256(encoded)
		checksum := hex.EncodeToString(digest[:])
		result = append(result, assetpipeline.Texture{
			Kind: "world-material", Name: name, Profile: "world-material", Source: world.EntryName,
			Output: checksum + ".qoi", ContentSHA256: checksum, OutputSHA256: checksum,
			SourceSHA256: cached.SourceDigest, CacheKey: cached.CacheKey, CacheHit: cached.CacheHit,
			Processor: worldmaterial.Feature, Encoding: "qoi", Width: width, Height: height,
			OutputBytes:           int64(len(encoded)),
			EstimatedDecodedBytes: int64(width) * int64(height) * pixelBytes,
		})
	}

	return result
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
			entry.Name == worldmaterial.MipTailEntry {
			return nil, nil, ErrSource
		}
	}

	layoutJSON, err := worldmaterial.EncodeLayout(pair.Layout)
	if err != nil {
		return nil, nil, err
	}

	var identity map[string]json.RawMessage

	if err := json.Unmarshal(metadata, &identity); err != nil || identity == nil {
		return nil, nil, ErrManifest
	}

	identity[worldmaterial.MetadataKey], err = json.Marshal(worldmaterial.Schema)
	if err != nil {
		return nil, nil, err
	}

	metadata, err = json.Marshal(identity)
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

	return data, metadata, nil
}
