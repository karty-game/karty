package build

import (
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/karty-game/karty/internal/toolchain"
)

// validateNativeHost inspects the destination binary without executing it.
func validateNativeHost(path, platform string) error {
	platform, err := toolchain.NativePlatform(platform)
	if err != nil {
		return err
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open native host: %w", err)
	}
	defer file.Close()

	var magic [4]byte
	if _, err := file.ReadAt(magic[:], 0); err != nil {
		return fmt.Errorf("native host %q is not an executable for %s: %w", path, platform, err)
	}

	if string(magic[:]) == "\x00asm" {
		return fmt.Errorf(
			"native host %q is WebAssembly; %s requires a native executable (check --host and the SDK host cache): %w",
			path,
			platform,
			os.ErrInvalid,
		)
	}

	if !nativeHostMatches(file, platform) {
		return fmt.Errorf(
			"native host %q is not an executable for %s (check --host and the SDK host cache): %w",
			path,
			platform,
			os.ErrInvalid,
		)
	}

	return nil
}

func nativeHostMatches(file io.ReaderAt, platform string) bool {
	switch {
	case strings.HasPrefix(platform, "linux-"):
		binary, err := elf.NewFile(file)
		if err != nil {
			return false
		}

		machine := elf.EM_X86_64
		if platform == "linux-arm64" {
			machine = elf.EM_AARCH64
		}

		return binary.Class == elf.ELFCLASS64 && binary.Data == elf.ELFDATA2LSB &&
			binary.Machine == machine && (binary.Type == elf.ET_EXEC || binary.Type == elf.ET_DYN)
	case platform == "darwin-arm64":
		if binary, err := macho.NewFile(file); err == nil {
			return binary.Cpu == macho.CpuArm64 && binary.Type == macho.TypeExec
		}

		if binary, err := macho.NewFatFile(file); err == nil {
			for _, arch := range binary.Arches {
				if arch.Cpu == macho.CpuArm64 && arch.Type == macho.TypeExec {
					return true
				}
			}
		}
	case strings.HasPrefix(platform, "windows-"):
		binary, err := pe.NewFile(file)
		if err != nil {
			return false
		}

		machine := uint16(pe.IMAGE_FILE_MACHINE_AMD64)
		if platform == "windows-arm64" {
			machine = pe.IMAGE_FILE_MACHINE_ARM64
		}

		_, is64 := binary.OptionalHeader.(*pe.OptionalHeader64)

		return binary.Machine == machine && is64 &&
			binary.Characteristics&pe.IMAGE_FILE_EXECUTABLE_IMAGE != 0 &&
			binary.Characteristics&pe.IMAGE_FILE_DLL == 0
	}

	return false
}
