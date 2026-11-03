package levelbuild

import (
	"errors"
	"math"
	"testing"
)

func TestChunkBuildEnablesEnclosurePreferenceWithoutNewAuthoringSettings(t *testing.T) {
	t.Parallel()

	definition := manifest{Chunks: &chunkBuildSettings{Enabled: true}}

	options, err := chunkOptions(definition)
	if err != nil || !options.EnclosureAware || options.MaxSpan != 48 {
		t.Fatal("ordinary chunk builds omitted enclosure-aware planning", err)
	}
}

func TestChunkSpanSettings(t *testing.T) {
	t.Parallel()

	for _, maximum := range []float64{32, 48, 0, -1, math.NaN(), math.Inf(1)} {
		def := manifest{Chunks: &chunkBuildSettings{Enabled: true, MaxSpan: &maximum}}
		options, err := chunkOptions(def)

		valid := maximum > 0 && !math.IsNaN(maximum) && !math.IsInf(maximum, 0)
		if valid && (err != nil || options.MaxSpan != maximum) || !valid && err == nil {
			t.Fatalf("max_span %g: options=%+v err=%v", maximum, options, err)
		}
	}
}

func TestSpatialTilesMustFitConfiguredChunkSpan(t *testing.T) {
	t.Parallel()

	def := manifest{Chunks: &chunkBuildSettings{Enabled: true, Spatial: spatialTestSettings()},
		Lightmap: &lightmapBuildSettings{Enabled: true, Fixed: true, Offline: true, Density: new(float64(4))}}
	if _, err := chunkOptions(def); !errors.Is(err, ErrManifest) {
		t.Fatal("spatial tile bypassed default span", err)
	}

	def.Chunks.MaxSpan = new(float64(256))
	if _, err := chunkOptions(def); err != nil {
		t.Fatal("explicit outdoor span rejected", err)
	}
}
