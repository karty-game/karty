package levelbuild

import (
	"os"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty/internal/project"
	"github.com/karty-game/karty/internal/sdk"
)

// BenchmarkEditorChunkGeometry excludes source/asset compilation, game code,
// transport and GPU uploads. Supply a built root KLD for representative content.
//
//nolint:gocognit // Four separate production-stage benchmarks share the same representative fixture and prepared plan.
func BenchmarkEditorChunkGeometry(b *testing.B) {
	path := os.Getenv("KARTY_BENCH_CHUNK_ROOT")
	if path == "" {
		b.Skip("set KARTY_BENCH_CHUNK_ROOT to a representative built root KLD")
	}

	module, err := os.ReadFile(path) //nolint:gosec // Operator explicitly selects a local fixture for this opt-in benchmark.
	if err != nil {
		b.Fatal(err)
	}

	envelope := executeLightmapEnvelope(b, module)

	encoded, _, found := envelope.Read(world.EntryName, 0, world.MaxEncodedSize)
	if !found {
		b.Fatal("root has no complete world")
	}

	document, err := world.Decode(encoded)
	if err != nil {
		b.Fatal(err)
	}

	definition := manifest{}
	definition.Level.Name = "editor-preview"
	definition.Chunks = &chunkBuildSettings{Enabled: true}

	options, err := chunkOptions(definition)
	if err != nil {
		b.Fatal(err)
	}

	plan, err := worldchunk.Plan(document, options)
	if err != nil {
		b.Fatal(err)
	}

	b.Logf("world: %d sectors; mesh-only plan: %d chunks", len(document.Sectors), len(plan.Chunks))

	b.Run("PlanMeshOnly", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			if _, err := worldchunk.Plan(document, options); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("EncodeAllChildren", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			for _, chunk := range plan.Chunks {
				if _, _, err := encodeWorldChunk(
					b.Context(),
					document,
					definition.Level.Name,
					chunk,
					nil,
					chunkBakeOptions(nil),
					true,
				); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
	b.Run("ShadowProxy", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			if _, err := encodeWorldChunkShadow(document); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("CompleteMeshOnlyBuild", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			result, err := buildWorldChunks(b.Context(), "", definition, document, worldMaterialBuild{metadata: []byte(`{}`)})
			if err != nil {
				b.Fatal(err)
			}

			var size int
			for _, chunk := range result.chunks {
				size += len(chunk.Bytes)
			}

			b.ReportMetric(float64(size), "child-bytes")
			b.ReportMetric(float64(len(result.chunks)), "chunks")
		}
	})
}

// BenchmarkEditorLevelBuild includes authored source and asset processing, with
// no game compilation or host launch. Cache warmth belongs to the supplied
// isolated project; a first run may populate its generated preview cache.
func BenchmarkEditorLevelBuild(b *testing.B) {
	root := os.Getenv("KARTY_BENCH_EDITOR_PROJECT")
	if root == "" {
		b.Skip("set KARTY_BENCH_EDITOR_PROJECT to an isolated representative project")
	}

	config, err := project.Load(root)
	if err != nil {
		b.Fatal(err)
	}

	selected, err := sdk.Resolve(config.SDK.Version)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		artifacts, err := BuildAllWithOptions(
			b.Context(),
			root,
			selected.Compatibility.UISchema,
			config.Assets.Theme.Source,
			selected,
			Options{Development: true},
		)
		if err != nil {
			b.Fatal(err)
		}

		var chunks int
		for _, artifact := range artifacts {
			chunks += len(artifact.Chunks)
		}

		b.ReportMetric(float64(chunks), "chunks")
	}
}
