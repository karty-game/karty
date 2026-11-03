//nolint:mnd // The old portable ASCII CPIO profile uses fixed octal fields and POSIX file-type bits.
package toolchain

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"github.com/karty-game/karty/internal/contextio"
)

type ktxCPIOEntry struct {
	name       string
	mode, size uint64
}

func extractKTXCPIO(ctx context.Context, contents []byte, files *ktxArchiveFiles) error {
	input := bytes.NewReader(contents)

	reader, err := gzip.NewReader(input)
	if err != nil {
		return err
	}

	reader.Multistream(false)
	stream := contextio.Reader(ctx, &ktxExpandedReader{input: reader, files: files})
	readErr := readKTXCPIO(ctx, stream, files)
	closeErr := reader.Close()

	if readErr != nil {
		return readErr
	}

	if closeErr != nil {
		return closeErr
	}

	if input.Len() != 0 {
		return os.ErrInvalid
	}

	return ctx.Err()
}

func readKTXCPIO(ctx context.Context, stream io.Reader, files *ktxArchiveFiles) error {
	for range materialEntryLimit {
		entry, err := readKTXCPIOEntry(stream)
		if err != nil {
			return err
		}

		if entry.name == "TRAILER!!!" {
			if entry.size != 0 {
				return os.ErrInvalid
			}

			return finishKTXStream(ctx, stream, 512)
		}

		if err := extractKTXCPIOEntry(ctx, stream, entry, files); err != nil {
			return err
		}
	}

	return os.ErrInvalid
}

func readKTXCPIOEntry(stream io.Reader) (ktxCPIOEntry, error) {
	var header [76]byte
	if _, err := io.ReadFull(stream, header[:]); err != nil {
		return ktxCPIOEntry{}, err
	}

	if string(header[:6]) != "070707" {
		return ktxCPIOEntry{}, os.ErrInvalid
	}

	mode, modeErr := strconv.ParseUint(string(header[18:24]), 8, 32)
	length, lengthErr := strconv.ParseUint(string(header[59:65]), 8, 32)

	size, sizeErr := strconv.ParseUint(string(header[65:76]), 8, 64)
	if modeErr != nil || lengthErr != nil || sizeErr != nil || length < 1 || length > 4096 || size > materialExpandedLimit {
		return ktxCPIOEntry{}, os.ErrInvalid
	}

	name := make([]byte, length)
	if _, err := io.ReadFull(stream, name); err != nil {
		return ktxCPIOEntry{}, err
	}

	if name[len(name)-1] != 0 || bytes.IndexByte(name[:len(name)-1], 0) >= 0 {
		return ktxCPIOEntry{}, os.ErrInvalid
	}

	return ktxCPIOEntry{name: string(name[:len(name)-1]), mode: mode, size: size}, nil
}

func extractKTXCPIOEntry(ctx context.Context, stream io.Reader, entry ktxCPIOEntry, files *ktxArchiveFiles) error {
	name := strings.TrimPrefix(entry.name, "./")
	if entry.mode&0o170000 == 0o040000 {
		if entry.size != 0 || (name != "." && !materialMemberPath(name)) {
			return os.ErrInvalid
		}

		return nil
	}

	if !materialMemberPath(name) || entry.size > uint64(materialExpandedLimit-files.bytes) ||
		entry.size > uint64(materialExpandedLimit-files.expanded) {
		return os.ErrInvalid
	}

	relative, found := strings.CutPrefix(name, "usr/local/")
	if !found || !materialMemberPath(relative) {
		return os.ErrInvalid
	}

	kind := entry.mode & 0o170000
	if (kind != 0o100000 && kind != 0o120000) || (kind == 0o120000 && entry.size > 4096) {
		return os.ErrInvalid
	}

	value := make([]byte, int(entry.size))
	if _, err := io.ReadFull(stream, value); err != nil {
		return err
	}

	if kind == 0o100000 {
		return files.add(relative, value, fs.FileMode(entry.mode)&0o777)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	target, err := ktxAliasTarget(relative, string(value))
	if err != nil {
		return err
	}

	return files.record(relative, ktxArchiveFile{link: target})
}
