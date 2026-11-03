//nolint:mnd // XAR and stapled-ticket field offsets are defined by their binary formats.
package toolchain

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"encoding/xml"
	"io"
	"os"
	"strings"

	"github.com/karty-game/karty/internal/contextio"
)

type ktxXARFile struct {
	Name string `xml:"name"`
	Type string `xml:"type"`
	Data *struct {
		Offset   uint64 `xml:"offset"`
		Length   uint64 `xml:"length"`
		Size     uint64 `xml:"size"`
		Encoding struct {
			Style string `xml:"style,attr"`
		} `xml:"encoding"`
	} `xml:"data"`
	Files []ktxXARFile `xml:"file"`
}

// Read flat Apple packages as data. Installer scripts and system installation
// commands are never executed; only payload files and license text are retained.
func extractKTXPackage(ctx context.Context, contents []byte, files *ktxArchiveFiles) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	heap, entries, err := ktxPackageHeap(ctx, contents)
	if err != nil {
		return err
	}

	walker := ktxPackageWalker{heap: heap, files: files, seen: make(map[string]bool)}
	if err := walker.walk(ctx, entries, "", 0); err != nil {
		return err
	}

	if !validKTXPackageTail(heap[walker.end:]) {
		return os.ErrInvalid
	}

	return nil
}

func ktxPackageHeap(ctx context.Context, contents []byte) ([]byte, []ktxXARFile, error) {
	if len(contents) < 28 || len(contents) > materialArchiveLimit || string(contents[:4]) != "xar!" ||
		binary.BigEndian.Uint16(contents[4:]) != 28 ||
		binary.BigEndian.Uint16(contents[6:]) != 1 {
		return nil, nil, os.ErrInvalid
	}

	compressed := binary.BigEndian.Uint64(contents[8:])

	expanded := binary.BigEndian.Uint64(contents[16:])
	if compressed == 0 || compressed > uint64(len(contents)-28) || expanded == 0 || expanded > 1<<20 {
		return nil, nil, os.ErrInvalid
	}

	toc, err := inflateKTXZlib(ctx, contents[28:28+compressed], expanded)
	if err != nil {
		return nil, nil, err
	}

	var archive struct {
		XMLName xml.Name `xml:"xar"`
		TOC     struct {
			Files []ktxXARFile `xml:"file"`
		} `xml:"toc"`
	}
	if err := xml.Unmarshal(toc, &archive); err != nil {
		return nil, nil, err
	}

	return contents[28+compressed:], archive.TOC.Files, nil
}

type ktxPackageWalker struct {
	heap  []byte
	files *ktxArchiveFiles
	seen  map[string]bool
	count int
	end   uint64
}

func (walker *ktxPackageWalker) walk(ctx context.Context, entries []ktxXARFile, prefix string, depth int) error {
	if depth > 16 {
		return os.ErrInvalid
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}

		if err := walker.entry(ctx, entry, prefix, depth); err != nil {
			return err
		}
	}

	return ctx.Err()
}

func (walker *ktxPackageWalker) entry(ctx context.Context, entry ktxXARFile, prefix string, depth int) error {
	walker.count++
	if walker.count > materialEntryLimit || entry.Name == "" || strings.Contains(entry.Name, "/") || !materialMemberPath(entry.Name) {
		return os.ErrInvalid
	}

	name := prefix + entry.Name

	key := strings.ToLower(name)
	if walker.seen[key] {
		return os.ErrInvalid
	}

	walker.seen[key] = true

	if entry.Type == "directory" {
		if entry.Data != nil {
			return os.ErrInvalid
		}

		return walker.walk(ctx, entry.Files, name+"/", depth+1)
	}

	if entry.Type != "file" || entry.Data == nil || len(entry.Files) != 0 {
		return os.ErrInvalid
	}

	data := entry.Data
	if data.Offset > uint64(len(walker.heap)) || data.Length > uint64(len(walker.heap))-data.Offset || data.Size > materialExpandedLimit {
		return os.ErrInvalid
	}

	walker.end = max(walker.end, data.Offset+data.Length)

	encoded := walker.heap[data.Offset : data.Offset+data.Length]
	if entry.Name == "Payload" {
		if data.Encoding.Style != "application/octet-stream" || data.Size != data.Length {
			return os.ErrInvalid
		}

		return extractKTXCPIO(ctx, encoded, walker.files)
	}

	if !strings.HasPrefix(name, "Resources/License") {
		return nil
	}

	if data.Encoding.Style != "application/x-gzip" || data.Size > uint64(materialExpandedLimit-walker.files.bytes) ||
		data.Size > uint64(materialExpandedLimit-walker.files.expanded) {
		return os.ErrInvalid
	}

	walker.files.expanded += int64(data.Size)

	value, err := inflateKTXZlib(ctx, encoded, data.Size)
	if err != nil {
		return err
	}

	return walker.files.add("licenses/"+entry.Name, value, 0o644)
}

// Apple may staple a notarization ticket after the XAR heap. Check the bounded
// wrapper and back-reference; the SDK's archive SHA-256 authenticates the full
// ticket bytes, so extraction needs no Apple certificate/network operation.
func validKTXPackageTail(tail []byte) bool {
	if len(tail) == 0 {
		return true
	}

	if len(tail) < 48 || len(tail) > 1<<20 {
		return false
	}

	footer := tail[len(tail)-16:]

	return string(tail[:4]) == "t8lr" && binary.LittleEndian.Uint32(tail[4:]) == 0x00010001 &&
		binary.LittleEndian.Uint64(tail[8:]) == 0 && string(tail[16:20]) == "s8ch" &&
		binary.LittleEndian.Uint32(tail[20:]) == 1 && string(footer[:4]) == "t8lr" &&
		binary.LittleEndian.Uint32(footer[4:]) == 0x00020001 && binary.LittleEndian.Uint64(footer[8:]) == uint64(len(tail)-32)
}

func inflateKTXZlib(ctx context.Context, contents []byte, size uint64) ([]byte, error) {
	if size > materialExpandedLimit {
		return nil, os.ErrInvalid
	}

	input := bytes.NewReader(contents)

	reader, err := zlib.NewReader(input)
	if err != nil {
		return nil, err
	}

	data, readErr := contextio.ReadAll(ctx, io.LimitReader(reader, int64(size)+1))
	closeErr := reader.Close()

	if readErr != nil {
		return nil, readErr
	}

	if closeErr != nil {
		return nil, closeErr
	}

	if uint64(len(data)) != size || input.Len() != 0 {
		return nil, os.ErrInvalid
	}

	return data, nil
}
