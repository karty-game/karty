package levelbuild

import (
	"context"
	"path/filepath"

	worldlightmapbake "github.com/karty-game/karty-sdk/bake/worldlightmap"
	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

func fixedChunkLighting(ctx context.Context, document world.Document, layout worldlightmap.Layout,
	materials []worldlightmapbake.Material, options worldlightmapbake.Options, cache *chunkPixelCache, compact bool,
) (worldlightmap.FixedLighting, []byte, error) {
	key, err := worldlightmapbake.FixedInputDigest(document, layout, materials, options)
	if err != nil {
		return worldlightmap.FixedLighting{}, nil, err
	}

	if lighting, image, found := readFixedChunkPixels(cache, key, layout, &document, compact); found {
		cache.hits++

		return lighting, image, nil
	}

	var result worldlightmapbake.FixedResult
	if cache != nil && cache.baker != nil {
		result, err = cache.baker.BakeFixed(ctx, layout, options)
	} else {
		result, err = worldlightmapbake.BakeFixed(ctx, document, layout, materials, options)
	}

	if err != nil {
		return worldlightmap.FixedLighting{}, nil, err
	}

	lighting := result.Lighting

	var image []byte

	var uniform bool
	if compact {
		lighting, uniform, err = worldlightmapbake.CompactFixedLighting(layout, result.Image, lighting)
		if err != nil {
			return worldlightmap.FixedLighting{}, nil, err
		}
	}

	if !uniform {
		image, _, err = qoi.Encode(result.Image, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceLinear})
		if err != nil {
			return worldlightmap.FixedLighting{}, nil, err
		}
	}

	lighting, err = worldlightmap.NewFixedLighting(layout, &document, image, lighting)
	if err != nil {
		return worldlightmap.FixedLighting{}, nil, err
	}

	if err := ctx.Err(); err != nil {
		return worldlightmap.FixedLighting{}, nil, err
	}

	if err := writeFixedChunkPixels(cache, key, lighting, image, compact); err != nil {
		return worldlightmap.FixedLighting{}, nil, err
	}

	if cache != nil {
		cache.misses++
	}

	return lighting, image, nil
}

func readFixedChunkPixels(cache *chunkPixelCache, key string, layout worldlightmap.Layout,
	document *world.Document, compact bool,
) (worldlightmap.FixedLighting, []byte, bool) {
	if !cache.enabled() || !validPixelCacheDigest(key) {
		return worldlightmap.FixedLighting{}, nil, false
	}

	encoded, err := readConfinedFile(cache.root, filepath.Join(cache.relative, fixedPixelCacheName(key, compact)),
		worldlightmap.MaxFixedLightingSize+worldlightmap.MaxPrebakeImageSize+4096)
	if err != nil {
		return worldlightmap.FixedLighting{}, nil, false
	}

	envelope, err := level.Decode(encoded)
	if err != nil || len(envelope.Entries) < 1 || len(envelope.Entries) > 2 {
		return worldlightmap.FixedLighting{}, nil, false
	}

	for _, entry := range envelope.Entries {
		if entry.Kind != level.EntryData || entry.Name != worldlightmap.FixedEntryName && entry.Name != worldlightmap.FixedImageEntryName {
			return worldlightmap.FixedLighting{}, nil, false
		}
	}

	manifest, _, found := envelope.Read(worldlightmap.FixedEntryName, 0, worldlightmap.MaxFixedLightingSize)

	image, _, imageFound := envelope.Read(worldlightmap.FixedImageEntryName, 0, worldlightmap.MaxPrebakeImageSize)

	if !found {
		return worldlightmap.FixedLighting{}, nil, false
	}

	lighting, err := worldlightmap.DecodeFixedLightingArtifact(manifest, image, layout)
	if err != nil || lighting.InputSHA256 != key || !fixedImageEntryMatches(lighting, image, imageFound) ||
		worldlightmap.ValidateFixedLighting(lighting, layout, document) != nil {
		return worldlightmap.FixedLighting{}, nil, false
	}

	return lighting, image, true
}

func writeFixedChunkPixels(cache *chunkPixelCache, key string, lighting worldlightmap.FixedLighting, image []byte, compact bool) error {
	if !cache.enabled() {
		return nil
	}

	if !validPixelCacheDigest(key) || lighting.InputSHA256 != key {
		return ErrSource
	}

	manifest, err := worldlightmap.EncodeFixedLighting(lighting)
	if err != nil {
		return err
	}

	metadata, err := level.EncodeMetadata(level.Metadata{})
	if err != nil {
		return err
	}

	entries := []level.SourceEntry{{Name: worldlightmap.FixedEntryName, Kind: level.EntryData, Data: manifest}}
	if len(image) != 0 {
		entries = append(entries, level.SourceEntry{Name: worldlightmap.FixedImageEntryName, Kind: level.EntryData, Data: image})
	}

	encoded, err := level.Encode(metadata, entries)
	if err != nil {
		return err
	}

	return atomicBakeWrite(cache.directory, fixedPixelCacheName(key, compact), encoded)
}

func fixedPixelCacheName(key string, compact bool) string {
	if compact {
		return "fixed-uniform-v1-" + key + ".bin"
	}

	return "fixed-" + key + ".bin"
}

func fixedImageEntryMatches(lighting worldlightmap.FixedLighting, image []byte, present bool) bool {
	return (lighting.ImageBytes != 0) == present && len(image) == lighting.ImageBytes
}
