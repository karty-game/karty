// Package toolchain resolves tools selected by a project's pinned SDK.
package toolchain

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type staticError string

func (err staticError) Error() string {
	return string(err)
}

const (
	errTinyGoVersion       staticError = "TinyGo version is not pinned by the selected SDK"
	errNotExecutable       staticError = "path is not an executable file"
	errTinyGoMetadata      staticError = "TinyGo installation metadata is incomplete"
	errUnexpectedHTTP      staticError = "unexpected HTTP status"
	errTinyGoChecksum      staticError = "TinyGo archive checksum mismatch"
	errInvalidTinyGoPath   staticError = "invalid TinyGo archive path"
	errTinyGoEntryTooLarge staticError = "TinyGo archive entry is too large"
	errWasmToolsVersion    staticError = "wasm-tools version is not pinned by the selected SDK"
	errWasmToolsMetadata   staticError = "wasm-tools installation metadata is incomplete"
	errWasmToolsChecksum   staticError = "wasm-tools archive checksum mismatch"
	errWasmEntryTooLarge   staticError = "wasm-tools archive entry is too large"
)

// TinyGoOptions supplies the supported TinyGo selection inputs.
type TinyGoOptions struct {
	Override string
	Version  string
	CacheDir string
	Getenv   func(string) string
}

// TinyGo returns the selected TinyGo executable. Selection order is explicit
// override, KARTY_TINYGO, then the CLI-managed user cache.
func TinyGo(options TinyGoOptions) (string, error) {
	getenv := options.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}

	if options.Override != "" {
		return executable("--tinygo", options.Override)
	}

	if path := getenv("KARTY_TINYGO"); path != "" {
		return executable("KARTY_TINYGO", path)
	}

	if options.Version == "" {
		return "", errTinyGoVersion
	}

	cacheDir := options.CacheDir
	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve Karty home directory: %w", err)
		}

		cacheDir = filepath.Join(home, ".karty")
	}

	path := filepath.Join(
		cacheDir,
		"tools",
		"tinygo",
		options.Version,
		runtime.GOOS+"-"+runtime.GOARCH,
		"tinygo",
		"bin",
		executableName("tinygo"),
	)

	return executable("managed TinyGo", path)
}

func executable(source, path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("%s executable: %w", source, err)
	}

	if info.IsDir() || (runtime.GOOS != "windows" && info.Mode()&0o111 == 0) {
		return "", fmt.Errorf("%s is not an executable file: %s: %w", source, path, errNotExecutable)
	}

	return path, nil
}

func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}

	return name
}
