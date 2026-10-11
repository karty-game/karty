package geometry_test

import (
	"path/filepath"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty/internal/worldbuild/geometry"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

func TestEnvironmentSampleVariantsCompileAndOwnSun(t *testing.T) {
	t.Parallel()

	materials := map[string]uint32{"stone": 1, "tile": 2, "wall": 3, "ceiling": 4, "ground": 5, "wood": 6, "metal": 7}

	for _, preset := range []string{"daylight", "cartoon", "fantasy", "space"} {
		expanded, err := source.Load(filepath.Join("..", "..", "..", "samples", "environment", "levels", preset), "world.yaml")
		if err != nil {
			t.Fatal(err)
		}

		expanded.Environment.Sun = &world.EnvironmentSun{Azimuth: 135, Elevation: 40}

		compiled, err := geometry.Compile(expanded, materials)
		if err != nil {
			t.Fatal(err)
		}

		if compiled.Environment.Preset != world.EnvironmentPreset(preset) || compiled.Environment.Version != world.EnvironmentVersion {
			t.Fatal(compiled.Environment)
		}

		compiled.Environment.Sun.Azimuth = 90
		if expanded.Environment.Sun.Azimuth != 135 {
			t.Fatal("compiled environment aliases authoring")
		}

		skyWalls, skySectors, portals := 0, 0, 0

		for _, sector := range compiled.Sectors {
			if sector.Sky {
				skySectors++
			}

			for _, wall := range sector.Walls {
				if wall.Sky {
					skyWalls++
				}

				if wall.Portal >= 0 {
					portals++
				}
			}
		}

		if skyWalls != 7 || skySectors < 2 || portals < 14 || compiled.StaticSolids == nil {
			t.Fatalf("sample lost outdoor layout: sky walls=%d sectors=%d portals=%d", skyWalls, skySectors, portals)
		}
	}
}

func TestEnvironmentCompileOwnsTimeAndCloudControls(t *testing.T) {
	t.Parallel()

	expanded, err := source.Load(filepath.Join("..", "..", "..", "samples", "environment", "levels", "daylight"), "world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	expanded.Environment.TimeOfDay = new(float64(0))
	expanded.Environment.CloudAmount = new(float64(.45))

	compiled, err := geometry.Compile(
		expanded,
		map[string]uint32{"stone": 1, "tile": 2, "wall": 3, "ceiling": 4, "ground": 5, "wood": 6, "metal": 7},
	)
	if err != nil {
		t.Fatal(err)
	}

	if compiled.Environment.TimeOfDay == nil || *compiled.Environment.TimeOfDay != 0 ||
		compiled.Environment.CloudAmount == nil || *compiled.Environment.CloudAmount != .45 {
		t.Fatal("compiled environment lost control presence", compiled.Environment)
	}

	*compiled.Environment.TimeOfDay = 12

	*compiled.Environment.CloudAmount = 1
	if *expanded.Environment.TimeOfDay != 0 || *expanded.Environment.CloudAmount != .45 {
		t.Fatal("compiled environment aliases authoring controls")
	}
}
