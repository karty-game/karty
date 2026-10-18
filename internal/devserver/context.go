package devserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
)

const contextChunkBytes = 32 * 1024

func readSnapshotContents(ctx context.Context, reader io.Reader, limit int64) ([]byte, error) {
	var output bytes.Buffer

	buffer := make([]byte, contextChunkBytes)
	reader = io.LimitReader(reader, limit+1)

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		count, err := reader.Read(buffer)

		if canceled := ctx.Err(); canceled != nil {
			return nil, canceled
		}

		if int64(output.Len()+count) > limit {
			return nil, ErrSnapshot
		}

		_, _ = output.Write(buffer[:count])
		if errors.Is(err, io.EOF) {
			return output.Bytes(), nil
		}

		if err != nil {
			return nil, err
		}
	}
}

func digestContext(ctx context.Context, data []byte) (string, error) {
	hash := sha256.New()

	for offset := 0; offset < len(data); offset += contextChunkBytes {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		_, _ = hash.Write(data[offset:min(offset+contextChunkBytes, len(data))])
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func cloneContext(ctx context.Context, data []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	result := make([]byte, len(data))
	for offset := 0; offset < len(data); offset += contextChunkBytes {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		copy(result[offset:min(offset+contextChunkBytes, len(data))], data[offset:min(offset+contextChunkBytes, len(data))])
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
