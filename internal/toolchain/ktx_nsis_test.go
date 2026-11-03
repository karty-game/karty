package toolchain

import (
	"bytes"
	"context"
	"crypto/sha256"
	"debug/pe"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"hash/crc32"
	"io/fs"
	"os"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/ulikunitz/xz/lzma"
)

func nsisTestBlock(t *testing.T, data []byte, compressed bool) []byte {
	t.Helper()

	payload := data
	word := uint32(len(data))

	if compressed {
		var buffer bytes.Buffer

		writer, err := (lzma.WriterConfig{DictCap: 1 << 16, EOSMarker: true}).NewWriter(&buffer)
		if err != nil {
			t.Fatal(err)
		}

		if _, err = writer.Write(data); err != nil {
			t.Fatal(err)
		}

		if err = writer.Close(); err != nil {
			t.Fatal(err)
		}

		classic := buffer.Bytes()
		payload = append(append([]byte(nil), classic[:5]...), classic[13:]...)
		word = uint32(len(payload)) | 0x80000000
	}

	return append(binary.LittleEndian.AppendUint32(nil, word), payload...)
}

func nsisTestHeader(t *testing.T, name string) ([]byte, []byte) {
	t.Helper()

	var strings []uint16

	str := func(variable int, suffix string) uint32 {
		index := uint32(len(strings))
		if variable >= 0 {
			strings = append(strings, 3, uint16(0x8080|(variable&0x7f)|((variable&0x3f80)<<1)))
		}

		strings = append(strings, utf16.Encode([]rune(suffix))...)
		strings = append(strings, 0)

		return index
	}
	root, out := str(nsisInstallDir, ""), str(nsisOutDir, "")
	bin := str(nsisSavedOutDir, "\\bin")
	exe, dll := str(-1, name), str(-1, "ktx.dll")
	plugin := str(nsisPluginsDir, "\\System.dll")
	share, license := str(nsisInstallDir, "\\share\\licenses"), str(-1, "LICENSE.txt")
	first := nsisTestBlock(t, []byte("MZ executable fixture"), true)
	second := nsisTestBlock(t, []byte("MZ dependency fixture"), false)
	third := nsisTestBlock(t, []byte("upstream license fixture"), true)
	payload := append(append(append([]byte(nil), first...), second...), third...)
	entries := [][7]uint32{
		{nsisExtractFile, 0, plugin, 0xffffffff}, // Unrelated installer plugin never extracted.
		{nsisSetOutPath, root, 1},
		{nsisAssignVariable, nsisSavedOutDir, out},
		{nsisSetOutPath, bin, 1},
		{nsisExtractFile, 0, exe, 0},
		{nsisExtractFile, 0, dll, uint32(len(first))},
		{1},
		{nsisSetOutPath, share, 1},
		{nsisExtractFile, 0, license, uint32(len(first) + len(second))},
	}

	const entryStart = 68

	header := make([]byte, entryStart+len(entries)*nsisInstructionSize+len(strings)*2)
	stringStart := entryStart + len(entries)*nsisInstructionSize
	binary.LittleEndian.PutUint32(header[20:], entryStart)
	binary.LittleEndian.PutUint32(header[24:], uint32(len(entries)))
	binary.LittleEndian.PutUint32(header[28:], uint32(stringStart))
	binary.LittleEndian.PutUint32(header[36:], uint32(len(header)))

	for i, entry := range entries {
		for j, v := range entry {
			binary.LittleEndian.PutUint32(header[entryStart+i*28+j*4:], v)
		}
	}

	for i, v := range strings {
		binary.LittleEndian.PutUint16(header[stringStart+i*2:], v)
	}

	return header, payload
}

func nsisTestArchive(t *testing.T, header, payload []byte) []byte {
	t.Helper()
	compressed := nsisTestBlock(t, header, true)
	contents := make([]byte, 512+28+len(compressed)+len(payload)+4)
	copy(contents, "MZ")
	binary.LittleEndian.PutUint32(contents[60:], 64)
	copy(contents[64:], []byte{'P', 'E', 0, 0})
	first := contents[512:540]
	copy(first[4:], []byte{0xef, 0xbe, 0xad, 0xde, 'N', 'u', 'l', 'l', 's', 'o', 'f', 't', 'I', 'n', 's', 't'})
	binary.LittleEndian.PutUint32(first[20:], uint32(len(header)))
	binary.LittleEndian.PutUint32(first[24:], uint32(28+len(compressed)+len(payload)+4))
	copy(contents[540:], compressed)
	copy(contents[540+len(compressed):], payload)
	binary.LittleEndian.PutUint32(contents[len(contents)-4:], crc32.ChecksumIEEE(contents[512:len(contents)-4]))

	return contents
}

func TestKTXNSISExtractsStaticRootFilesWithoutInstallerExecution(t *testing.T) {
	t.Parallel()
	header, payload := nsisTestHeader(t, "ktx.exe")
	archive := nsisTestArchive(t, header, payload)
	files := map[string][]byte{}

	err := extractKTXNSIS(t.Context(), archive, func(name string, data []byte, mode fs.FileMode) error {
		if mode != 0o644 {
			t.Fatal("unexpected mode", mode)
		}

		files[name] = append([]byte(nil), data...)

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 3 || string(files["bin/ktx.exe"]) != "MZ executable fixture" ||
		string(files["bin/ktx.dll"]) != "MZ dependency fixture" ||
		string(files["share/licenses/LICENSE.txt"]) != "upstream license fixture" {
		t.Fatal("wrong files", files)
	}
}

func TestKTXNSISRejectsEscapesCorruptionAndUnboundedHeaders(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"../ktx.exe", "/absolute", "C:\\outside.exe", "CON.txt", "bad\\..\\ktx.exe", "ktx.dll"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			header, payload := nsisTestHeader(t, name)
			if extractKTXNSIS(
				t.Context(),
				nsisTestArchive(t, header, payload),
				func(string, []byte, fs.FileMode) error { return nil },
			) == nil {
				t.Fatal("unsafe or duplicate path accepted")
			}
		})
	}

	header, payload := nsisTestHeader(t, "ktx.exe")
	for _, mutate := range []func([]byte){
		func(b []byte) { binary.LittleEndian.PutUint32(b[20:], 0xffffffff) },
		func(b []byte) { binary.LittleEndian.PutUint32(b[24:], 0xffffffff) },
		func(b []byte) { binary.LittleEndian.PutUint32(b[28:], 0xffffffff) },
		func(b []byte) { binary.LittleEndian.PutUint32(b[36:], 0xffffffff) },
	} {
		bad := append([]byte(nil), header...)
		mutate(bad)

		if extractKTXNSIS(t.Context(), nsisTestArchive(t, bad, payload), func(string, []byte, fs.FileMode) error { return nil }) == nil {
			t.Fatal("invalid header block accepted")
		}
	}

	bad := nsisTestArchive(t, header, payload)

	bad[len(bad)-1] ^= 1
	if extractKTXNSIS(t.Context(), bad, func(string, []byte, fs.FileMode) error { return nil }) == nil {
		t.Fatal("bad NSIS CRC accepted")
	}
}

func TestKTXNSISLZMABlocksAreBoundedAndCompletelyConsumed(t *testing.T) {
	t.Parallel()

	block := nsisTestBlock(t, bytes.Repeat([]byte("data"), 64), true)
	if _, _, err := nsisBlock(t.Context(), block, 0, 32); err == nil {
		t.Fatal("oversized output accepted")
	}

	trailing := append(append([]byte(nil), block...), 1)
	binary.LittleEndian.PutUint32(trailing, binary.LittleEndian.Uint32(trailing)+1)

	if _, _, err := nsisBlock(t.Context(), trailing, 0, 1024); err == nil {
		t.Fatal("trailing bytes hidden by LZMA buffering")
	}

	corrupt := append([]byte(nil), block...)
	binary.LittleEndian.PutUint32(corrupt[5:], 0xffffffff)

	if _, _, err := nsisBlock(t.Context(), corrupt, 0, 1024); err == nil {
		t.Fatal("excessive LZMA dictionary accepted")
	}

	if _, _, err := nsisBlock(t.Context(), block[:len(block)-1], 0, 1024); err == nil {
		t.Fatal("truncated block accepted")
	}
}

func TestKTXNSISStaticPathsRejectUnresolvedVariablesAndEscapes(t *testing.T) {
	t.Parallel()

	for _, text := range []nsisText{
		{variable: -1, suffix: "../outside"},
		{variable: -1, suffix: "/absolute"},
		{variable: nsisInstallDir, suffix: "/../outside"},
		{variable: nsisInstallDir, suffix: "//server/share"},
		{variable: nsisSavedOutDir, suffix: "/bin"},
		{variable: 23, suffix: "/outside"}, // $EXEDIR is never an installation root.
	} {
		if _, _, err := nsisPath(text, "bin", true, "", false, false); err == nil {
			t.Fatal("unsafe static path accepted", text)
		}
	}

	if name, inside, err := nsisPath(
		nsisText{variable: nsisPluginsDir, suffix: "/System.dll"},
		"",
		false,
		"",
		false,
		false,
	); err != nil || inside ||
		name != "" {
		t.Fatal("temporary plugin treated as install file", name, inside, err)
	}
}

func TestKTXNSISCancellationAndEmitterErrors(t *testing.T) {
	t.Parallel()
	header, payload := nsisTestHeader(t, "ktx.exe")
	archive := nsisTestArchive(t, header, payload)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := extractKTXNSIS(
		ctx,
		archive,
		func(string, []byte, fs.FileMode) error {
			t.Fatal("emitted after cancel")

			return nil
		},
	); !errors.Is(
		err,
		context.Canceled,
	) {
		t.Fatal(err)
	}

	sentinel := os.ErrPermission
	if err := extractKTXNSIS(t.Context(), archive, func(string, []byte, fs.FileMode) error { return sentinel }); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
}

//nolint:paralleltest // Actual decompression is serialized to bound decoder memory and isolate CPU qualification.
func TestOfficialKTXWindowsNSISArchiveExtraction(t *testing.T) {
	file := os.Getenv("KARTY_KTX_NSIS_FIXTURE")
	if file == "" {
		t.Skip("set KARTY_KTX_NSIS_FIXTURE to the checksum-pinned upstream Windows installer for CPU-only extraction qualification")
	}
	// #nosec G703 -- Explicit trusted fixture path; its exact official package SHA-256 is checked immediately.
	contents, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	digest := sha256.Sum256(contents)
	if hex.EncodeToString(digest[:]) != "1f323b0fec19794f5e6c0425a61d4b1da396872a10be862d105f4f4b2d2957fe" {
		t.Fatal("unexpected official Windows package")
	}

	files := map[string][]byte{}

	var count, total int

	err = extractKTXNSIS(t.Context(), contents, func(name string, data []byte, _ fs.FileMode) error {
		count++

		total += len(data)
		if name == "bin/ktx.exe" || name == "bin/ktx.dll" || name == "include/ktx.h" || name == "share/doc/KTX-Software/html/license.html" {
			files[name] = append([]byte(nil), data...)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if count < 100 || len(files) != 4 {
		t.Fatal("incomplete upstream extraction", count, len(files))
	}

	for _, name := range []string{"bin/ktx.exe", "bin/ktx.dll"} {
		binary, err := pe.NewFile(bytes.NewReader(files[name]))
		if err != nil {
			t.Fatal(name, err)
		}

		if binary.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
			t.Fatal("wrong PE architecture", name, binary.Machine)
		}

		symbols, err := binary.ImportedSymbols()
		if err != nil {
			t.Fatal(err)
		}

		var libraries []string
		for _, symbol := range symbols {
			if at := strings.LastIndexByte(symbol, ':'); at >= 0 && !slices.Contains(libraries, symbol[at+1:]) {
				libraries = append(libraries, symbol[at+1:])
			}
		}

		slices.Sort(libraries)
		t.Logf("%s extracted AMD64 PE, imports %v", name, libraries)
	}

	t.Logf("actual upstream NSIS CPU extraction: %d files, %d bytes; no Windows execution", count, total)
}
