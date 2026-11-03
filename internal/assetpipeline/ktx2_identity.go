package assetpipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/karty-game/karty/internal/contextio"
	"github.com/karty-game/karty/internal/sdk"
)

const (
	ktxIdentityReadBatch      = 64
	ktxIdentityDirectoryLimit = 4096
	ktxIdentityLibraryLimit   = 16
	ktxIdentityFileLimit      = 64 << 20
	ktxIdentityTotalLimit     = 256 << 20
)

func materialKTX2Recipe(
	ctx context.Context,
	encoder string,
	manifest sdk.Manifest,
	request KTX2EncodeRequest,
	managed bool,
) (Recipe, error) {
	absolute, err := filepath.Abs(encoder)
	if err != nil {
		return Recipe{}, err
	}

	executable, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return Recipe{}, err
	}

	executableHash, _, err := hashKTXIdentityFile(ctx, executable, ktxIdentityFileLimit)
	if err != nil {
		return Recipe{}, err
	}

	revisions := []Revision{
		{Name: "encoder", Version: executableHash},
		{Name: "encoder-platform", Version: runtime.GOOS + "-" + runtime.GOARCH},
		{Name: "layout", Version: "rgba-array-mips@1"},
		{Name: "encoder-loader", Version: "local-libraries-no-loader-overrides@1"},
	}
	if managed {
		archive := manifest.Artifacts.KTX[runtime.GOOS+"-"+runtime.GOARCH]
		if manifest.Tools.KTX == "" || manifest.Tools.KTXRevision == "" || archive.SHA256 == "" {
			return Recipe{}, ErrCacheRecipe
		}

		revisions = append(
			revisions,
			Revision{Name: "encoder-version", Version: manifest.Tools.KTX},
			Revision{
				Name:    "encoder-source",
				Version: manifest.Tools.KTXRevision,
			},
			Revision{Name: "encoder-archive", Version: archive.SHA256},
		)
	}

	libraries, err := ktxLibraryRevisions(ctx, executable, runtime.GOOS)
	if err != nil {
		return Recipe{}, err
	}

	revisions = append(revisions, libraries...)

	return NewRecipe("world-material-gpu", materialGPUProcessor, revisions, struct {
		Width, Height, Layers  uint32
		SRGB                   bool
		Levels                 int
		Codec                  string
		Quality, Zstd, Threads int
	}{request.Width, request.Height, request.Layers, request.SRGB, len(request.Levels), "uastc", ktxUASTCQuality, ktxZstdLevel, ktxEncoderThreads})
}

// Only official executable and neighboring library layouts participate. File aliases are
// resolved within the real library directory; absolute install paths do not
// affect identity. Statically linked candidate encoders can have no libktx.
func ktxLibraryRevisions(ctx context.Context, executable, system string) ([]Revision, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	directories := []struct{ name, path string }{{"bin", filepath.Dir(executable)}}
	if system != "windows" {
		directories = append(directories, struct{ name, path string }{
			"lib", filepath.Join(filepath.Dir(filepath.Dir(executable)), "lib"),
		})
	}

	var (
		revisions []Revision
		total     int64
	)

	for _, directory := range directories {
		libraries, size, err := ktxDirectoryLibraryRevisions(ctx, directory.path, system, directory.name, ktxIdentityTotalLimit-total)
		if err != nil {
			return nil, err
		}

		revisions = append(revisions, libraries...)
		total += size

		if len(revisions) > ktxIdentityLibraryLimit {
			return nil, ErrCacheRecipe
		}
	}

	return revisions, nil
}

func ktxDirectoryLibraryRevisions(ctx context.Context, directory, system, prefix string, remaining int64) ([]Revision, int64, error) {
	// #nosec G703 -- Inspect only the known neighboring library directory of the explicitly selected encoder.
	info, err := os.Lstat(directory)
	if os.IsNotExist(err) {
		return nil, 0, nil
	}

	if err != nil {
		return nil, 0, err
	}

	if !info.IsDir() {
		return nil, 0, fmt.Errorf("KTX library directory must be a real directory: %w", ErrCacheRecipe)
	}

	names, err := ktxLibraryNames(ctx, directory, system)
	if err != nil {
		return nil, 0, err
	}

	var total int64

	revisions := make([]Revision, 0, len(names))
	for _, name := range names {
		resolved, err := filepath.EvalSymlinks(filepath.Join(directory, name))
		if err != nil {
			return nil, 0, err
		}

		relative, err := filepath.Rel(directory, resolved)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
			return nil, 0, fmt.Errorf("KTX library alias escapes its directory: %w", ErrCacheRecipe)
		}

		digest, size, err := hashKTXIdentityFile(ctx, resolved, min(ktxIdentityFileLimit, remaining-total))
		if err != nil {
			return nil, 0, err
		}

		total += size

		revisions = append(revisions, Revision{Name: "encoder-lib-" + prefix + "-" + name, Version: digest})
	}

	return revisions, total, nil
}

func ktxLibraryNames(ctx context.Context, directory, system string) ([]string, error) {
	// #nosec G703 -- The caller checked this neighboring encoder library directory is a real directory.
	folder, err := os.Open(directory)
	if err != nil {
		return nil, err
	}
	defer folder.Close()

	var (
		names []string
		count int
	)

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		entries, err := folder.ReadDir(ktxIdentityReadBatch)
		if err != nil && err != io.EOF {
			return nil, err
		}

		count += len(entries)
		if count > ktxIdentityDirectoryLimit {
			return nil, ErrCacheRecipe
		}

		for _, entry := range entries {
			name := entry.Name()

			matches := system == "linux" && strings.HasPrefix(name, "libktx.so") ||
				system == "darwin" && strings.HasPrefix(name, "libktx") && strings.HasSuffix(name, ".dylib") ||
				system == "windows" && strings.EqualFold(name, "ktx.dll")
			if matches {
				if !validCacheToken("encoder-lib-"+name, maxRevisionNameLength) {
					return nil, ErrCacheRecipe
				}

				names = append(names, name)
				if len(names) > ktxIdentityLibraryLimit {
					return nil, ErrCacheRecipe
				}
			}
		}

		if err == io.EOF {
			break
		}
	}

	slices.Sort(names)

	return names, nil
}

func hashKTXIdentityFile(ctx context.Context, name string, limit int64) (string, int64, error) {
	// #nosec G703 -- The caller supplies the explicit encoder or a library alias already confined to its real library directory.
	file, err := os.Open(name)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", 0, err
	}

	if !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > limit {
		return "", 0, fmt.Errorf("invalid or oversized KTX encoder dependency: %w", ErrCacheRecipe)
	}

	hash := sha256.New()

	count, err := contextio.Copy(ctx, hash, io.LimitReader(file, limit+1))
	if err != nil {
		return "", 0, err
	}

	if count != info.Size() || count > limit {
		return "", 0, fmt.Errorf("KTX dependency changed during hashing: %w", ErrCacheRecipe)
	}

	return hex.EncodeToString(hash.Sum(nil)), count, nil
}
