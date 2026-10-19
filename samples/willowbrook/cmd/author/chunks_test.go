package main

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"testing"

	"github.com/karty-game/tiled/assets"
	"github.com/karty-game/tiled/source"
)

func TestBuildChunksMatchEveryAuthoredBackgroundPixel(t *testing.T) {
	fsys := os.DirFS("../..")
	level, err := source.Load(fsys, "internal/village/village.tmj")
	if err != nil {
		t.Fatal(err)
	}
	images := map[string]image.Image{}
	chunks, err := assets.BakeStaticLayers(fsys, level, []string{"Ground", "Paths"}, assets.ChunkOptions{Size: 256, PixelScale: 4},
		func(chunk assets.StaticChunk) (string, error) {
			name := fmt.Sprintf("build/chunk-%d.png", len(images))
			images[name] = chunk.Pixels
			return name, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	read := func(name string) image.Image {
		if im := images[name]; im != nil {
			return im
		}
		f, err := fsys.Open(name)
		if err != nil {
			t.Fatal(err)
		}
		im, err := png.Decode(f)
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			t.Fatal(err, closeErr)
		}
		images[name] = im
		return im
	}
	unique := map[string]bool{}
	for _, chunk := range chunks {
		unique[chunk.Image] = true
		bounds := image.Rect(int(chunk.X*4), int(chunk.Y*4), int((chunk.X+chunk.Width)*4), int((chunk.Y+chunk.Height)*4))
		expected := image.NewNRGBA(bounds)
		for _, tile := range level.Sprites {
			if tile.Layer != chunk.Layer {
				continue
			}
			// These layers are unflipped32-world-pixel cells at their original128px resolution; the generic chunker
			// separately tests every flip flag and cross-chunk clipping.
			if tile.FlipX || tile.FlipY || tile.Diagonal || tile.Rotation != 0 || tile.Opacity != 1 ||
				tile.Width*4 != float64(tile.SourceWidth) ||
				tile.Height*4 != float64(tile.SourceHeight) {
				t.Fatal("update the independent pixel oracle for changed background geometry", tile)
			}
			r := image.Rect(int(tile.X*4), int(tile.Y*4), int((tile.X+tile.Width)*4), int((tile.Y+tile.Height)*4))
			if r.Overlaps(bounds) {
				draw.Draw(expected, r, read(tile.Image), image.Pt(tile.SourceX, tile.SourceY), draw.Over)
			}
		}
		actual := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
		draw.Draw(actual, actual.Bounds(), read(chunk.Image), image.Point{}, draw.Src)
		if !bytes.Equal(actual.Pix, expected.Pix) {
			t.Fatal("chunk image differs from independently composited source tiles", chunk.Image)
		}
	}
	const bytesPerChunk = 1024 * 1024 * 4
	if len(unique)*bytesPerChunk > 256<<20 {
		t.Fatal("unique chunk pixel budget exceeded", len(unique))
	}
	t.Logf(
		"backgrounds: %d placements, %d unique images, %d decoded RGBA bytes",
		len(chunks),
		len(unique),
		len(unique)*bytesPerChunk,
	)
	if len(chunks) <= 1 || len(chunks) > 192 {
		t.Fatal("unexpected static chunk population", len(chunks))
	}
}
