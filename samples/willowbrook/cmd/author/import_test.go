package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/karty-game/tiled"
	"github.com/karty-game/tiled/source"
)

type countedImages struct {
	fs.FS
	reads map[string]int
}

func (f countedImages) Open(name string) (fs.File, error) { f.reads[name]++; return f.FS.Open(name) }
func pngFixture(t *testing.T, im image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, im); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func TestImportTiledAtlasMetadataAndImageIdentity(t *testing.T) {
	atlas := image.NewNRGBA(image.Rect(0, 0, 7, 4))
	fill(atlas, image.Rect(1, 1, 3, 3), rgb(20, 150, 40))
	fill(atlas, image.Rect(4, 1, 6, 3), rgb(190, 40, 30))
	other := image.NewNRGBA(atlas.Bounds())
	fill(other, image.Rect(1, 1, 3, 3), rgb(30, 60, 190))
	set := []byte(
		`{"tilewidth":2,"tileheight":2,"columns":2,"tilecount":2,"margin":1,"spacing":1,"image":"atlas.png","imagewidth":7,"imageheight":4}`,
	)
	files := fstest.MapFS{
		"maps/map.tmj": {
			Data: []byte(
				`{"orientation":"orthogonal","width":3,"height":1,"tilewidth":2,"tileheight":2,"tilesets":[{"firstgid":1,"source":"../a/set.tsj"},{"firstgid":3,"source":"../b/set.tsj"}],"layers":[{"type":"tilelayer","name":"tiles","width":3,"height":1,"data":[1,2,3]}]}`,
			),
		},
		"a/set.tsj": {
			Data: set,
		},
		"b/set.tsj":   {Data: set},
		"a/atlas.png": {Data: pngFixture(t, atlas)},
		"b/atlas.png": {Data: pngFixture(t, other)},
	}
	level, err := source.Load(files, "maps/map.tmj")
	if err != nil {
		t.Fatal(err)
	}
	reads := map[string]int{}
	out := map[string]*image.NRGBA{}
	aliases, err := importTiledImages(countedImages{files, reads}, level, func(n string, im *image.NRGBA) { out[n] = im })
	if err != nil {
		t.Fatal(err)
	}
	if len(aliases) != 3 || len(out) != 3 || reads["a/atlas.png"] != 1 || reads["b/atlas.png"] != 1 {
		t.Fatalf("source cache or full-path identity lost: aliases=%d outputs=%d reads=%v", len(aliases), len(out), reads)
	}
	want := []color.NRGBA{rgb(20, 150, 40), rgb(190, 40, 30), rgb(30, 60, 190)}
	for i, s := range level.Sprites {
		im := out[aliases[tiled.ImageKey(s)]]
		if im == nil || im.Bounds() != image.Rect(0, 0, 2, 2) || im.NRGBAAt(0, 0) != want[i] || im.NRGBAAt(1, 1) != want[i] {
			t.Fatalf("tile %d ignored atlas margin/spacing or used the other atlas", i)
		}
	}
}
func TestImportWholeGeneratedImageAliasesWithoutDuplicate(t *testing.T) {
	im := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	fill(im, im.Bounds(), rgb(20, 40, 60))
	old := textures
	textures = map[string]*image.NRGBA{"fixture": im}
	t.Cleanup(func() { textures = old })
	s := tiled.Sprite{Image: target + "/fixture.png", SourceWidth: 3, SourceHeight: 2}
	files := fstest.MapFS{s.Image: {Data: pngFixture(t, im)}}
	aliases, err := importTiledImages(
		files,
		&tiled.Map{Sprites: []tiled.Sprite{s, s}},
		func(string, *image.NRGBA) { t.Fatal("whole generated image was duplicated") },
	)
	if err != nil {
		t.Fatal(err)
	}
	if aliases[tiled.ImageKey(s)] != "fixture" || len(aliases) != 1 {
		t.Fatalf("whole image alias: %v", aliases)
	}
}
func TestImportRejectsOutOfImageSourceRectangles(t *testing.T) {
	im := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	files := fstest.MapFS{"image.png": {Data: pngFixture(t, im)}}
	for _, r := range [][4]int{{-1, 0, 1, 1}, {0, -1, 1, 1}, {0, 0, 0, 1}, {0, 0, 1, 0}, {3, 0, 2, 1}, {0, 3, 1, 2}, {int(^uint(0) >> 1), 0, 2, 1}} {
		s := tiled.Sprite{Image: "image.png", SourceX: r[0], SourceY: r[1], SourceWidth: r[2], SourceHeight: r[3]}
		if _, err := importTiledImages(
			files,
			&tiled.Map{Sprites: []tiled.Sprite{s}},
			func(string, *image.NRGBA) { t.Fatal("invalid crop emitted an image") },
		); err == nil {
			t.Fatalf("accepted source rectangle %v", r)
		}
	}
}

func TestImportIncludesFramesNotPlacedInTheMap(t *testing.T) {
	im := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	fill(im, im.Bounds(), rgb(40, 60, 80))
	a := tiled.Sprite{Image: "base.png", SourceWidth: 2, SourceHeight: 2}
	b := tiled.Sprite{Image: "frame.png", SourceWidth: 2, SourceHeight: 2}
	a.Frames = []tiled.Frame{{Sprite: a, Duration: 300}, {Sprite: b, Duration: 300}}
	files := fstest.MapFS{"base.png": {Data: pngFixture(t, im)}, "frame.png": {Data: pngFixture(t, im)}}
	out := map[string]*image.NRGBA{}
	aliases, err := importTiledImages(files, &tiled.Map{Sprites: []tiled.Sprite{a}}, func(n string, im *image.NRGBA) { out[n] = im })
	if err != nil {
		t.Fatal(err)
	}
	if len(aliases) != 2 || out[aliases[tiled.ImageKey(b)]] == nil {
		t.Fatalf("unplaced animation frame omitted from bundle: %v", aliases)
	}
}
