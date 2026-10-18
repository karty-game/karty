package assets_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/karty-game/tiled"
	"github.com/karty-game/tiled/assets"
	"github.com/karty-game/tiled/source"
)

func imageBytes(t *testing.T, im image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if e := png.Encode(&b, im); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func solid(r image.Rectangle, c color.NRGBA) *image.NRGBA {
	im := image.NewNRGBA(r)
	draw.Draw(im, r, &image.Uniform{C: c}, image.Point{}, draw.Src)
	return im
}

type countedFS struct {
	fs.FS
	opens map[string]int
}

func (f countedFS) Open(name string) (fs.File, error) { f.opens[name]++; return f.FS.Open(name) }
func collect(out map[string]assets.Input) func(assets.Input) (string, error) {
	return func(in assets.Input) (string, error) { out[in.Key] = in; return in.Key, nil }
}

func TestAtlasMarginsSpacingFlipsAndUnplacedAnimationFrames(t *testing.T) {
	im := image.NewNRGBA(image.Rect(0, 0, 10, 4))
	colors := []color.NRGBA{{R: 20, G: 150, B: 40, A: 255}, {R: 190, G: 40, B: 30, A: 255}, {R: 30, G: 60, B: 190, A: 255}}
	for i, c := range colors {
		r := image.Rect(1+i*3, 1, 3+i*3, 3)
		draw.Draw(im, r, &image.Uniform{C: c}, image.Point{}, draw.Src)
	}
	gids := make([]uint32, 8)
	for i := range gids {
		gids[i] = 1 | uint32(i)<<29
	}
	raw, e := json.Marshal(
		map[string]any{
			"orientation": "orthogonal",
			"width":       8,
			"height":      1,
			"tilewidth":   2,
			"tileheight":  2,
			"tilesets":    []any{map[string]any{"firstgid": 1, "source": "../tiles/set.tsj"}},
			"layers":      []any{map[string]any{"type": "tilelayer", "name": "all-flips", "width": 8, "height": 1, "data": gids}},
		},
	)
	if e != nil {
		t.Fatal(e)
	}
	files := fstest.MapFS{
		"maps/map.tmj": {Data: raw},
		"tiles/set.tsj": {
			Data: []byte(
				`{"tilewidth":2,"tileheight":2,"columns":3,"tilecount":3,"margin":1,"spacing":1,"image":"atlas.png","imagewidth":10,"imageheight":4,"tiles":[{"id":0,"animation":[{"tileid":0,"duration":80},{"tileid":1,"duration":90},{"tileid":2,"duration":100}]}]}`,
			),
		},
		"tiles/atlas.png": {Data: imageBytes(t, im)},
	}
	level, e := source.Load(files, "maps/map.tmj")
	if e != nil {
		t.Fatal(e)
	}
	opens := map[string]int{}
	inputs := map[string]assets.Input{}
	aliases, e := assets.Import(countedFS{files, opens}, level, collect(inputs))
	if e != nil {
		t.Fatal(e)
	}
	if len(level.Sprites) != 8 || len(inputs) != 3 || len(aliases) != 3 || opens["tiles/atlas.png"] != 1 {
		t.Fatalf("flip/frame dedup or image cache failed: sprites=%d inputs=%d opens=%v", len(level.Sprites), len(inputs), opens)
	}
	for i, c := range colors {
		key := fmt.Sprintf("tiles/atlas.png#%d,1,2,2", 1+i*3)
		in, ok := inputs[key]
		if !ok || aliases[key] != key || in.Source != "tiles/atlas.png" || in.Rectangle != image.Rect(1+i*3, 1, 3+i*3, 3) ||
			in.SourceBounds != im.Bounds() ||
			in.Pixels.Bounds() != image.Rect(0, 0, 2, 2) {
			t.Fatalf("incorrect metadata-derived rectangle for tile%d: %+v", i, in)
		}
		for y := range 2 {
			for x := range 2 {
				if in.Pixels.NRGBAAt(x, y) != c {
					t.Fatal("flips changed stored source pixels or atlas crop crossed its gutter")
				}
			}
		}
	}
}
func TestImageCollectionsUseFullPathsAndImportEveryFrame(t *testing.T) {
	a := solid(image.Rect(0, 0, 3, 2), color.NRGBA{R: 35, G: 80, B: 120, A: 190})
	b := solid(a.Bounds(), color.NRGBA{R: 180, G: 120, B: 50, A: 255})
	files := fstest.MapFS{
		"map.tmj": {
			Data: []byte(
				`{"orientation":"orthogonal","width":1,"height":1,"tilewidth":3,"tileheight":2,"tilesets":[{"firstgid":1,"source":"tiles.tsj"}],"layers":[{"type":"tilelayer","name":"one","width":1,"height":1,"data":[1]}]}`,
			),
		},
		"tiles.tsj": {
			Data: []byte(
				`{"tilewidth":3,"tileheight":2,"columns":0,"tilecount":2,"tiles":[{"id":0,"image":"a/same.png","imagewidth":3,"imageheight":2,"animation":[{"tileid":0,"duration":100},{"tileid":1,"duration":100}]},{"id":1,"image":"b/same.png","imagewidth":3,"imageheight":2}]}`,
			),
		},
		"a/same.png": {Data: imageBytes(t, a)},
		"b/same.png": {Data: imageBytes(t, b)},
	}
	level, e := source.Load(files, "map.tmj")
	if e != nil {
		t.Fatal(e)
	}
	opens := map[string]int{}
	inputs := map[string]assets.Input{}
	aliases, e := assets.Import(countedFS{files, opens}, level, collect(inputs))
	if e != nil {
		t.Fatal(e)
	}
	if len(aliases) != 2 || opens["a/same.png"] != 1 || opens["b/same.png"] != 1 {
		t.Fatalf("full-path identity or frames lost: aliases=%v opens=%v", aliases, opens)
	}
	for source, want := range map[string]*image.NRGBA{"a/same.png": a, "b/same.png": b} {
		in := inputs[source+"#0,0,3,2"]
		if in.Rectangle != in.SourceBounds || in.Pixels.NRGBAAt(1, 1) != want.NRGBAAt(1, 1) {
			t.Fatal("whole image or alpha was changed")
		}
	}
}
func TestRejectsInvalidSourceRectanglesBeforeEmission(t *testing.T) {
	im := solid(image.Rect(0, 0, 4, 4), color.NRGBA{A: 255})
	files := fstest.MapFS{"image.png": {Data: imageBytes(t, im)}}
	for _, r := range [][4]int{{-1, 0, 1, 1}, {0, -1, 1, 1}, {0, 0, 0, 1}, {0, 0, 1, 0}, {3, 0, 2, 1}, {0, 3, 1, 2}, {int(^uint(0) >> 1), 0, 2, 1}} {
		s := tiled.Sprite{Image: "image.png", SourceX: r[0], SourceY: r[1], SourceWidth: r[2], SourceHeight: r[3]}
		if _, e := assets.Import(
			files,
			&tiled.Map{Sprites: []tiled.Sprite{s}},
			func(assets.Input) (string, error) { t.Fatal("invalid source rectangle emitted pixels"); return "", nil },
		); e == nil {
			t.Fatalf("accepted rectangle%v", r)
		}
	}
}
func TestRejectsEscapingPathsBeforeFilesystemAccess(t *testing.T) {
	for _, name := range []string{"", "../outside.png", "a/../../outside.png", "/outside.png", "./image.png", "a\\image.png", "C:/image.png", "a\x00image.png"} {
		opens := map[string]int{}
		s := tiled.Sprite{Image: name, SourceWidth: 1, SourceHeight: 1}
		if _, e := assets.Import(
			countedFS{fstest.MapFS{}, opens},
			&tiled.Map{Sprites: []tiled.Sprite{s}},
			func(assets.Input) (string, error) { t.Fatal("invalid path emitted pixels"); return "", nil },
		); e == nil ||
			len(opens) != 0 {
			t.Fatalf("invalid path reached filesystem:%q (%v)", name, opens)
		}
	}
}
func TestCallbackFailuresAndInvalidInputs(t *testing.T) {
	im := solid(image.Rect(0, 0, 1, 1), color.NRGBA{A: 255})
	files := fstest.MapFS{"image.png": {Data: imageBytes(t, im)}}
	level := &tiled.Map{Sprites: []tiled.Sprite{{Image: "image.png", SourceWidth: 1, SourceHeight: 1}}}
	sentinel := errors.New("output failed")
	if aliases, e := assets.Import(
		files,
		level,
		func(assets.Input) (string, error) { return "", sentinel },
	); !errors.Is(e, sentinel) ||
		aliases != nil {
		t.Fatal("callback failure was lost")
	}
	if _, e := assets.Import(files, level, func(assets.Input) (string, error) { return "", nil }); e == nil {
		t.Fatal("accepted empty output name")
	}
	emit := func(assets.Input) (string, error) { return "output", nil }
	for _, args := range []struct {
		fs    fs.FS
		level *tiled.Map
		emit  func(assets.Input) (string, error)
	}{{nil, level, emit}, {files, nil, emit}, {files, level, nil}} {
		if _, e := assets.Import(args.fs, args.level, args.emit); e == nil {
			t.Fatal("accepted nil import argument")
		}
	}
	if _, e := assets.Import(fstest.MapFS{}, level, emit); !errors.Is(e, fs.ErrNotExist) {
		t.Fatal("filesystem error was lost")
	}
	if _, e := assets.Import(fstest.MapFS{"image.png": {Data: []byte("invalid PNG")}}, level, emit); e == nil {
		t.Fatal("accepted a corrupt PNG")
	}
}

func TestCallbackOwnsPixelsWithoutMutatingTheDecodedSourceCache(t *testing.T) {
	im := image.NewNRGBA(image.Rect(0, 0, 3, 1))
	for x := range 3 {
		im.SetNRGBA(x, 0, color.NRGBA{R: uint8(30 + x), G: 80, B: 120, A: 255})
	}
	a := tiled.Sprite{Image: "atlas.png", SourceWidth: 2, SourceHeight: 1}
	b := a
	b.SourceX = 1
	files := fstest.MapFS{"atlas.png": {Data: imageBytes(t, im)}}
	calls := 0
	_, err := assets.Import(files, &tiled.Map{Sprites: []tiled.Sprite{a, b}}, func(in assets.Input) (string, error) {
		calls++
		if calls == 1 {
			in.Pixels.SetNRGBA(1, 0, color.NRGBA{R: 255, A: 255})
		} else if in.Pixels.NRGBAAt(0, 0) != im.NRGBAAt(1, 0) {
			t.Fatal("callback mutated the shared decoded source")
		}
		return in.Key, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("distinct overlapping rectangles were omitted")
	}
}
