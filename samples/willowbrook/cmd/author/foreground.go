package main

import (
	"crypto/sha256"
	"fmt"
	"image"
	"math"
	"path"
	"strings"

	"github.com/karty-game/tiled"
)

// Foreground shapes use standard Tiled tile object groups. They copy original
// pixels into an equal-size canvas, so ordinary sorted image objects can cover
// the player at a front rail or cheek without redrawing or stretching the art.
func prepareForegroundMasks(level *tiled.Map) error {
	for _, set := range level.Tilesets {
		for _, tile := range set.Tiles {
			if tile.ObjectGroup == nil {
				continue
			}
			shapes := []tiled.Object{}
			for _, o := range tile.ObjectGroup.Objects {
				class := o.Class
				if class == "" {
					class = o.Type
				}
				if class == "foreground" {
					shapes = append(shapes, o)
				}
			}
			if len(shapes) == 0 {
				continue
			}
			if tile.Image == "" {
				return fmt.Errorf("foreground tile %d requires an image-collection image", tile.ID)
			}
			out, e := foregroundPixels(load(tile.Image), shapes)
			if e != nil {
				return fmt.Errorf("foreground image %s: %w", tile.Image, e)
			}
			name := tile.Properties.String("foregroundTexture", "")
			if name == "" {
				if path.Dir(tile.Image) == target {
					name = strings.TrimSuffix(path.Base(tile.Image), path.Ext(tile.Image)) + "-front"
				} else {
					name = fmt.Sprintf("tiled-%x", sha256.Sum256([]byte(tile.Image)))[:18] + "-front"
				}
			}
			if name == "" || path.Base(name) != name || strings.ContainsAny(name, "\\:") {
				return fmt.Errorf("invalid foreground texture name %q", name)
			}
			save(name, out)
		}
	}
	return nil
}
func foregroundPixels(im image.Image, shapes []tiled.Object) (*image.NRGBA, error) {
	b := im.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	for _, o := range shapes {
		if !finite(o.X) || !finite(o.Y) || o.Rotation != 0 || o.Point || o.Ellipse || len(o.Polyline) > 0 {
			return nil, fmt.Errorf("unsupported foreground shape %q", o.Name)
		}
		if len(o.Polygon) > 0 {
			if len(o.Polygon) < 3 {
				return nil, fmt.Errorf("foreground polygon needs three points")
			}
			for _, p := range o.Polygon {
				x, y := o.X+p.X, o.Y+p.Y
				if !finite(x) || !finite(y) || x < 0 || y < 0 || x > w || y > h {
					return nil, fmt.Errorf("foreground polygon exceeds image")
				}
			}
		} else if !finite(o.Width) || !finite(o.Height) || o.X < 0 || o.Y < 0 || o.Width <= 0 || o.Height <= 0 || o.X+o.Width > w || o.Y+o.Height > h {
			return nil, fmt.Errorf("foreground rectangle exceeds image")
		}
	}
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := range b.Dy() {
		for x := range b.Dx() {
			px, py := float64(x)+.5, float64(y)+.5
			inside := false
			for _, o := range shapes {
				if len(o.Polygon) == 0 {
					inside = px >= o.X && px < o.X+o.Width && py >= o.Y && py < o.Y+o.Height
				} else {
					hit := false
					for i, a := range o.Polygon {
						c := o.Polygon[(i+1)%len(o.Polygon)]
						ax, ay, cx, cy := a.X+o.X, a.Y+o.Y, c.X+o.X, c.Y+o.Y
						if (ay > py) != (cy > py) && px < (cx-ax)*(py-ay)/(cy-ay)+ax {
							hit = !hit
						}
					}
					inside = hit
				}
				if inside {
					break
				}
			}
			if inside {
				out.Set(x, y, im.At(b.Min.X+x, b.Min.Y+y))
			}
		}
	}
	return out, nil
}
