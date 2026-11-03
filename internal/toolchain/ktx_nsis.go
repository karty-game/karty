//nolint:mnd // NSIS3 binary header and instruction offsets are fixed by the published format.
package toolchain

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"io/fs"
	"strings"
	"unicode/utf16"

	"github.com/karty-game/karty/internal/contextio"
	"github.com/ulikunitz/xz/lzma"
)

const (
	nsisFirstHeaderSize = 28
	nsisInstructionSize = 28
	nsisHeaderLimit     = 4 << 20
	nsisDictionaryLimit = 8 << 20
	nsisFileLimit       = 64 << 20
	nsisSetOutPath      = 11
	nsisExtractFile     = 20
	nsisAssignVariable  = 25
	nsisInstallDir      = 21
	nsisOutDir          = 22
	nsisPluginsDir      = 26
	nsisSavedOutDir     = 31
)

const errKTXNSIS staticError = "unsupported or invalid KTX NSIS archive"

type nsisFile struct {
	name   string
	offset uint32
}

// extractKTXNSIS reads the pinned official Unicode NSIS3/non-solid LZMA KTX
// package as an archive. It never executes the PE stub or installer operations.
// Only static installation-root file declarations are materialized; temporary
// installer plugins are omitted. Registry, PATH, shortcuts, calls, uninstallers
// and all other script operations are never interpreted or performed.
// Format reference: NSIS Source/exehead/{fileform.h,fileform.c,state.h}.
func extractKTXNSIS(ctx context.Context, contents []byte, emit func(string, []byte, fs.FileMode) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if emit == nil || len(contents) < 512 || len(contents) > materialArchiveLimit || !bytes.Equal(contents[:2], []byte("MZ")) {
		return errKTXNSIS
	}

	pe := uint64(binary.LittleEndian.Uint32(contents[60:]))
	if pe > uint64(len(contents)-4) || !bytes.Equal(contents[pe:pe+4], []byte{'P', 'E', 0, 0}) {
		return errKTXNSIS
	}

	first := -1

	for offset := 512; offset+nsisFirstHeaderSize <= min(len(contents), nsisHeaderLimit); offset += 512 {
		if bytes.Equal(
			contents[offset+4:offset+20],
			[]byte{0xef, 0xbe, 0xad, 0xde, 'N', 'u', 'l', 'l', 's', 'o', 'f', 't', 'I', 'n', 's', 't'},
		) {
			first = offset

			break
		}
	}

	if first < 0 {
		return errKTXNSIS
	}

	header := contents[first : first+nsisFirstHeaderSize]
	expected := uint64(binary.LittleEndian.Uint32(header[20:]))

	following := uint64(binary.LittleEndian.Uint32(header[24:]))
	if binary.LittleEndian.Uint32(header) != 0 || expected < 68 || expected > nsisHeaderLimit ||
		following < nsisFirstHeaderSize+8 || following > uint64(len(contents)-first) {
		return errKTXNSIS
	}

	end := uint64(first) + following
	// NSIS excludes its first 512 stub bytes and any appended PE certificate.
	if crc32.ChecksumIEEE(contents[512:end-4]) != binary.LittleEndian.Uint32(contents[end-4:end]) {
		return errKTXNSIS
	}

	storage := contents[uint64(first)+nsisFirstHeaderSize : end-4]

	decoded, next, err := nsisBlock(ctx, storage, 0, nsisHeaderLimit)
	if err != nil {
		return err
	}

	if uint64(len(decoded)) != expected {
		return errKTXNSIS
	}

	files, err := nsisFiles(decoded)
	if err != nil {
		return err
	}

	storage = storage[next:]

	var total int64
	for _, file := range files {
		data, _, err := nsisBlock(ctx, storage, uint64(file.offset), min(nsisFileLimit, materialExpandedLimit-total))
		if err != nil {
			return fmt.Errorf("KTX NSIS member %q: %w", file.name, err)
		}

		total += int64(len(data))
		if err := emit(file.name, data, 0o644); err != nil {
			return err
		}
	}

	return nil
}

// nsisBlock validates and decodes one independent NSIS block. NSIS stores a
// high-bit compression marker followed by LZMA properties and an EOS stream;
// reconstruct the classic .lzma size field without changing its compressed data.
func nsisBlock(ctx context.Context, storage []byte, offset uint64, limit int64) ([]byte, uint64, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}

	if limit < 0 || offset > uint64(len(storage)) || uint64(len(storage))-offset < 4 {
		return nil, 0, errKTXNSIS
	}

	word := binary.LittleEndian.Uint32(storage[offset:])
	length := uint64(word & 0x7fffffff)

	start := offset + 4
	if length > uint64(len(storage))-start {
		return nil, 0, errKTXNSIS
	}

	payload := storage[start : start+length]
	if word&0x80000000 == 0 {
		if length > uint64(limit) {
			return nil, 0, errKTXNSIS
		}

		return payload, start + length, nil
	}

	if len(payload) < 6 || payload[0] != 0x5d || binary.LittleEndian.Uint32(payload[1:5]) > nsisDictionaryLimit {
		return nil, 0, errKTXNSIS
	}

	var classic [13]byte
	copy(classic[:5], payload[:5])

	for i := 5; i < len(classic); i++ {
		classic[i] = 0xff
	}

	input := nsisLZMAInput{header: bytes.NewReader(classic[:]), stream: bytes.NewReader(payload[5:])}

	reader, err := (lzma.ReaderConfig{DictCap: nsisDictionaryLimit}).NewReader(&input)
	if err != nil {
		return nil, 0, errKTXNSIS
	}

	data, err := io.ReadAll(io.LimitReader(contextio.Reader(ctx, reader), limit+1))
	if err != nil {
		return nil, 0, err
	}

	if int64(len(data)) > limit || !reader.EOSMarker() || input.stream.Len() != 0 {
		return nil, 0, errKTXNSIS
	}

	return data, start + length, nil
}

// Implement ByteReader as well as Reader so the LZMA decoder does not buffer
// beyond its EOS marker and conceal trailing compressed bytes.
type nsisLZMAInput struct{ header, stream *bytes.Reader }

func (r *nsisLZMAInput) Read(p []byte) (int, error) {
	if r.header.Len() != 0 {
		return r.header.Read(p)
	}

	return r.stream.Read(p)
}
func (r *nsisLZMAInput) ReadByte() (byte, error) {
	if r.header.Len() != 0 {
		return r.header.ReadByte()
	}

	return r.stream.ReadByte()
}

func nsisFiles(header []byte) ([]nsisFile, error) {
	if len(header) < 68 {
		return nil, errKTXNSIS
	}
	// The pinned x86 Unicode installer stores eight 8-byte block descriptors.
	entries := uint64(binary.LittleEndian.Uint32(header[20:]))
	count := uint64(binary.LittleEndian.Uint32(header[24:]))
	stringsAt := uint64(binary.LittleEndian.Uint32(header[28:]))

	stringsEnd := uint64(binary.LittleEndian.Uint32(header[36:]))
	if count == 0 || count > materialEntryLimit || entries < 68 || entries > uint64(len(header)) ||
		count*nsisInstructionSize > uint64(len(header))-entries || entries+count*nsisInstructionSize != stringsAt ||
		stringsAt >= stringsEnd || stringsEnd > uint64(len(header)) || (stringsEnd-stringsAt)%2 != 0 {
		return nil, errKTXNSIS
	}

	table := header[stringsAt:stringsEnd]
	collector := nsisFileCollector{table: table, seen: make(map[string]bool)}

	for index := range count {
		instruction := header[entries+index*nsisInstructionSize : entries+(index+1)*nsisInstructionSize]
		if err := collector.instruction(instruction); err != nil {
			return nil, err
		}
	}

	if !collector.seen["bin/ktx.exe"] || !collector.seen["bin/ktx.dll"] {
		return nil, errKTXNSIS
	}

	return collector.files, nil
}

type nsisFileCollector struct {
	table               []byte
	files               []nsisFile
	seen                map[string]bool
	directory, saved    string
	active, savedActive bool
}

func (collector *nsisFileCollector) instruction(instruction []byte) error {
	operation := binary.LittleEndian.Uint32(instruction)
	first := binary.LittleEndian.Uint32(instruction[4:])
	second := binary.LittleEndian.Uint32(instruction[8:])

	switch operation {
	case 1:
		collector.active, collector.savedActive = false, false
	case nsisAssignVariable:
		if first == nsisSavedOutDir {
			return collector.assign(instruction, second)
		}
	case nsisSetOutPath:
		if second == 0 {
			return nil
		}

		if second != 1 {
			return errKTXNSIS
		}

		return collector.directoryPath(first)
	case nsisExtractFile:
		return collector.file(second, binary.LittleEndian.Uint32(instruction[12:]))
	}

	return nil
}

func (collector *nsisFileCollector) assign(instruction []byte, index uint32) error {
	text, err := nsisString(collector.table, index)
	if err != nil {
		return err
	}

	if text.variable != nsisOutDir || text.suffix != "" || binary.LittleEndian.Uint32(instruction[12:]) != 0 ||
		binary.LittleEndian.Uint32(instruction[16:]) != 0 {
		collector.savedActive = false

		return nil
	}

	collector.saved, collector.savedActive = collector.directory, collector.active

	return nil
}

func (collector *nsisFileCollector) directoryPath(index uint32) error {
	text, err := nsisString(collector.table, index)
	if err != nil {
		return err
	}

	collector.directory, collector.active, err = nsisPath(
		text,
		collector.directory,
		collector.active,
		collector.saved,
		collector.savedActive,
		true,
	)

	return err
}

func (collector *nsisFileCollector) file(index, offset uint32) error {
	text, err := nsisString(collector.table, index)
	if err != nil {
		return err
	}

	name, inside, err := nsisPath(text, collector.directory, collector.active, collector.saved, collector.savedActive, false)
	if err != nil {
		return err
	}

	if !inside {
		return nil
	}

	key := strings.ToLower(name)
	if collector.seen[key] || len(collector.files) >= materialEntryLimit {
		return errKTXNSIS
	}

	collector.seen[key] = true
	collector.files = append(collector.files, nsisFile{name: name, offset: offset})

	return nil
}

type nsisText struct {
	variable int
	suffix   string
}

func nsisString(table []byte, index uint32) (nsisText, error) {
	offset := uint64(index) * 2
	if offset >= uint64(len(table)) {
		return nsisText{}, errKTXNSIS
	}

	var units []uint16

	ended := false

	for ; offset+2 <= uint64(len(table)) && len(units) < 4096; offset += 2 {
		unit := binary.LittleEndian.Uint16(table[offset:])
		if unit == 0 {
			ended = true

			break
		}

		units = append(units, unit)
	}

	if !ended {
		return nsisText{}, errKTXNSIS
	}

	text := nsisText{variable: -1}

	if len(units) >= 2 && units[0] == 3 {
		code := units[1]
		if code&0x8080 != 0x8080 {
			return nsisText{}, errKTXNSIS
		}

		text.variable = int((code>>8&0x7f)<<7 | code&0x7f)
		units = units[2:]
	}

	for position := 0; position < len(units); position++ {
		unit := units[position]
		if unit < 32 {
			return nsisText{}, errKTXNSIS
		}

		if utf16.IsSurrogate(rune(unit)) {
			if unit > 0xdbff || position+1 == len(units) || units[position+1] < 0xdc00 || units[position+1] > 0xdfff {
				return nsisText{}, errKTXNSIS
			}

			position++
		}
	}

	text.suffix = strings.ReplaceAll(string(utf16.Decode(units)), "\\", "/")

	return text, nil
}

func nsisPath(text nsisText, directory string, active bool, saved string, savedActive bool, allowRoot bool) (string, bool, error) {
	base := ""

	suffix := text.suffix
	switch text.variable {
	case nsisInstallDir:
	case nsisSavedOutDir:
		if !savedActive {
			return "", false, errKTXNSIS
		}

		base = saved
	case nsisOutDir:
		if !active {
			return "", false, errKTXNSIS
		}

		base = directory
	case nsisPluginsDir:
		if !materialMemberPath(strings.TrimPrefix(suffix, "/")) {
			return "", false, errKTXNSIS
		}

		return "", false, nil
	case -1:
		if !active || !materialMemberPath(suffix) {
			return "", false, errKTXNSIS
		}

		base = directory
	default:
		return "", false, errKTXNSIS
	}

	if text.variable != -1 {
		if suffix != "" && !strings.HasPrefix(suffix, "/") {
			return "", false, errKTXNSIS
		}

		suffix = strings.TrimPrefix(suffix, "/")
	}

	name := suffix
	if base != "" && suffix != "" {
		name = base + "/" + suffix
	} else if base != "" {
		name = base
	}

	if name == "" && allowRoot {
		return "", true, nil
	}

	if !materialMemberPath(name) {
		return "", false, errKTXNSIS
	}

	return name, true, nil
}
