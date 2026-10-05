// karty-release synchronizes sample pins with the released SDK default.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/karty-game/karty/internal/release"
	"github.com/pelletier/go-toml/v2"
)

func main() {
	check := flag.Bool("check", false, "reject sample SDK pins that differ from their declared SDK")

	flag.Parse()

	if err := run(".", *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(root string, check bool) error {
	version := release.SDKVersion()
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(version) {
		return fmt.Errorf("invalid CurrentSDK %q: %w", release.CurrentSDK, os.ErrInvalid)
	}

	paths, err := filepath.Glob(filepath.Join(root, "samples", "*", "karty.toml"))
	if err != nil {
		return err
	}

	if len(paths) == 0 {
		return fmt.Errorf("no samples found; run from the CLI repository root: %w", os.ErrInvalid)
	}

	updates := map[string][]byte{}

	for _, path := range paths {
		original, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		updated, err := pinSample(original, version)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}

		if !bytes.Equal(original, updated) {
			updates[path] = updated
		}
	}

	if check && len(updates) != 0 {
		return fmt.Errorf("%d sample SDK pins differ from declared versions; run mise run prepare-release: %w", len(updates), os.ErrInvalid)
	}

	for _, path := range paths {
		if updated, ok := updates[path]; ok {
			if err := os.WriteFile(path, updated, 0o600); err != nil {
				return err
			}
		}
	}

	return nil
}

func pinSample(data []byte, version string) ([]byte, error) {
	var config struct {
		SDK struct {
			Version string `toml:"version"`
		} `toml:"sdk"`
	}
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	if config.SDK.Version == version {
		return data, nil
	}

	section := bytes.Index(data, []byte("[sdk]"))
	if section < 0 {
		return nil, fmt.Errorf("missing [sdk] section: %w", os.ErrInvalid)
	}

	start := section + len("[sdk]")

	end := len(data)
	if next := bytes.Index(data[start:], []byte("\n[")); next >= 0 {
		end = start + next
	}

	pattern := regexp.MustCompile(`(?m)^(version\s*=\s*)["'][^"']*["']`)
	if len(pattern.FindAll(data[start:end], -1)) != 1 {
		return nil, fmt.Errorf("expected one SDK version assignment: %w", os.ErrInvalid)
	}

	replacement := pattern.ReplaceAll(data[start:end], []byte(`${1}"`+version+`"`))

	var result strings.Builder
	result.Write(data[:start])
	result.Write(replacement)
	result.Write(data[end:])

	return []byte(result.String()), nil
}
