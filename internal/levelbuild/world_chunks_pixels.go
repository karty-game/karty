package levelbuild

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	worldlightmapbake "github.com/karty-game/karty-sdk/bake/worldlightmap"
	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

// Pixel entries are optional build accelerators. Complete world/chunk artifacts
// retain their independent full-document hashes and runtime validation.
const (
	chunkPixelCacheDirectory = "pixels"
	chunkPixelCacheSchema    = "karty-cli-chunk-pixels@1"
	chunkPixelCacheEntry     = "@cache/lightmap"
	maxChunkPixelCacheBytes  = worldlightmap.MaxPrebakeImageSize + 4096
)

type chunkPixelCache struct {
	root, relative, directory string
	hits, misses              int
	baker                     *worldlightmapbake.Baker
}

func prepareChunkPixelCache(root, directory string) (*chunkPixelCache, error) {
	parent, err := prepareBakeDirectory(root, directory)
	if err != nil {
		return nil, err
	}

	relative, err := automaticBakePath(root, directory)
	if err != nil {
		return nil, err
	}

	path := filepath.Join(parent, chunkPixelCacheDirectory)
	// #nosec G703 -- parent is confined by prepareBakeDirectory; the child name is constant.
	if err := os.Mkdir(path, 0o755); err != nil && !os.IsExist(err) {
		return nil, err
	}

	// #nosec G703 -- check the constant child under the confined bake directory without following symlinks.
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("chunk pixel cache must be a confined directory: %w", ErrSource)
	}

	return &chunkPixelCache{root: root, relative: filepath.Join(relative, chunkPixelCacheDirectory), directory: path}, nil
}

func (cache *chunkPixelCache) read(key, producer string, rgbmRange float64) ([]byte, bool) {
	if !cache.enabled() || !validPixelCacheDigest(key) {
		return nil, false
	}

	encoded, err := readConfinedFile(cache.root, filepath.Join(cache.relative, key+".bin"), maxChunkPixelCacheBytes)
	if err != nil {
		return nil, false
	}

	envelope, err := level.Decode(encoded)
	if err != nil || len(envelope.Entries) != 1 || envelope.Entries[0].Name != chunkPixelCacheEntry ||
		envelope.Entries[0].Kind != level.EntryData || envelope.Entries[0].Length > worldlightmap.MaxPrebakeImageSize {
		return nil, false
	}

	metadata, err := level.DecodeMetadata(envelope.Metadata)
	actualRange, rangeValid := metadata["rgbmRange"].AsFloat()

	imageDigest, digestValid := metadata.String("imageSha256")
	if err != nil || len(metadata) != 5 || !chunkMetadataMatches(metadata, "schema", chunkPixelCacheSchema) ||
		!chunkMetadataMatches(metadata, "dependencySha256", key) || !chunkMetadataMatches(metadata, "producer", producer) ||
		!rangeValid || actualRange != rgbmRange || !digestValid || !validPixelCacheDigest(imageDigest) {
		return nil, false
	}

	image, _, found := envelope.Read(chunkPixelCacheEntry, 0, worldlightmap.MaxPrebakeImageSize)
	if !found {
		return nil, false
	}

	digest := sha256.Sum256(image)
	if hex.EncodeToString(digest[:]) != imageDigest {
		return nil, false
	}

	imageMetadata, err := qoi.Validate(image)
	if err != nil || imageMetadata.Channels != qoi.ChannelsRGBA || imageMetadata.Colorspace != qoi.ColorspaceLinear {
		return nil, false
	}

	return image, true
}

func (cache *chunkPixelCache) write(key, producer string, rgbmRange float64, image []byte) error {
	if !cache.enabled() {
		return nil
	}

	if !validPixelCacheDigest(key) || len(image) > worldlightmap.MaxPrebakeImageSize {
		return ErrSource
	}

	digest := sha256.Sum256(image)

	metadata, err := level.EncodeMetadata(level.Metadata{
		"schema":           level.StringValue(chunkPixelCacheSchema),
		"dependencySha256": level.StringValue(key),
		"producer":         level.StringValue(producer),
		"rgbmRange":        level.FloatValue(rgbmRange),
		"imageSha256":      level.StringValue(hex.EncodeToString(digest[:])),
	})
	if err != nil {
		return err
	}

	encoded, err := level.Encode(metadata, []level.SourceEntry{{Name: chunkPixelCacheEntry, Kind: level.EntryData, Data: image}})
	if err != nil || len(encoded) > maxChunkPixelCacheBytes {
		return ErrSource
	}

	return atomicBakeWrite(cache.directory, key+".bin", encoded)
}

func (cache *chunkPixelCache) enabled() bool {
	return cache != nil && cache.directory != ""
}

func validPixelCacheDigest(value string) bool {
	if len(value) != hex.EncodedLen(sha256.Size) {
		return false
	}

	for _, c := range value {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}

	return true
}

// One task owns the document/images and releases shared transport at completion.
// A cache without a disk directory only shares in-memory preparation.
func prepareChunkBaker(cache *chunkPixelCache, document world.Document, materials []worldlightmapbake.Material,
	enabled bool) (*chunkPixelCache, func()) {
	if !enabled {
		return cache, func() {}
	}

	if cache == nil {
		cache = &chunkPixelCache{}
	}

	cache.baker = worldlightmapbake.NewBaker(document, materials)

	return cache, func() { cache.baker = nil }
}
