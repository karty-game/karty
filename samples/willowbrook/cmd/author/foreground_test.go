package main

import (
	"image"
	"image/color"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/karty-game/tiled"
	"github.com/karty-game/tiled/source"
)

func TestForegroundMetadataCopiesPixelsOnIdenticalCanvas(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 5, 4))
	for y := range 4 {
		for x := range 5 {
			source.SetNRGBA(x, y, color.NRGBA{uint8(x + 30), uint8(y + 60), 90, 255})
		}
	}
	shapes := []tiled.Object{
		{Name: "rail", X: 1, Y: 0, Width: 2, Height: 1},
		{Name: "post", Polygon: []tiled.Point{{X: 3, Y: 2}, {X: 5, Y: 2}, {X: 5, Y: 4}, {X: 3, Y: 4}}},
	}
	out, err := foregroundPixels(source, shapes)
	if err != nil {
		t.Fatal(err)
	}
	if out.Bounds() != source.Bounds() {
		t.Fatal("foreground canvas changed")
	}
	for y := range 4 {
		for x := range 5 {
			want := color.NRGBA{}
			if (y == 0 && x >= 1 && x < 3) || (y >= 2 && x >= 3) {
				want = source.NRGBAAt(x, y)
			}
			if out.NRGBAAt(x, y) != want {
				t.Fatalf("mask changed source pixels at%d,%d", x, y)
			}
		}
	}
}
func TestForegroundRejectsShapesOutsideSourceCanvas(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for _, shape := range []tiled.Object{{X: -1, Width: 2, Height: 2}, {Width: 5, Height: 2}, {Width: 2, Height: 2, Rotation: 10}, {Width: 2, Height: 2, Ellipse: true}, {Polygon: []tiled.Point{{X: 0, Y: 0}, {X: 4, Y: 0}, {X: 5, Y: 4}}}, {Polygon: []tiled.Point{{X: 0, Y: 0}, {X: 4, Y: 0}, {X: math.NaN(), Y: 4}}}} {
		if _, err := foregroundPixels(source, []tiled.Object{shape}); err == nil {
			t.Fatalf("accepted unsupported foreground shape:%+v", shape)
		}
	}
}
func TestTiledLandmarksUseWholeImagesAndMatchingForegroundPoses(t *testing.T) {
	level, err := source.Load(os.DirFS("../.."), "internal/village/village.tmj")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"staircase-intact", "bridge-intact"} {
		var base, front []tiled.Sprite
		for _, s := range level.Sprites {
			if s.Image == "levels/village/"+name+".png" {
				base = append(base, s)
			}
			if s.Image == "levels/village/"+name+"-front.png" {
				front = append(front, s)
			}
		}
		if len(base) != 1 || len(front) != 1 {
			t.Fatalf("%s is fragmented or has no front mask", name)
		}
		a, b := base[0], front[0]
		if a.X != b.X || a.Y != b.Y || a.Width != b.Width || a.Height != b.Height || a.SourceWidth != b.SourceWidth ||
			a.SourceHeight != b.SourceHeight ||
			a.Layer >= b.Layer {
			t.Fatalf("%s foreground lost its image pose", name)
		}
		if math.Abs(a.Width/a.Height-float64(a.SourceWidth)/float64(a.SourceHeight)) > .00001 {
			t.Fatalf("%s image was stretched", name)
		}
	}
	for _, l := range level.Layers {
		for _, o := range l.Objects {
			if strings.HasPrefix(o.Name, "stair-tread-") || strings.HasPrefix(o.Name, "bridge-post-") ||
				strings.HasPrefix(o.Name, "bridge-rail-") ||
				o.Name == "bridge-front-beam" {
				t.Fatalf("obsolete fragmented landmark object remains:%s", o.Name)
			}
		}
		if l.Name == "Bridge deck" {
			for _, gid := range l.Data {
				if gid != 0 {
					t.Fatal("old bridge plank tiles remain under intact image")
				}
			}
		}
	}
}
