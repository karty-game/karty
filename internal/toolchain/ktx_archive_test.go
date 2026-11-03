package toolchain

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"crypto/sha256"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestKTXArchiveAliasesRemainConfinedRegularFiles(t *testing.T) {
	t.Parallel()

	files := ktxArchiveFiles{files: make(map[string]ktxArchiveFile)}
	if err := files.add("lib/libktx.so.4.4.2", []byte("library"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := files.record("lib/libktx.so.4", ktxArchiveFile{link: "lib/libktx.so.4.4.2"}); err != nil {
		t.Fatal(err)
	}

	value, err := files.resolve("lib/libktx.so.4", make(map[string]bool))
	if err != nil || string(value.data) != "library" {
		t.Fatal(value, err)
	}

	if files.record("escape", ktxArchiveFile{link: "../escape"}) == nil {
		t.Fatal("escaping alias accepted")
	}

	if files.add("lib/libktx.so.4", nil, 0o644) == nil {
		t.Fatal("duplicate accepted")
	}

	if err := files.record("cycle", ktxArchiveFile{link: "cycle"}); err != nil {
		t.Fatal(err)
	}

	if _, err := files.resolve("cycle", make(map[string]bool)); err == nil {
		t.Fatal("alias cycle accepted")
	}
}

func ktxPackageFixture(t *testing.T, member string, payload []byte) []byte {
	t.Helper()

	var body bytes.Buffer

	compressor := gzip.NewWriter(&body)
	write := func(name string, data []byte, mode uint32) {
		t.Helper()

		header := fmt.Sprintf("070707%06o%06o%06o%06o%06o%06o%06o%011o%06o%011o", 0, 0, mode, 0, 0, 1, 0, 0, len(name)+1, len(data))
		if len(header) != 76 {
			t.Fatal(len(header))
		}

		if _, err := io.WriteString(compressor, header+name+"\x00"); err != nil {
			t.Fatal(err)
		}

		if _, err := compressor.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	write(member, payload, 0o100755)
	write("TRAILER!!!", nil, 0)

	if err := compressor.Close(); err != nil {
		t.Fatal(err)
	}

	toc := fmt.Sprintf(
		`<xar><toc><file><name>tools.pkg</name><type>directory</type><file><name>Payload</name><type>file</type><data><offset>0</offset><length>%d</length><size>%d</size><encoding style="application/octet-stream"/></data></file></file></toc></xar>`,
		body.Len(),
		body.Len(),
	)

	var compressed bytes.Buffer

	z := zlib.NewWriter(&compressed)
	if _, err := io.WriteString(z, toc); err != nil {
		t.Fatal(err)
	}

	if err := z.Close(); err != nil {
		t.Fatal(err)
	}

	header := make([]byte, 28+compressed.Len()+body.Len())
	copy(header, "xar!")
	binary.BigEndian.PutUint16(header[4:], 28)
	binary.BigEndian.PutUint16(header[6:], 1)
	binary.BigEndian.PutUint64(header[8:], uint64(compressed.Len()))
	binary.BigEndian.PutUint64(header[16:], uint64(len(toc)))
	copy(header[28:], compressed.Bytes())
	copy(header[28+compressed.Len():], body.Bytes())

	return header
}

func TestKTXPackageExtractionRejectsMalformedAndEscapingFiles(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"./usr/local/bin/ktx", "../../escape", "./usr/local/../escape", "/usr/local/bin/ktx"} {
		files := ktxArchiveFiles{files: make(map[string]ktxArchiveFile)}

		err := extractKTXPackage(t.Context(), ktxPackageFixture(t, name, []byte("binary")), &files)
		if name == "./usr/local/bin/ktx" {
			if err != nil || string(files.files["bin/ktx"].data) != "binary" {
				t.Fatal(files, err)
			}
		} else if err == nil {
			t.Fatalf("unsafe package path accepted %q", name)
		}
	}

	fixture := ktxPackageFixture(t, "./usr/local/bin/ktx", []byte("binary"))
	for _, data := range [][]byte{fixture[:27], fixture[:len(fixture)-1], append(bytes.Clone(fixture), 1)} {
		files := ktxArchiveFiles{files: make(map[string]ktxArchiveFile)}
		if extractKTXPackage(t.Context(), data, &files) == nil {
			t.Fatal("malformed package accepted")
		}
	}
}

type officialKTXFixture struct{ platform, name, format, hash string }

//nolint:paralleltest // Real package decompression and warm-cache qualification are serialized to bound memory and disk work.
func TestOfficialKTXPublishedPackages(t *testing.T) {
	directory := os.Getenv("KARTY_TEST_KTX_PACKAGES")
	if directory == "" {
		t.Skip("set KARTY_TEST_KTX_PACKAGES for official artifact interoperability")
	}

	fixtures := []officialKTXFixture{
		{
			"linux-amd64",
			"KTX-Software-4.4.2-Linux-x86_64.tar.bz2",
			"tar.bz2",
			"a8781bad05f9624edbf910b7f258cd0a4ba7d3e63b49ecc0a0ab440bf6a0a245",
		},
		{
			"linux-arm64",
			"KTX-Software-4.4.2-Linux-arm64.tar.bz2",
			"tar.bz2",
			"60382e7b842177b8048bd58ccdc770383f8ef65b94452a25d3afdb55f2405c5a",
		},
		{"darwin-arm64", "KTX-Software-4.4.2-Darwin-arm64.pkg", "pkg", "500bd8f9d63358c3f3a0d83b724c8574436a72c37dc0e4bad90ec1ca38032c3c"},
		{"windows-amd64", "KTX-Software-4.4.2-Windows-x64.exe", "nsis", "1f323b0fec19794f5e6c0425a61d4b1da396872a10be862d105f4f4b2d2957fe"},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.platform, func(t *testing.T) {
			contents := readOfficialKTXFixture(t, directory, fixture)
			installOfficialKTXFixture(t, fixture, contents)
		})
	}
}

func readOfficialKTXFixture(t *testing.T, directory string, fixture officialKTXFixture) []byte {
	t.Helper()

	file := filepath.Join(directory, fixture.name)
	if fixture.platform == "linux-amd64" {
		// #nosec G703 -- Explicit trusted local fixture directory; the publisher SHA-256 is checked before extraction.
		if _, err := os.Stat(file); os.IsNotExist(err) {
			file = filepath.Join(directory, "ktx.tar.bz2")
		}
	}
	// #nosec G703 -- Explicit opt-in fixture path, authenticated against the published whole-file digest below.
	contents, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	if fmt.Sprintf("%x", sha256.Sum256(contents)) != fixture.hash {
		t.Fatal("publisher checksum mismatch")
	}

	return contents
}

func installOfficialKTXFixture(t *testing.T, fixture officialKTXFixture, contents []byte) {
	t.Helper()

	executable := "bin/ktx"
	if fixture.platform == "windows-amd64" {
		executable += ".exe"
	}

	options := InstallMaterialToolOptions{
		Spec: MaterialToolSpec{
			Tool: MaterialToolKTX, Revision: "4d6fc70eaf62ad0558e63e8d97eb9766118327a6",
			Artifacts: map[string]MaterialToolArtifact{fixture.platform: {
				URL:    "https://github.com/KhronosGroup/KTX-Software/releases/download/v4.4.2/" + fixture.name,
				SHA256: fixture.hash, Format: fixture.format, Executable: executable,
			}},
		},
		Platform: fixture.platform, CacheDir: t.TempDir(),
		Download: func(context.Context, string) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(contents)), nil
		},
	}

	installed, err := InstallMaterialTool(t.Context(), options)
	if err != nil {
		t.Fatal(err)
	}

	checkOfficialKTXImage(t, fixture.platform, installed)

	if runtime.GOOS+"-"+runtime.GOARCH == fixture.platform {
		output, err := exec.CommandContext(t.Context(), installed, "--version").CombinedOutput()
		if err != nil || !strings.Contains(string(output), "4.4.2") {
			t.Fatal(string(output), err)
		}
	}

	options.Download = func(context.Context, string) (io.ReadCloser, error) {
		t.Error("warm installation downloaded again")

		return nil, os.ErrNotExist
	}
	if _, err := InstallMaterialTool(t.Context(), options); err != nil {
		t.Fatal("warm integrity", err)
	}
}

func checkOfficialKTXImage(t *testing.T, target, installed string) {
	t.Helper()

	contents, err := os.ReadFile(installed)
	if err != nil {
		t.Fatal(err)
	}

	switch target {
	case "linux-arm64":
		image, err := elf.NewFile(bytes.NewReader(contents))
		if err != nil || image.Machine != elf.EM_AARCH64 || image.Class != elf.ELFCLASS64 {
			t.Fatal("invalid ARM64 ELF", err)
		}
	case "darwin-arm64":
		image, err := macho.NewFile(bytes.NewReader(contents))
		if err != nil || image.Cpu != macho.CpuArm64 {
			t.Fatal("invalid macOS executable", err)
		}

		alias, err := os.Lstat(filepath.Join(filepath.Dir(installed), "../lib/libktx.4.dylib"))
		if err != nil || !alias.Mode().IsRegular() {
			t.Fatal("library alias not materialized", err)
		}
	case "windows-amd64":
		image, err := pe.NewFile(bytes.NewReader(contents))
		if err != nil || image.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
			t.Fatal("invalid Windows AMD64 PE", err)
		}
	}
}
