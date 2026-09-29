package assetpipeline_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/karty-game/karty/internal/assetpipeline"
)

func TestProjectCacheColdWarmAndEquivalentRecipeReuse(t *testing.T) {
	t.Parallel()

	cache := assetpipeline.NewProjectCache(t.TempDir())
	source := snapshot(t, []byte("source"))
	firstRecipe := recipe(t, map[string]any{"width": 32, "filter": "nearest"})
	secondRecipe := recipe(t, map[string]any{"filter": "nearest", "width": 32})
	processor, calls := fakeProcessor()

	first, err := cache.Resolve(context.Background(), source, firstRecipe, processor)
	if err != nil {
		t.Fatal(err)
	}

	second, err := cache.Resolve(context.Background(), source, secondRecipe, processor)
	if err != nil {
		t.Fatal(err)
	}

	if first.CacheHit || !second.CacheHit || calls.Load() != 1 {
		t.Fatalf("cold/warm results = %+v / %+v, calls = %d", first, second, calls.Load())
	}

	if first.CacheKey != second.CacheKey || first.RecipeDigest != second.RecipeDigest || first.OutputDigest != second.OutputDigest {
		t.Fatal("equivalent recipes did not reuse the same artifact")
	}
}

func TestProjectCacheInvalidatesOnlyChangedInputs(t *testing.T) {
	t.Parallel()

	cache := assetpipeline.NewProjectCache(t.TempDir())
	processor, calls := fakeProcessor()
	recipeA := recipe(t, map[string]any{"size": 1})
	recipeB := recipe(t, map[string]any{"size": 2})
	sourceA := snapshot(t, []byte("asset-a"))
	sourceB := snapshot(t, []byte("asset-b"))

	for _, request := range []struct {
		source assetpipeline.SourceSnapshot
		recipe assetpipeline.Recipe
	}{
		{sourceA, recipeA}, {sourceB, recipeA}, {sourceA, recipeA}, {sourceB, recipeB}, {sourceA, recipeA},
	} {
		if _, err := cache.Resolve(context.Background(), request.source, request.recipe, processor); err != nil {
			t.Fatal(err)
		}
	}

	if calls.Load() != 3 {
		t.Fatalf("processor calls = %d, want 3", calls.Load())
	}
}

func TestProjectCacheRepairsCorruptPayloadAndMetadata(t *testing.T) {
	t.Parallel()

	for name, corrupt := range map[string]func(t *testing.T, artifact assetpipeline.Artifact){
		"payload": func(t *testing.T, artifact assetpipeline.Artifact) {
			t.Helper()

			if err := os.WriteFile(artifact.PayloadPath, []byte("corrupt"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"metadata": func(t *testing.T, artifact assetpipeline.Artifact) {
			t.Helper()

			if err := os.WriteFile(filepath.Join(artifact.EntryPath, "metadata.json"), []byte("{"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cache := assetpipeline.NewProjectCache(t.TempDir())
			processor, calls := fakeProcessor()
			source := snapshot(t, []byte("repair-me"))
			recipe := recipe(t, map[string]any{"mode": "test"})

			first, err := cache.Resolve(context.Background(), source, recipe, processor)
			if err != nil {
				t.Fatal(err)
			}

			corrupt(t, first)

			repaired, err := cache.Resolve(context.Background(), source, recipe, processor)
			if err != nil {
				t.Fatal(err)
			}

			if repaired.CacheHit || calls.Load() != 2 {
				t.Fatalf("repair = %+v, calls = %d", repaired, calls.Load())
			}

			contents, err := os.ReadFile(repaired.PayloadPath)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(contents, []byte("REPAIR-ME")) {
				t.Fatalf("repaired payload = %q", contents)
			}
		})
	}
}

func TestProjectCacheConcurrentWritersConverge(t *testing.T) {
	t.Parallel()

	cache := assetpipeline.NewProjectCache(t.TempDir())
	source := snapshot(t, []byte("concurrent"))
	recipe := recipe(t, map[string]any{"mode": "shared"})

	var calls atomic.Int64

	processor := func(ctx context.Context, source assetpipeline.SourceSnapshot, _ assetpipeline.Recipe) (assetpipeline.Processed, error) {
		if err := ctx.Err(); err != nil {
			return assetpipeline.Processed{}, err
		}

		calls.Add(1)
		time.Sleep(20 * time.Millisecond)

		return assetpipeline.Processed{
			Encoding: "test@1", Payload: bytes.ToUpper(source.Bytes()), Metadata: map[string]any{"ok": true},
		}, nil
	}

	const writers = 16

	artifacts := make(chan assetpipeline.Artifact, writers)
	errors := make(chan error, writers)

	var group sync.WaitGroup
	for range writers {
		group.Go(func() {
			artifact, err := cache.Resolve(context.Background(), source, recipe, processor)
			if err != nil {
				errors <- err

				return
			}

			artifacts <- artifact
		})
	}

	group.Wait()
	close(errors)
	close(artifacts)

	for err := range errors {
		t.Error(err)
	}

	var key string
	for artifact := range artifacts {
		if key == "" {
			key = artifact.CacheKey
		} else if artifact.CacheKey != key {
			t.Fatalf("cache keys differ: %q != %q", artifact.CacheKey, key)
		}
	}

	if calls.Load() != 1 {
		t.Fatalf("concurrent processor calls = %d, want 1", calls.Load())
	}
}

func TestProjectCacheIgnoresInterruptedTemporaryEntry(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	interrupted := filepath.Join(root, ".karty", "cache", "assets", "v2", ".tmp-interrupted")
	if err := os.MkdirAll(interrupted, 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(interrupted, "payload"), []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}

	processor, calls := fakeProcessor()

	artifact, err := assetpipeline.NewProjectCache(root).Resolve(
		context.Background(), snapshot(t, []byte("complete")), recipe(t, map[string]any{"mode": "safe"}), processor,
	)
	if err != nil {
		t.Fatal(err)
	}

	if artifact.CacheHit || calls.Load() != 1 {
		t.Fatalf("artifact = %+v, calls = %d", artifact, calls.Load())
	}

	if _, err := os.Stat(interrupted); err != nil {
		t.Fatalf("unrelated interrupted entry was not safely ignored: %v", err)
	}
}

func TestSnapshotFileHashesContentsDespiteSameSizeAndMtime(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "source.bin")
	stamp := time.Unix(1_700_000_000, 0)
	writeWithTime(t, path, []byte("AAAA"), stamp)

	first, err := assetpipeline.SnapshotFile(path)
	if err != nil {
		t.Fatal(err)
	}

	writeWithTime(t, path, []byte("BBBB"), stamp)

	second, err := assetpipeline.SnapshotFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if first.Digest() == second.Digest() || first.Size() != second.Size() {
		t.Fatal("same-size, same-mtime source edit did not change its content identity")
	}

	processor, calls := fakeProcessor()
	cache := assetpipeline.NewProjectCache(root)
	recipe := recipe(t, map[string]any{"mode": "hash"})

	firstArtifact, err := cache.Resolve(context.Background(), first, recipe, processor)
	if err != nil {
		t.Fatal(err)
	}

	secondArtifact, err := cache.Resolve(context.Background(), second, recipe, processor)
	if err != nil {
		t.Fatal(err)
	}

	if firstArtifact.CacheKey == secondArtifact.CacheKey || calls.Load() != 2 {
		t.Fatal("content change did not invalidate the cache")
	}
}

func TestProjectCacheRejectsUnboundedMetadata(t *testing.T) {
	t.Parallel()

	cache := assetpipeline.NewProjectCache(t.TempDir())

	_, err := cache.Resolve(context.Background(), snapshot(t, []byte("source")), recipe(t, map[string]any{"mode": "metadata"}),
		func(context.Context, assetpipeline.SourceSnapshot, assetpipeline.Recipe) (assetpipeline.Processed, error) {
			return assetpipeline.Processed{
				Encoding: "test@1",
				Payload:  []byte("output"),
				Metadata: map[string]string{"large": string(make([]byte, assetpipeline.MaxCacheMetadataBytes))},
			}, nil
		})
	if err == nil {
		t.Fatal("Resolve() accepted unbounded metadata")
	}
}

func fakeProcessor() (assetpipeline.Processor, *atomic.Int64) {
	var calls atomic.Int64

	return func(_ context.Context, source assetpipeline.SourceSnapshot, recipe assetpipeline.Recipe) (assetpipeline.Processed, error) {
		calls.Add(1)

		metadata := map[string]any{"recipe": recipe.Digest(), "source": source.Digest()}

		return assetpipeline.Processed{Encoding: "test@1", Payload: bytes.ToUpper(source.Bytes()), Metadata: metadata}, nil
	}, &calls
}

func snapshot(t *testing.T, contents []byte) assetpipeline.SourceSnapshot {
	t.Helper()

	result, err := assetpipeline.SnapshotBytes(contents)
	if err != nil {
		t.Fatal(err)
	}

	return result
}

func recipe(t *testing.T, options any) assetpipeline.Recipe {
	t.Helper()

	result, err := assetpipeline.NewRecipe("texture", "test@1", []assetpipeline.Revision{
		{Name: "codec", Version: "1"}, {Name: "normalizer", Version: "1"},
	}, options)
	if err != nil {
		t.Fatal(err)
	}

	return result
}

func writeWithTime(t *testing.T, path string, contents []byte, stamp time.Time) {
	t.Helper()

	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
}
