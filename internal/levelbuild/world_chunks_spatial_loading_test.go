package levelbuild

import (
	"math"
	"testing"
)

func TestSpatialLoadingSettingsStayIndependentFromFineGeometry(t *testing.T) {
	t.Parallel()

	for _, size := range []float64{0, 24, 48, 96, 12, 72, 8192, math.NaN(), math.Inf(1)} {
		settings := &spatialChunkSettings{TileSize: 24, LoadTileSize: size}
		err := validateSpatialLoadingSettings(settings)

		valid := size == 0 || size == 24 || size == 48 || size == 96
		if valid != (err == nil) {
			t.Fatal("invalid independent loading size decision", size, err)
		}
	}
}
