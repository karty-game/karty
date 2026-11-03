package levelbuild

import (
	"os"
	"testing"

	"github.com/karty-game/karty/internal/project"
	"github.com/karty-game/karty/internal/sdk"
)

// BenchmarkEditorChunkGeometry excludes source/asset compilation, game code,
// transport and GPU uploads. Supply a built root KLD for representative content.
//

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
