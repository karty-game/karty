package assetpipeline

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/sdk"
)

func ktxIdentityFixture(t *testing.T, root, system string) (string, string) {
	t.Helper()

	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o750); err != nil {
		t.Fatal(err)
	}

	executable := filepath.Join(bin, "ktx")
	if err := os.WriteFile(executable, []byte("unchanged encoder executable"), 0o600); err != nil {
		t.Fatal(err)
	}

	folder, name := filepath.Join(root, "lib"), "libktx.so.4.4.2"

	switch system {
	case "darwin":
		name = "libktx.4.dylib"
	case "windows":
		folder, name = bin, "ktx.dll"
	}

	if err := os.MkdirAll(folder, 0o750); err != nil {
		t.Fatal(err)
	}

	library := filepath.Join(folder, name)
	if err := os.WriteFile(library, []byte("first encoder library"), 0o600); err != nil {
		t.Fatal(err)
	}

	return executable, library
}

func TestKTXLibraryChangeInvalidatesEncoderCache(t *testing.T) {
	t.Parallel()
	executable, library := ktxIdentityFixture(t, t.TempDir(), runtime.GOOS)
	request := KTX2EncodeRequest{Width: 256, Height: 256, Layers: 1, Levels: make([][]byte, 9)}

	source, err := SnapshotBytes([]byte("identical prepared material pixels"))
	if err != nil {
		t.Fatal(err)
	}

	cache := NewProjectCache(t.TempDir())
	processed := 0
	resolve := func() Artifact {
		t.Helper()

		recipe, err := materialKTX2Recipe(t.Context(), executable, sdk.Manifest{}, request, false)
		if err != nil {
			t.Fatal(err)
		}

		artifact, err := cache.Resolve(t.Context(), source, recipe, func(context.Context, SourceSnapshot, Recipe) (Processed, error) {
			processed++

			return Processed{Encoding: materialGPUProcessor, Payload: []byte("encoded texture fixture")}, nil
		})
		if err != nil {
			t.Fatal(err)
		}

		return artifact
	}

	first, warm := resolve(), resolve()
	if first.CacheHit || !warm.CacheHit || first.CacheKey != warm.CacheKey || processed != 1 {
		t.Fatal("stable encoder dependencies did not reuse cached output")
	}

	executableBefore, err := SnapshotFile(executable)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(library, []byte("updated encoder library"), 0o600); err != nil {
		t.Fatal(err)
	}

	changed := resolve()

	executableAfter, err := SnapshotFile(executable)
	if err != nil || executableBefore.Digest() != executableAfter.Digest() {
		t.Fatal("fixture changed the encoder executable", err)
	}

	if changed.CacheHit || changed.CacheKey == first.CacheKey || processed != 2 {
		t.Fatal("library update reused stale texture output")
	}
}

func TestKTXManagedPinsParticipateInRecipe(t *testing.T) {
	t.Parallel()
	executable, _ := ktxIdentityFixture(t, t.TempDir(), runtime.GOOS)
	selected := sdk.Manifest{}
	selected.Tools.KTX, selected.Tools.KTXRevision = "4.4.2", "4d6fc70eaf62ad0558e63e8d97eb9766118327a6"
	platform := runtime.GOOS + "-" + runtime.GOARCH
	selected.Artifacts.KTX = map[string]sdk.MaterialToolArtifact{platform: {SHA256: strings.Repeat("a", 64)}}
	request := KTX2EncodeRequest{Width: 256, Height: 256, Layers: 1, Levels: make([][]byte, 9)}

	first, err := materialKTX2Recipe(t.Context(), executable, selected, request, true)
	if err != nil {
		t.Fatal(err)
	}

	for name, mutate := range map[string]func(*sdk.Manifest){
		"upstream version": func(m *sdk.Manifest) { m.Tools.KTX = "4.4.3" },
		"upstream source":  func(m *sdk.Manifest) { m.Tools.KTXRevision = strings.Repeat("c", 40) },
		"archive checksum": func(m *sdk.Manifest) {
			entry := m.Artifacts.KTX[platform]
			entry.SHA256 = strings.Repeat("b", 64)
			m.Artifacts.KTX[platform] = entry
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			changed := selected
			changed.Artifacts.KTX = maps.Clone(selected.Artifacts.KTX)
			mutate(&changed)

			recipe, err := materialKTX2Recipe(t.Context(), executable, changed, request, true)
			if err != nil || recipe.Digest() == first.Digest() {
				t.Fatal("managed tool pin change did not invalidate recipe", err)
			}
		})
	}
}

func TestKTXExecutableDirectoryLibraryChangeInvalidatesRecipe(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "linux" {
		t.Skip("qualifies the official Linux RUNPATH")
	}

	executable, _ := ktxIdentityFixture(t, t.TempDir(), "linux")

	library := filepath.Join(filepath.Dir(executable), "libktx.so.4")
	if err := os.WriteFile(library, []byte("RUNPATH executable directory first"), 0o600); err != nil {
		t.Fatal(err)
	}

	first, err := ktxLibraryRevisions(t.Context(), executable, "linux")
	if err != nil || len(first) != 2 {
		t.Fatal("both official RUNPATH directories must participate", first, err)
	}

	request := KTX2EncodeRequest{Width: 256, Height: 256, Layers: 1, Levels: make([][]byte, 9)}

	recipe, err := materialKTX2Recipe(t.Context(), executable, sdk.Manifest{}, request, false)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(library, []byte("RUNPATH executable directory changed"), 0o600); err != nil {
		t.Fatal(err)
	}

	changed, err := ktxLibraryRevisions(t.Context(), executable, "linux")
	if err != nil || len(changed) != 2 || first[0].Version == changed[0].Version || first[1] != changed[1] {
		t.Fatal("shadowing executable-directory library did not change identity independently", changed, err)
	}

	changedRecipe, err := materialKTX2Recipe(t.Context(), executable, sdk.Manifest{}, request, false)
	if err != nil || recipe.Digest() == changedRecipe.Digest() {
		t.Fatal("shadowing executable-directory library reused encoder recipe", err)
	}
}

func TestKTXEncoderEnvironmentRejectsLoaderOverrides(t *testing.T) {
	t.Parallel()

	original := []string{"PATH=/tools", "LD_LIBRARY_PATH=/unrecorded", "LD_PRELOAD=/injected.so",
		"LD_AUDIT=/audit.so", "DYLD_LIBRARY_PATH=/unrecorded", "DYLD_INSERT_LIBRARIES=/injected.dylib", "HOME=/home/user"}

	before := slices.Clone(original)
	if got := ktxEncoderEnvironment(original); !slices.Equal(got, []string{"PATH=/tools", "HOME=/home/user"}) ||
		!slices.Equal(original, before) {
		t.Fatal("encoder inherited loader injection or mutated process environment", got)
	}
}

func TestKTXActualEncoderIgnoresInheritedLibraryOverride(t *testing.T) {
	encoder := os.Getenv("KARTY_TEST_KTX")
	if runtime.GOOS != "linux" || encoder == "" {
		t.Skip("requires an explicitly selected actual Linux encoder")
	}

	directory := t.TempDir()

	library := filepath.Join(directory, "libktx.so.4")
	if err := os.WriteFile(library, []byte("invalid shared library deliberately shadows RUNPATH"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("LD_LIBRARY_PATH", directory)
	t.Setenv("LD_PRELOAD", library)
	base, tiles := gpuMaterialFixture(t)

	arrays, err := PrepareWorldMaterialGPU(t.Context(), base, tiles)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := runKTX2Encoder(t.Context(), encoder, arrays[0]); err != nil {
		t.Fatal("unrecorded inherited loader settings affected encoding", err)
	}
}

func TestKTXLibraryLayoutsAndConfinement(t *testing.T) {
	t.Parallel()

	for _, system := range []string{"linux", "darwin", "windows"} {
		t.Run(system, func(t *testing.T) {
			t.Parallel()
			executable, _ := ktxIdentityFixture(t, t.TempDir(), system)

			first, err := ktxLibraryRevisions(t.Context(), executable, system)
			if err != nil || len(first) != 1 {
				t.Fatal("known official library layout not fingerprinted", first, err)
			}

			other, _ := ktxIdentityFixture(t, t.TempDir(), system)

			second, err := ktxLibraryRevisions(t.Context(), other, system)
			if err != nil || len(second) != 1 || first[0] != second[0] {
				t.Fatal("installation location affected dependency identity", err)
			}
		})
	}

	executable, library := ktxIdentityFixture(t, t.TempDir(), "linux")

	alias := filepath.Join(filepath.Dir(library), "libktx.so")
	if err := os.Symlink(filepath.Base(library), alias); err != nil {
		t.Skip("file symlinks unavailable", err)
	}

	revisions, err := ktxLibraryRevisions(t.Context(), executable, "linux")
	if err != nil || len(revisions) != 2 || revisions[0].Name >= revisions[1].Name || revisions[0].Version != revisions[1].Version {
		t.Fatal("confined aliases were not hashed in deterministic order", revisions, err)
	}

	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(executable, alias); err != nil {
		t.Fatal(err)
	}

	if _, err := ktxLibraryRevisions(t.Context(), executable, "linux"); !errors.Is(err, ErrCacheRecipe) {
		t.Fatal("library alias escaped its directory", err)
	}
}

func TestKTXDependencyBoundsAndCancellation(t *testing.T) {
	t.Parallel()
	executable, library := ktxIdentityFixture(t, t.TempDir(), "linux")

	file, err := os.OpenFile(library, os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	if err := file.Truncate(ktxIdentityFileLimit + 1); err != nil {
		_ = file.Close()

		t.Fatal(err)
	}

	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := ktxLibraryRevisions(t.Context(), executable, "linux"); !errors.Is(err, ErrCacheRecipe) {
		t.Fatal("oversized encoder dependency accepted", err)
	}

	if err := os.WriteFile(library, []byte("bounded library"), 0o600); err != nil {
		t.Fatal(err)
	}

	for index := range ktxIdentityLibraryLimit {
		name := filepath.Join(filepath.Dir(library), fmt.Sprintf("libktx.so.alias-%02d", index))
		if err := os.WriteFile(name, []byte("library alias bytes"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := ktxLibraryRevisions(t.Context(), executable, "linux"); !errors.Is(err, ErrCacheRecipe) {
		t.Fatal("unbounded encoder dependency count accepted", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := ktxLibraryRevisions(ctx, executable, "linux"); !errors.Is(err, context.Canceled) {
		t.Fatal("dependency hashing ignored cancellation", err)
	}
}
