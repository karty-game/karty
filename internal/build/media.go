package build

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/karty-game/karty-sdk/format/cartridge"
)

const mediaContentDirectory = "content"

func prepareMediaDirectory(root, extension, temporaryPrefix string) (string, error) {
	directory := filepath.Join(root, mediaContentDirectory)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return "", err
	}

	entries, err := os.ReadDir(directory)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() || (filepath.Ext(entry.Name()) != extension && !strings.HasPrefix(entry.Name(), temporaryPrefix)) {
			continue
		}

		if err := os.Remove(filepath.Join(directory, entry.Name())); err != nil {
			return "", err
		}
	}

	return directory, nil
}

type mediaHashWriter struct {
	destination io.Writer
	complete    hash.Hash
	chunk       hash.Hash
	chunkSize   int
	chunkBytes  int
	chunks      []string
}

func newMediaHashWriter(destination io.Writer, chunkSize int) *mediaHashWriter {
	return &mediaHashWriter{destination: destination, complete: sha256.New(), chunk: sha256.New(), chunkSize: chunkSize}
}

func (writer *mediaHashWriter) Write(data []byte) (int, error) {
	written := 0
	for written < len(data) {
		size := min(writer.chunkSize-writer.chunkBytes, len(data)-written)
		part := data[written : written+size]
		bytesWritten, err := writer.destination.Write(part)

		if bytesWritten > 0 {
			_, _ = writer.complete.Write(part[:bytesWritten])
			_, _ = writer.chunk.Write(part[:bytesWritten])
			writer.chunkBytes += bytesWritten
			written += bytesWritten

			if writer.chunkBytes == writer.chunkSize {
				writer.flushChunk()
			}
		}

		if err == nil && bytesWritten != size {
			err = io.ErrShortWrite
		}

		if err != nil {
			return written, err
		}
	}

	return written, nil
}

func (writer *mediaHashWriter) finish() (string, []string) {
	if writer.chunkBytes > 0 {
		writer.flushChunk()
	}

	return hex.EncodeToString(writer.complete.Sum(nil)), writer.chunks
}

func (writer *mediaHashWriter) flushChunk() {
	writer.chunks = append(writer.chunks, hex.EncodeToString(writer.chunk.Sum(nil)))
	writer.chunk.Reset()
	writer.chunkBytes = 0
}

func writeMediaEnvelope(
	destination io.Writer,
	source io.Reader,
	kind cartridge.MediaKind,
	payloadSize int64,
	chunkSize int,
) (storedSize int64, digest string, chunks []string, err error) {
	storedSize, err = cartridge.MediaEnvelopeSize(kind, uint64(payloadSize))
	if err != nil {
		return 0, "", nil, err
	}

	hashed := newMediaHashWriter(destination, chunkSize)

	wrapped, err := cartridge.NewMediaWriter(hashed, kind, uint64(payloadSize))
	if err != nil {
		return 0, "", nil, err
	}

	if _, err = io.Copy(wrapped, source); err != nil {
		return 0, "", nil, err
	}

	if err = wrapped.Finish(); err != nil {
		return 0, "", nil, err
	}

	digest, chunks = hashed.finish()

	return storedSize, digest, chunks, nil
}
