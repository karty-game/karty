package toolchain

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/karty-game/karty/internal/contextio"
)

func extractMaterialArchive(ctx context.Context, destination string, contents []byte) error {
	writer := &materialArchiveWriter{
		destination: destination,
		seen:        make(map[string]bool),
	}

	return extractMaterialZip(ctx, contents, writer)
}

// materialArchiveWriter tracks bounds and member uniqueness across the full archive.
type materialArchiveWriter struct {
	destination string
	seen        map[string]bool
	total       int64
	entries     int
}

func (writer *materialArchiveWriter) write(ctx context.Context, name string, directory bool, size int64, source io.Reader) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	writer.entries++

	if directory {
		name = strings.TrimSuffix(name, "/")
	}

	if writer.entries > materialEntryLimit || !materialMemberPath(name) || writer.seen[name] ||
		size < 0 || size > materialExpandedLimit-writer.total {
		return fmt.Errorf("invalid or oversized material archive member %q: %w", name, os.ErrInvalid)
	}

	writer.seen[name] = true
	writer.total += size

	target := filepath.Join(writer.destination, filepath.FromSlash(name))
	if directory {
		return os.MkdirAll(target, 0o750)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return err
	}

	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}

	_, copyErr := contextio.CopyN(ctx, file, source, size)
	closeErr := file.Close()

	if copyErr != nil {
		return copyErr
	}

	return closeErr
}

func extractMaterialZip(ctx context.Context, contents []byte, writer *materialArchiveWriter) error {
	reader, err := zip.NewReader(bytes.NewReader(contents), int64(len(contents)))
	if err != nil {
		return err
	}

	if len(reader.File) > materialEntryLimit {
		return fmt.Errorf("too many material archive members: %w", os.ErrInvalid)
	}

	for _, entry := range reader.File {
		if err := ctx.Err(); err != nil {
			return err
		}

		if err := extractMaterialZipEntry(ctx, entry, writer); err != nil {
			return err
		}
	}

	return nil
}

func extractMaterialZipEntry(ctx context.Context, entry *zip.File, writer *materialArchiveWriter) error {
	if (!entry.Mode().IsRegular() && !entry.Mode().IsDir()) || entry.UncompressedSize64 > materialExpandedLimit {
		return fmt.Errorf("non-regular or oversized material ZIP member: %w", os.ErrInvalid)
	}

	input, err := entry.Open()
	if err != nil {
		return err
	}

	writeErr := writer.write(ctx, entry.Name, entry.Mode().IsDir(), int64(entry.UncompressedSize64), input)
	// Read through EOF to enforce ZIP CRC and declared size.
	if writeErr == nil {
		var extra [1]byte

		readCount, eofErr := contextio.Reader(ctx, input).Read(extra[:])
		if canceled := ctx.Err(); canceled != nil {
			_ = input.Close()

			return canceled
		}

		if readCount != 0 || eofErr != io.EOF {
			writeErr = fmt.Errorf("invalid material ZIP contents: %w", os.ErrInvalid)
		}
	}

	closeErr := input.Close()

	if writeErr != nil {
		return writeErr
	}

	return closeErr
}
