package build

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/karty-game/karty/internal/contextio"
)

func copyFile(ctx context.Context, source, destination string, mode os.FileMode) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// #nosec G304 -- The caller supplies a validated build-owned or pinned tool path.
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()

	return copyToFile(ctx, file, destination, mode)
}

// Copy beside the destination and publish only after all reads/writes complete.
// A canceled copy keeps the prior destination and removes its partial output.
func copyToFile(ctx context.Context, input io.Reader, destination string, mode os.FileMode) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	file, err := os.CreateTemp(filepath.Dir(destination), ".copy-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())

	if err := file.Chmod(mode); err != nil {
		_ = file.Close()

		return err
	}

	_, copyErr := contextio.Copy(ctx, file, input)
	if err := errors.Join(copyErr, file.Close(), ctx.Err()); err != nil {
		return err
	}

	// #nosec G703 -- Both paths belong to the validated target staging directory.
	return os.Rename(file.Name(), destination)
}

func fileHash(ctx context.Context, path string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	// #nosec G304 -- The caller supplies a validated build artifact path.
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := contextio.Copy(ctx, hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)[:8]), nil
}

func readRootFile(ctx context.Context, root *os.Root, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	file, err := root.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return contextio.ReadAll(ctx, file)
}

func writeRootFile(ctx context.Context, root *os.Root, path string, contents []byte, mode fs.FileMode) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	file, err := root.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}

	_, writeErr := contextio.Copy(ctx, file, bytes.NewReader(contents))

	return errors.Join(writeErr, file.Close())
}
