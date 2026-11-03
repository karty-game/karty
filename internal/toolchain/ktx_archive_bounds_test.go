package toolchain

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

func TestKTXArchiveRejectsCaseInsensitiveDuplicateFiles(t *testing.T) {
	t.Parallel()

	files := ktxArchiveFiles{files: make(map[string]ktxArchiveFile)}
	if err := files.add("bin/ktx.exe", []byte("tool"), 0o644); err != nil {
		t.Fatal(err)
	}

	if files.add("BIN/KTX.EXE", []byte("other tool"), 0o644) == nil {
		t.Fatal("Windows-equivalent filename accepted twice")
	}
}

func TestKTXExpandedBudgetCannotSynthesizeEOF(t *testing.T) {
	t.Parallel()

	files := ktxArchiveFiles{expanded: materialExpandedLimit - 2}

	reader := ktxExpandedReader{input: bytes.NewReader([]byte("abc")), files: &files}
	if _, err := io.ReadAll(&reader); err == nil || errors.Is(err, io.EOF) {
		t.Fatal("exhausted expansion budget concealed unread bytes", err)
	}

	files.expanded = materialExpandedLimit - 2
	reader.input = bytes.NewReader([]byte("ab"))

	data, err := io.ReadAll(&reader)
	if err != nil || string(data) != "ab" || files.expanded != materialExpandedLimit {
		t.Fatal("exact-size stream rejected", string(data), err)
	}
}

func TestKTXPackagePayloadsShareExpandedBudget(t *testing.T) {
	t.Parallel()

	files := ktxArchiveFiles{files: make(map[string]ktxArchiveFile), expanded: materialExpandedLimit - 2}

	fixture := ktxPackageFixture(t, "./usr/local/bin/ktx", []byte("binary"))
	if extractKTXPackage(t.Context(), fixture, &files) == nil {
		t.Fatal("component payload reset the aggregate expansion budget")
	}
}

func TestKTXCPIOConsumesGzipChecksumAndRejectsTrailingData(t *testing.T) {
	t.Parallel()
	fixture := ktxPackageFixture(t, "./usr/local/bin/ktx", []byte("binary"))
	compressedTOC := binary.BigEndian.Uint64(fixture[8:])
	payload := fixture[28+compressedTOC:]
	badChecksum := bytes.Clone(payload)

	badChecksum[len(badChecksum)-8] ^= 1
	for _, malformed := range [][]byte{
		append(bytes.Clone(payload), 1), payload[:len(payload)-1], badChecksum,
	} {
		files := ktxArchiveFiles{files: make(map[string]ktxArchiveFile)}
		if extractKTXCPIO(t.Context(), malformed, &files) == nil {
			t.Fatal("incomplete or trailing gzip stream accepted")
		}
	}
}

func TestKTXVirtualLibraryAliasesRemainInsideArchive(t *testing.T) {
	t.Parallel()

	for _, link := range []string{"../../escape", "/absolute", "bad\\path", "bad\x00path"} {
		if _, err := ktxAliasTarget("lib/libktx.so", link); err == nil {
			t.Fatal("escaping library alias accepted", link)
		}
	}

	if target, err := ktxAliasTarget("lib/libktx.so", "libktx.so.4"); err != nil || target != "lib/libktx.so.4" {
		t.Fatal(target, err)
	}
}

func TestKTXNestedArchiveReadsRespectCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	files := ktxArchiveFiles{files: make(map[string]ktxArchiveFile)}

	fixture := ktxPackageFixture(t, "./usr/local/bin/ktx", []byte("binary"))
	if err := extractKTXPackage(ctx, fixture, &files); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}

	if err := extractKTXTar(ctx, nil, "https://example.invalid/KTX.tar.bz2", &files); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
