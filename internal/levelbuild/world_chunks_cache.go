package levelbuild

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/pelletier/go-toml/v2"
)

// This is a private build cache revision, not a new cartridge format. Bump it
// when changing production without changing the SDK's public schema/version.
const chunkCacheRevision = "karty-cli-chunk-cache@5"

type worldChunkCache struct {
	key     string
	entries []level.SourceEntry
	chunks  []ChunkArtifact
}

func worldChunkCacheKey(
	ctx context.Context,
	directory string,
	definition manifest,
	document world.Document,
	assets *assetBuild,
) (string, error) {
	encoded, err := world.Encode(document)
	if err != nil {
		return "", err
	}

	settings, err := toml.Marshal(struct {
		Level    string
		Chunks   *chunkBuildSettings
		Lightmap *lightmapBuildSettings
		Textures []textureEntry
	}{definition.Level.Name, definition.Chunks, definition.Lightmap, definition.Textures})
	if err != nil {
		return "", err
	}

	sdkInputs, err := toml.Marshal(assets.manifest)
	if err != nil {
		return "", err
	}

	hash := sha256.New()
	add := func(data []byte) {
		var size [8]byte
		binary.LittleEndian.PutUint64(size[:], uint64(len(data)))
		hash.Write(size[:])
		hash.Write(data)
	}
	add([]byte(chunkCacheRevision))
	add(sdkInputs)
	add(settings)
	add(encoded)

	for _, texture := range definition.Textures {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		contents, err := readConfinedFile(directory, texture.Source, asset.MaxSourceAssetBytes)
		if err != nil {
			return "", err
		}

		digest := sha256.Sum256(contents)
		add(digest[:])
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// Read the bounded cache container and every hash-bound child before choosing
// streaming. A missing/corrupt/escaped child makes the whole cache unavailable.
func readWorldChunkCache(
	ctx context.Context,
	root, directory, key string,
	document *world.Document,
	definition manifest,
) (worldChunkCache, bool) {
	cache := worldChunkCache{key: key}

	path, err := automaticBakePath(root, directory)
	if err != nil {
		return cache, false
	}

	encoded, err := readConfinedFile(root, filepath.Join(path, "chunks-"+key+".bin"), level.MaxEnvelopeSize)
	if err != nil {
		return cache, false
	}

	envelope, err := level.Decode(encoded)
	if err != nil || len(envelope.Entries) < 1 || len(envelope.Entries) > 2 {
		return cache, false
	}

	for _, entry := range envelope.Entries {
		if entry.Kind != level.EntryData || (entry.Name != worldchunk.CatalogEntryName && entry.Name != worldchunk.ShadowEntryName) {
			return cache, false
		}

		data, _, found := envelope.Read(entry.Name, 0, entry.Length)
		if !found {
			return cache, false
		}

		cache.entries = append(cache.entries, level.SourceEntry{Name: entry.Name, Kind: entry.Kind, Data: data})
	}

	catalogBytes, _, found := envelope.Read(worldchunk.CatalogEntryName, 0, worldchunk.MaxEncodedSize)
	if !found {
		return cache, false
	}

	catalog, err := worldchunk.DecodeCatalog(catalogBytes, document)
	if err != nil {
		return cache, false
	}

	options, err := chunkOptions(definition)
	if err != nil {
		return cache, false
	}

	groups, err := resolveChunkAreas(definition.Chunks.Areas, document)
	if err != nil {
		return cache, false
	}

	if shadow, _, found := envelope.Read(worldchunk.ShadowEntryName, 0, worldchunk.MaxShadowEncodedSize); found {
		if _, err := worldchunk.DecodeShadowProxy(shadow, document); err != nil {
			return cache, false
		}
	}

	for _, child := range catalog.Chunks {
		if ctx.Err() != nil {
			return cache, false
		}

		if !chunkDescriptorWithinOptions(child, options, groups) {
			return cache, false
		}

		data, err := readConfinedFile(root, filepath.Join(path, child.ContentSHA256+".chunk.kld"), int(child.Size))
		if err != nil || uint64(len(data)) != child.Size {
			return cache, false
		}

		digest := sha256.Sum256(data)
		if hex.EncodeToString(digest[:]) != child.ContentSHA256 {
			return cache, false
		}

		if err := validateWorldChunkCacheChild(data, child, &catalog, document, options.NoLightmap, definition.Lightmap); err != nil {
			return cache, false
		}

		cache.chunks = append(cache.chunks, ChunkArtifact{ID: child.ID, Bytes: data,
			ContentSHA256: child.ContentSHA256, EnvelopeVersion: level.EnvelopeVersion})
	}

	return cache, true
}

func writeWorldChunkCache(ctx context.Context, root, directory, key string, result worldMaterialBuild) error {
	var entries []level.SourceEntry

	for _, entry := range result.data {
		if entry.Name == worldchunk.CatalogEntryName || entry.Name == worldchunk.ShadowEntryName {
			entries = append(entries, entry)
		}
	}

	metadata, err := level.EncodeMetadata(level.Metadata{})
	if err != nil {
		return err
	}

	encoded, err := level.Encode(metadata, entries)
	if err != nil {
		return err
	}

	path, err := prepareBakeDirectory(root, directory)
	if err != nil {
		return err
	}

	for _, child := range result.chunks {
		if err := ctx.Err(); err != nil {
			return err
		}

		if err := atomicBakeWrite(path, child.ContentSHA256+".chunk.kld", child.Bytes); err != nil {
			return err
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	// Publish the container last, after every complete immutable child exists.
	return atomicBakeWrite(path, "chunks-"+key+".bin", encoded)
}

func prepareWorldChunkCache(
	ctx context.Context,
	directory string,
	definition manifest,
	document world.Document,
	assets *assetBuild,
) (manifest, worldChunkCache, bool, error) {
	if definition.Chunks == nil || !definition.Chunks.Enabled {
		return definition, worldChunkCache{}, false, nil
	}
	// Validate area selectors before consuming cached chunk ownership.
	if _, err := resolveChunkAreas(definition.Chunks.Areas, &document); err != nil {
		return definition, worldChunkCache{}, false, err
	}

	if definition.Lightmap != nil && definition.Lightmap.Environment != nil && !assets.development {
		// Variant modules have their own dependency-keyed pixel cache. Rebuild the
		// small catalogs so every staged condition child remains accounted for.
		return definition, worldChunkCache{}, false, nil
	}

	key, err := worldChunkCacheKey(ctx, directory, definition, document, assets)
	if err != nil {
		return definition, worldChunkCache{}, false, err
	}

	cache, found := readWorldChunkCache(ctx, assets.projectRoot, directory, key, &document, definition)
	if err := ctx.Err(); err != nil {
		return definition, worldChunkCache{}, false, err
	}

	if !found && assets.development {
		// Editor previews can produce geometry, but never trace lightmap rays.
		// Keep a separate mesh-only cache so it cannot replace a completed bake.
		if definition.Lightmap != nil {
			definition.Lightmap = nil

			key, err := worldChunkCacheKey(ctx, directory, definition, document, assets)
			if err != nil {
				return definition, worldChunkCache{}, false, err
			}

			cache, found = readWorldChunkCache(ctx, assets.projectRoot, directory, key, &document, definition)
		}

		log.Info("Using mesh-only preview chunks with live lighting", "level_name", definition.Level.Name)
	}

	return definition, cache, found, nil
}

func finishWorldChunks(ctx context.Context, directory string, definition manifest, document world.Document,
	result worldMaterialBuild, assets *assetBuild, cache worldChunkCache, found bool) (worldMaterialBuild, error) {
	if found {
		return applyWorldChunks(result, cache.entries, cache.chunks, definition)
	}

	if assets.development && definition.Lightmap != nil && definition.Lightmap.Enabled {
		return worldMaterialBuild{}, fmt.Errorf("development cannot produce lightmap bakes: %w", ErrManifest)
	}

	var pixels *chunkPixelCache

	if definition.Lightmap != nil && definition.Lightmap.Enabled {
		var err error

		pixels, err = prepareChunkPixelCache(assets.projectRoot, directory)
		if err != nil {
			return worldMaterialBuild{}, err
		}
	}

	result, err := buildWorldChunks(ctx, directory, definition, document, result, pixels)
	if err != nil {
		return worldMaterialBuild{}, err
	}

	if definition.Lightmap == nil || definition.Lightmap.Environment == nil {
		if err := writeWorldChunkCache(ctx, assets.projectRoot, directory, cache.key, result); err != nil {
			return worldMaterialBuild{}, fmt.Errorf("write chunk cache: %w", err)
		}
	}

	if pixels != nil {
		log.Info("Chunk lightmap cache", "level_name", definition.Level.Name, "baked", pixels.misses, "reused", pixels.hits)
	}

	return result, nil
}
