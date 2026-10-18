package tiled_test

import (
	"bytes"
	"encoding/binary"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/karty-game/tiled"
	"github.com/karty-game/tiled/source"
)

func compiledFixture(t testing.TB) []byte {
	t.Helper()
	m, err := source.Load(os.DirFS("testdata/orthogonal-flips"), "map.tmj")
	if err != nil {
		t.Fatal(err)
	}
	data, err := tiled.Encode(m)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func TestCompiledMapPreservesOfficialTiledTransforms(t *testing.T) {
	original, err := source.Load(os.DirFS("testdata/orthogonal-flips"), "map.tmj")
	if err != nil {
		t.Fatal(err)
	}
	data := compiledFixture(t)
	m, err := tiled.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m.Sprites, original.Sprites) || !reflect.DeepEqual(m.Objects, original.Objects) ||
		!reflect.DeepEqual(m.LayerNames, original.LayerNames) ||
		m.Width != original.Width ||
		m.TileWidth != original.TileWidth {
		t.Fatal("compiled map changed placements or gameplay metadata")
	}
	if len(m.Layers) != 0 || len(m.Tilesets) != 0 {
		t.Fatal("editable documents reached runtime")
	}
	again, err := tiled.Encode(original)
	if err != nil || !bytes.Equal(data, again) {
		t.Fatal("nondeterministic compilation", err)
	}
	for i := 0; i < len(data); i++ {
		if _, err := tiled.Decode(data[:i]); err == nil {
			t.Fatalf("accepted truncation %d", i)
		}
	}
	for _, bad := range [][]byte{append(append([]byte{}, data...), 0), []byte(`{"orientation":"orthogonal"}`)} {
		if _, err := tiled.Decode(bad); err == nil {
			t.Fatal("accepted legacy or trailing input")
		}
	}
	bad := append([]byte{}, data...)
	binary.LittleEndian.PutUint16(bad[4:], 2)
	if _, err := tiled.Decode(bad); err == nil {
		t.Fatal("accepted unknown binary version")
	}
}
func TestCompiledPropertiesAnimationAndCollisionRoundTrip(t *testing.T) {
	m := &tiled.Map{
		Document: tiled.Document{
			Width:      2,
			Height:     1,
			TileWidth:  32,
			TileHeight: 32,
			Properties: tiled.Properties{
				{Name: "message", Value: "a, \"quoted\" clue\n"},
				{Name: "speed", Value: float64(3.5)},
				{Name: "open", Value: true},
				{Name: "unset"},
			},
		},
		LayerNames: []string{"ground"},
	}
	sprite := tiled.Sprite{
		Image:        "art/atlas.png",
		SourceWidth:  16,
		SourceHeight: 24,
		Width:        40,
		Height:       60,
		Opacity:      .5,
		FlipX:        true,
		Diagonal:     true,
		Properties:   tiled.Properties{{Name: "windPeriod", Value: float64(5)}},
	}
	sprite.Frames = []tiled.Frame{{Sprite: sprite, Duration: 240}}
	m.Sprites = []tiled.Sprite{sprite}
	m.Objects = []tiled.Region{{Name: "wall", Class: "collision", Polygon: []tiled.Point{{X: 1, Y: 2}, {X: 3, Y: 4}, {X: 5, Y: 6}}}}
	data, err := tiled.Encode(m)
	if err != nil {
		t.Fatal(err)
	}
	got, err := tiled.Decode(data)
	if err != nil || !reflect.DeepEqual(got.Sprites, m.Sprites) || !reflect.DeepEqual(got.Objects, m.Objects) ||
		!reflect.DeepEqual(got.Properties, m.Properties) {
		t.Fatal("lost compiled properties, frames or collision", err)
	}
	m.Properties = append(m.Properties, tiled.Property{Name: "class", Value: map[string]any{"nested": "unsupported"}})
	if _, err := tiled.Encode(m); err == nil {
		t.Fatal("accepted an unsupported property shape")
	}
}
func TestRuntimePackageClosuresExcludeSourceDecoders(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), "go", "list", "-deps", ".", "./karty2d")
	cmd.Env = append(os.Environ(), "GOWORK=off")
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("inspect runtime packages: %v\n%s", err, data)
	}
	for _, name := range strings.Fields(string(data)) {
		if strings.HasPrefix(name, "encoding/json") || name == "encoding/csv" || name == "encoding/xml" || name == "encoding/gob" {
			t.Fatalf("runtime imports %s", name)
		}
	}
}
func TestCompilationRejectsInvalidLayersAndSourceRectangles(t *testing.T) {
	for _, change := range []func(*tiled.Sprite){
		func(s *tiled.Sprite) { s.Layer = 1 },
		func(s *tiled.Sprite) { s.SourceX = tiled.MaxImageDimension },
		func(s *tiled.Sprite) { s.SourceY = tiled.MaxImageDimension },
		func(s *tiled.Sprite) {
			s.Frames = []tiled.Frame{{Sprite: *s, Duration: 1}}
			s.Frames[0].Sprite.Layer = 1
		},
	} {
		m, err := tiled.Decode(compiledFixture(t))
		if err != nil {
			t.Fatal(err)
		}
		// The fixture has one layer, indexed zero.
		m.LayerNames = []string{"ground"}
		m.Sprites = m.Sprites[:1]
		m.Sprites[0].Layer = 0
		change(&m.Sprites[0])
		if _, err := tiled.Encode(m); err == nil {
			t.Fatal("compiled invalid layer or source rectangle")
		}
	}
}
func FuzzCompiledMap(f *testing.F) {
	f.Add(compiledFixture(f))
	f.Add([]byte("KTM1"))
	f.Fuzz(func(t *testing.T, data []byte) { _, _ = tiled.Decode(data) })
}
func TestCompiledMapLoadKeepsPathConfinement(t *testing.T) {
	for _, name := range []string{"../escape", "/absolute", "C:drive", "x\\y"} {
		if _, err := tiled.Load(fstest.MapFS{}, name); err == nil {
			t.Fatal("accepted unsafe compiled path", name)
		}
	}
}
