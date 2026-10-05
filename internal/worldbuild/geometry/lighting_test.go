package geometry_test

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"

	sdkworld "github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty/internal/worldbuild/geometry"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

func TestCompileLightingOwnsPayloadAndPreservesOrder(t *testing.T) {
	t.Parallel()

	expanded := lightingExpanded(50)
	expanded.Lighting.AmbientCube = &sdkworld.AmbientCube{NegativeZ: sdkworld.Vec3{Y: .4}}
	expanded.Lighting.Actors = true
	expanded.Lighting.Lights[0].Motion = &sdkworld.LightMotion{Version: 1, PeriodSeconds: 12}

	document, err := geometry.Compile(expanded, map[string]uint32{"wall": 1})
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(document.Lighting, expanded.Lighting) {
		t.Fatalf("compiled lighting changed: %+v", document.Lighting)
	}

	encoded, err := sdkworld.Encode(document)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := sdkworld.Decode(encoded)
	if err != nil || !reflect.DeepEqual(decoded.Lighting, expanded.Lighting) {
		t.Fatalf("encoded lighting changed: %+v, %v", decoded.Lighting, err)
	}

	document.Lighting.Ambient.X = .9
	document.Lighting.AmbientCube.NegativeZ.Y = .9

	document.Lighting.Lights[0].ID = "changed"

	document.Lighting.Lights[0].Motion.PeriodSeconds = 20
	if expanded.Lighting.Lights[0].Motion.PeriodSeconds != 12 || expanded.Lighting.Ambient.X != .1 ||
		expanded.Lighting.AmbientCube.NegativeZ.Y != .4 ||
		!document.Lighting.Actors ||
		expanded.Lighting.Lights[0].ID != "light-50" {
		t.Fatal("compiled lighting aliases the source")
	}

	expanded.Lighting.Lights[1].Position.X = 42
	if document.Lighting.Lights[1].Position.X != -1 {
		t.Fatal("source lighting aliases compiled light storage")
	}
}

func TestCompileValidatesEntireExpandedLightingBeforeGeometry(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		mutate func(*sdkworld.Lighting)
		want   error
	}{
		"51":      {func(l *sdkworld.Lighting) { l.Lights = append(l.Lights, sdkworld.PointLight{ID: "extra", Radius: 1}) }, sdkworld.ErrBounds},
		"version": {func(l *sdkworld.Lighting) { l.Version = 0 }, sdkworld.ErrVersion},
		"cube last": {func(l *sdkworld.Lighting) {
			l.AmbientCube = &sdkworld.AmbientCube{NegativeZ: sdkworld.Vec3{Z: math.NaN()}}
		}, sdkworld.ErrLighting},
		"ambient":        {func(l *sdkworld.Lighting) { l.Ambient.Z = math.NaN() }, sdkworld.ErrLighting},
		"last duplicate": {func(l *sdkworld.Lighting) { l.Lights[49].ID = l.Lights[0].ID }, sdkworld.ErrIdentity},
		"last radius":    {func(l *sdkworld.Lighting) { l.Lights[49].Radius = -1 }, sdkworld.ErrLighting},
		"last position":  {func(l *sdkworld.Lighting) { l.Lights[49].Position.Y = math.Inf(1) }, sdkworld.ErrLighting},
		"last color":     {func(l *sdkworld.Lighting) { l.Lights[49].Color.Z = 1.1 }, sdkworld.ErrLighting},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			expanded := lightingExpanded(50)
			test.mutate(expanded.Lighting)

			document, err := geometry.Compile(expanded, map[string]uint32{"wall": 1})
			if !errors.Is(err, test.want) || !reflect.DeepEqual(document, sdkworld.Document{}) {
				t.Fatalf("malformed lighting produced partial compiled output: %+v, %v", document, err)
			}
		})
	}
}

func TestCompileAbsentLightingRemainsAbsent(t *testing.T) {
	t.Parallel()

	expanded := lightingExpanded(0)
	expanded.Lighting = nil

	document, err := geometry.Compile(expanded, map[string]uint32{"wall": 1})
	if err != nil || document.Lighting != nil || document.Version != sdkworld.Version {
		t.Fatalf("unlit compilation changed: %+v, %v", document, err)
	}
}

func lightingExpanded(count int) source.Expanded {
	expanded := source.Expanded{
		Rooms: []source.Room{
			{
				ID:              "room",
				SourceRoom:      "room",
				Boundary:        edges([]point{{0, 0}, {2, 0}, {2, 2}, {0, 2}}, []string{"south", "east", "north", "west"}),
				Ceiling:         worldsource.Plane{C: 4},
				FloorMaterial:   "wall",
				CeilingMaterial: "wall",
			},
		},
		Lighting: &sdkworld.Lighting{Version: 1, Ambient: sdkworld.Vec3{X: .1}, Lights: make([]sdkworld.PointLight, count)},
	}
	for index := range count {
		expanded.Lighting.Lights[index] = sdkworld.PointLight{
			ID: fmt.Sprintf("light-%d", count-index), Position: sdkworld.Vec3{X: -float64(index), Y: 10, Z: 2},
			Color: sdkworld.Vec3{X: .5, Y: .25, Z: 1}, Radius: 6,
		}
	}

	return expanded
}
