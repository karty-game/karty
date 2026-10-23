package levelbuild

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"

	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/worldchunk"
)

func readWorldChunkCacheModules(ctx context.Context, root, path string, descriptors []worldchunk.Chunk,
	options worldchunk.Options, groups []int,
) ([]ChunkArtifact, error) {
	chunks := make([]ChunkArtifact, 0, len(descriptors))
	for _, child := range descriptors {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if !chunkDescriptorWithinOptions(child, options, groups) {
			return nil, worldchunk.ErrChunk
		}

		data, err := readConfinedFile(root, filepath.Join(path, child.ContentSHA256+".chunk.kld"), int(child.Size))
		if err != nil || uint64(len(data)) != child.Size {
			return nil, worldchunk.ErrChunk
		}

		digest := sha256.Sum256(data)
		if hex.EncodeToString(digest[:]) != child.ContentSHA256 {
			return nil, worldchunk.ErrChunk
		}

		chunks = append(chunks, ChunkArtifact{ID: child.ID, Bytes: data,
			ContentSHA256: child.ContentSHA256, EnvelopeVersion: level.EnvelopeVersion})
	}

	return chunks, nil
}
