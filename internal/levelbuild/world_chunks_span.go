package levelbuild

import (
	"fmt"
	"math"
)

const (
	defaultChunkSpanMeters = 48
	spanRepartitionFactor  = 2
)

func chunkSpanLimit(settings *chunkBuildSettings) (float64, error) {
	maximum := float64(defaultChunkSpanMeters)

	if settings.MaxSpan != nil {
		if math.IsNaN(*settings.MaxSpan) || math.IsInf(*settings.MaxSpan, 0) || *settings.MaxSpan <= 0 {
			return 0, fmt.Errorf("max_span must be positive finite meters: %w", ErrManifest)
		}

		maximum = *settings.MaxSpan
	}

	if settings.Spatial != nil && settings.Spatial.BackgroundTileSize > maximum {
		return 0, fmt.Errorf("spatial background_tile_size exceeds max_span; raise the level limit for outdoor coverage: %w", ErrManifest)
	}

	return maximum, nil
}

// Include terrain render resources and spatial children before any bake. The
// primary planner only sees its selected semantic mesh, not cropped terrain.
