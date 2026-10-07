package geometry

import (
	"errors"
	"testing"

	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

func TestDirectBandReferencesAndDisabledOverrides(t *testing.T) {
	t.Parallel()

	height, repeat := .5, 2.0
	disabled := false
	settings := &worldsource.BandSettings{
		Top:    &worldsource.HorizontalBandSettings{Texture: "top", Height: &height, RepeatWidth: &repeat},
		Bottom: &worldsource.HorizontalBandSettings{Texture: "bottom", Height: &height},
	}
	room := source.Room{Boundary: []worldsource.Edge{{Bands: settings}}}
	materials := map[string]uint32{"top": 2, "bottom": 3}

	config, err := compileFrameConfig(room, 0, materials, 4)
	if err != nil {
		t.Fatal(err)
	}

	if config.Top.Piece.Material != 2 || config.Bottom.Piece.Material != 3 || config.Top.Repeat != 2 || config.PerimeterOffset != 4 ||
		config.Start.Width != 0 ||
		config.End.Width != 0 {
		t.Fatalf("direct band config: %+v", config)
	}

	settings.Top.Texture = "missing"

	if _, err := compileFrameConfig(room, 0, materials, 0); !errors.Is(err, ErrMaterial) {
		t.Fatalf("missing band accepted: %v", err)
	}

	settings.Top.Enabled = &disabled
	if config, err := compileFrameConfig(room, 0, materials, 0); err != nil || config.Top.Height != 0 || config.Bottom.Piece.Material != 3 {
		t.Fatalf("disabled band resolved: %+v %v", config, err)
	}

	settings.Bottom.Height = nil

	if _, err := compileFrameConfig(room, 0, materials, 0); !errors.Is(err, worldsource.ErrBounds) {
		t.Fatalf("missing height accepted: %v", err)
	}
}
