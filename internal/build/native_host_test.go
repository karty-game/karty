package build

import (
	"bytes"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These fixtures contain parseable executable headers for each destination.
// They test format selection, not execution of a real host.
func nativeHeader(t *testing.T, platform string) []byte {
	t.Helper()

	var data bytes.Buffer

	write := func(value any) {
		t.Helper()

		if err := binary.Write(&data, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}

	switch {
	case strings.HasPrefix(platform, "linux-"):
		machine := elf.EM_X86_64
		if platform == "linux-arm64" {
			machine = elf.EM_AARCH64
		}

		var ident [16]byte
		copy(ident[:], "\x7fELF\x02\x01\x01")
		write(elf.Header64{Ident: ident, Type: uint16(elf.ET_EXEC), Machine: uint16(machine), Version: 1, Ehsize: 64})
	case platform == "darwin-arm64":
		write(macho.FileHeader{Magic: macho.Magic64, Cpu: macho.CpuArm64, Type: macho.TypeExec})
		write(uint32(0)) // Mach-O 64-bit reserved field.
	case strings.HasPrefix(platform, "windows-"):
		machine := uint16(pe.IMAGE_FILE_MACHINE_AMD64)
		if platform == "windows-arm64" {
			machine = pe.IMAGE_FILE_MACHINE_ARM64
		}

		dos := make([]byte, 64)
		copy(dos, "MZ")
		binary.LittleEndian.PutUint32(dos[0x3c:], 64)
		data.Write(dos)
		data.WriteString("PE\x00\x00")

		optional := pe.OptionalHeader64{Magic: 0x20b, NumberOfRvaAndSizes: 16}
		write(
			pe.FileHeader{
				Machine:              machine,
				SizeOfOptionalHeader: uint16(binary.Size(optional)),
				Characteristics:      pe.IMAGE_FILE_EXECUTABLE_IMAGE,
			},
		)
		write(optional)
	default:
		t.Fatalf("unsupported fixture platform %s", platform)
	}

	return data.Bytes()
}

func TestNativeHostDestinationFormats(t *testing.T) {
	t.Parallel()

	platforms := []string{"linux-amd64", "linux-arm64", "darwin-arm64", "windows-amd64", "windows-arm64"}
	for _, source := range platforms {
		path := filepath.Join(t.TempDir(), "karty-host")
		if err := os.WriteFile(path, nativeHeader(t, source), 0o600); err != nil {
			t.Fatal(err)
		}

		for _, destination := range platforms {
			err := validateNativeHost(path, destination)
			if (err == nil) != (source == destination) {
				t.Fatalf("%s host for %s: %v", source, destination, err)
			}
		}
	}
}

func TestNativeStageRejectsInvalidHostBeforeChangingDistribution(t *testing.T) {
	t.Parallel()

	for _, contents := range [][]byte{[]byte("\x00asm\x01\x00\x00\x00"), []byte("not an executable"), {0}, nativeHeader(t, "linux-arm64")} {
		root := t.TempDir()

		host := filepath.Join(root, "host")
		if err := os.WriteFile(host, contents, 0o600); err != nil {
			t.Fatal(err)
		}

		dist := filepath.Join(root, "dist")

		staged := filepath.Join(dist, "native", "darwin-arm64", "karty-host")
		if err := os.MkdirAll(filepath.Dir(staged), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(staged, []byte("previous distribution"), 0o600); err != nil {
			t.Fatal(err)
		}

		err := stageTarget(dist, root, "missing-game.kart", nil, host, "native", "", "darwin-arm64", false, "test", false)
		if err == nil || !strings.Contains(err.Error(), "native host") {
			t.Fatalf("invalid host was not rejected before staging: %v", err)
		}

		if string(contents[:min(4, len(contents))]) == "\x00asm" && !strings.Contains(err.Error(), "WebAssembly") {
			t.Fatalf("missing actionable WASM diagnostic: %v", err)
		}

		got, err := os.ReadFile(staged)
		if err != nil || string(got) != "previous distribution" {
			t.Fatalf("previous distribution changed: %q, %v", got, err)
		}
	}
}
