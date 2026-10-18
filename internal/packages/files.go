// Package packages resolves local Karty package archives and development folders.
package packages

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"unicode/utf8"
)

const (
	ManifestName                  = "karty-package.toml"
	InventoryName                 = "karty-package.files.json"
	MaxFiles                      = 8192
	MaxFileBytes                  = 64 << 20
	MaxTotalBytes                 = 512 << 20
	MaxArchiveBytes               = 256 << 20
	MaxManifestBytes              = 1 << 20
	MaxPackages                   = 16
	MaxTextures                   = 256
	MaxPathDepth                  = 32
	generatedFileMode fs.FileMode = 0600
)

var ErrPackage = errors.New("invalid Karty package")

func validPath(name string) bool {
	if !fs.ValidPath(name) || name == "." || len(name) > 1024 || strings.ContainsAny(name, `\:<>"|?*`) ||
		strings.Count(name, "/") > MaxPathDepth {
		return false
	}

	if !utf8.ValidString(name) {
		return false
	}

	for _, character := range name {
		if character < 32 || character == 127 {
			return false
		}
	}

	for part := range strings.SplitSeq(name, "/") {
		if strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return false
		}

		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" ||
			(len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9') {
			return false
		}
	}

	return true
}

func Read(ctx context.Context, root *os.Root, name string, limit int64) ([]byte, error) {
	if !validPath(name) {
		return nil, fmt.Errorf("%w: invalid path %q", ErrPackage, name)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	info, err := root.Stat(name)
	if err != nil {
		return nil, err
	}

	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("%w: oversized or nonregular %s", ErrPackage, name)
	}

	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err = file.Stat()
	if err != nil {
		return nil, err
	}

	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, ErrPackage
	}

	return readBounded(ctx, file, limit)
}

func readBounded(ctx context.Context, reader io.Reader, limit int64) ([]byte, error) {
	var out strings.Builder

	buffer := make([]byte, 32<<10)
	reader = io.LimitReader(reader, limit+1)

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		n, err := reader.Read(buffer)
		if int64(out.Len()+n) > limit {
			return nil, fmt.Errorf("%w: input exceeds %d bytes", ErrPackage, limit)
		}

		_, _ = out.Write(buffer[:n])
		if errors.Is(err, io.EOF) {
			return []byte(out.String()), nil
		}

		if err != nil {
			return nil, err
		}
	}
}

func Digest(data []byte) string {
	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:])
}

// MakeDirectories rejects every existing symlink component, including links
// confined within the root. Generated outputs must not redirect into user files.
func MakeDirectories(root *os.Root, name string) error {
	if name == "." {
		return nil
	}

	if !validPath(name) {
		return ErrPackage
	}

	parts := strings.Split(name, "/")
	for i := range parts {
		part := strings.Join(parts[:i+1], "/")

		info, err := root.Lstat(part)
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("%w: %s must be a real directory", ErrPackage, part)
			}

			continue
		}

		if !os.IsNotExist(err) {
			return err
		}

		if err := root.Mkdir(part, 0750); err != nil && !os.IsExist(err) {
			return err
		}

		info, err = root.Lstat(part)
		if err != nil || !info.IsDir() {
			return ErrPackage
		}
	}

	return nil
}

// WriteGenerated never follows an existing file link. Immutable files are checked
// byte-for-byte when reused; mutable metadata is published with rooted rename.
func WriteGenerated(ctx context.Context, root *os.Root, name string, data []byte, immutable bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if len(data) > MaxArchiveBytes {
		return ErrPackage
	}

	if err := MakeDirectories(root, path.Dir(name)); err != nil {
		return err
	}

	complete, mode, err := existingOutput(ctx, root, name, data, immutable)
	if err != nil || complete {
		return err
	}

	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}

	temporary := path.Join(path.Dir(name), fmt.Sprintf(".karty-%x.tmp", nonce))

	file, err := root.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer func() { _ = root.Remove(temporary) }()

	_, writeErr := file.Write(data)
	closeErr := file.Close()

	if writeErr != nil {
		return writeErr
	}

	if closeErr != nil {
		return closeErr
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	return root.Rename(temporary, name)
}

func existingOutput(ctx context.Context, root *os.Root, name string, data []byte, immutable bool) (bool, fs.FileMode, error) {
	info, err := root.Lstat(name)
	if os.IsNotExist(err) {
		return false, generatedFileMode, nil
	}

	if err != nil {
		return false, 0, err
	}

	if !info.Mode().IsRegular() {
		return false, 0, ErrPackage
	}

	if !immutable {
		return false, info.Mode().Perm(), nil
	}

	before, err := Read(ctx, root, name, max(MaxFileBytes, int64(len(data))))
	if err != nil {
		return false, 0, err
	}

	if Digest(before) != Digest(data) {
		return false, 0, fmt.Errorf("%w: cached input changed: %s", ErrPackage, name)
	}

	return true, info.Mode().Perm(), nil
}
