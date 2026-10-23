// Package testpackages locates checksum-pinned public package fixtures prepared
// before contributor test timers start. Accessors never contact the network.
package testpackages

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/pelletier/go-toml/v2"
)

//go:embed pins.toml
var pinsData []byte

// Pin records immutable published package bytes.
type Pin struct {
	Archive string `toml:"archive"`
	SHA256  string `toml:"sha256"`
}

// Pins returns a fresh copy of the committed release pins.
func Pins() (map[string]Pin, error) {
	var document struct {
		Packages map[string]Pin `toml:"packages"`
	}
	if err := toml.Unmarshal(pinsData, &document); err != nil {
		return nil, fmt.Errorf("read published package fixture pins: %w", err)
	}

	return document.Packages, nil
}

// Project returns the ignored fixture project, or an explicit absolute override.
func Project() (string, error) {
	if value := os.Getenv("KARTY_TEST_PACKAGES"); value != "" {
		if !filepath.IsAbs(value) {
			return "", fmt.Errorf("KARTY_TEST_PACKAGES must be an absolute path: %w", os.ErrInvalid)
		}

		return filepath.Clean(value), nil
	}

	_, source, _, ok := runtime.Caller(0)
	if !ok || !filepath.IsAbs(source) {
		return "", fmt.Errorf("cannot locate fixture project; set KARTY_TEST_PACKAGES to an absolute path: %w", os.ErrInvalid)
	}

	return filepath.Join(filepath.Dir(source), "..", "..", "dist", "validation", "released-packages"), nil
}

func fixture(name string) (string, Pin, error) {
	pins, err := Pins()
	if err != nil {
		return "", Pin{}, err
	}

	pin, ok := pins[name]
	if !ok {
		return "", Pin{}, fmt.Errorf("unknown published package fixture %q: %w", name, os.ErrInvalid)
	}

	project, err := Project()

	return project, pin, err
}

func unprepared(name string, err error) error {
	return fmt.Errorf("published package fixture %s: %w; run mise run prepare-test-packages", name, err)
}

// Source returns prepared package source. The manifest itself must be regular;
// the package directory is the resolver's normal validated cache link.
func Source(name string) (string, error) {
	project, _, err := fixture(name)
	if err != nil {
		return "", err
	}

	path := filepath.Join(project, ".karty", "packages", name)

	info, err := os.Lstat(filepath.Join(path, "karty-package.toml"))
	if err != nil {
		return "", unprepared(name, err)
	}

	if !info.Mode().IsRegular() {
		return "", unprepared(name, os.ErrInvalid)
	}

	return path, nil
}

// Archive returns prepared archive bytes only after checking their SHA-256.
func Archive(name string) (string, error) {
	project, pin, err := fixture(name)
	if err != nil {
		return "", err
	}

	path := filepath.Join(project, ".karty", "packages", ".downloads", pin.SHA256+".kpkg")
	if err := verifyArchive(path, pin.SHA256); err != nil {
		return "", unprepared(name, err)
	}

	return path, nil
}

func verifyArchive(path, digest string) error {
	const maximumArchive = 256 << 20

	info, err := os.Lstat(path)
	if err != nil {
		return err
	}

	if !info.Mode().IsRegular() || info.Size() > maximumArchive {
		return os.ErrInvalid
	}

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := sha256.New()

	count, err := io.Copy(hash, io.LimitReader(file, maximumArchive+1))
	if err != nil {
		return err
	}

	if count > maximumArchive || hex.EncodeToString(hash.Sum(nil)) != digest {
		return fmt.Errorf("archive SHA-256 mismatch: %w", os.ErrInvalid)
	}

	return nil
}
