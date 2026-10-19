package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

var (
	errNativeHostMissing = errors.New("KARTY_HOST_NATIVE must name the native host artifact")
	errWebHostMissing    = errors.New("KARTY_HOST_WEB must name the browser host artifact")
	errHostCheckMissing  = errors.New("KARTY_HOST_CHECK must name the karty-host-check executable")
	errBuildChanged      = errors.New("repeated build produced different distribution bytes")
)

func distributionDigest(root string) (string, error) {
	paths := make([]string, 0, 4)

	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.Type().IsRegular() {
			paths = append(paths, path)
		}

		return nil
	}); err != nil {
		return "", fmt.Errorf("walk distribution: %w", err)
	}

	slices.Sort(paths)

	digest := sha256.New()

	for _, path := range paths {
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return "", err
		}

		contents, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}

		digest.Write([]byte(filepath.ToSlash(relative)))
		digest.Write([]byte{0})
		digest.Write(contents)
	}

	return hex.EncodeToString(digest.Sum(nil)), nil
}
