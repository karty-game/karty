package tiled_test

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"testing"

	"github.com/karty-game/tiled"
	"github.com/karty-game/tiled/source"
)

// This golden is produced by the official Tiled 1.11.2 rasterizer, not by this
// package. It covers all eight flags, unequal source/target dimensions, scaled
// tile offsets and tile objects rotated about both supported alignment points.
func TestTransformsMatchOfficialTiledRaster(t *testing.T) {
	const directory = "testdata/orthogonal-flips"
	level, err := source.Load(os.DirFS(directory), "map.tmj")
	if err != nil {
		t.Fatal(err)
	}
	if len(level.Sprites) != 32 {
		t.Fatalf("oracle lost cases: got %d sprites, want 32", len(level.Sprites))
	}
	want := readOraclePNG(t, directory+"/tiled-1.11.2.png")
	atlas := readOraclePNG(t, directory+"/atlas.png")
	got := image.NewNRGBA(want.Bounds())
	nearestTies := make([]bool, got.Rect.Dx()*got.Rect.Dy())
	for _, sprite := range level.Sprites {
		cx, cy := tiled.Center(sprite)
		sx, sy, angle := tiled.Transform(sprite)
		cos, sin := math.Cos(angle), math.Sin(angle)
		halfWidth, halfHeight := float64(sprite.SourceWidth)/2, float64(sprite.SourceHeight)/2
		extentX := math.Abs(halfWidth*sx*cos) + math.Abs(halfHeight*sy*sin)
		extentY := math.Abs(halfWidth*sx*sin) + math.Abs(halfHeight*sy*cos)
		for y := max(int(math.Floor(cy-extentY)), got.Rect.Min.Y); y < min(int(math.Ceil(cy+extentY)), got.Rect.Max.Y); y++ {
			for x := max(int(math.Floor(cx-extentX)), got.Rect.Min.X); x < min(int(math.Ceil(cx+extentX)), got.Rect.Max.X); x++ {
				// Inverse-map destination pixel centers using the exported sprite
				// geometry. No Tiled flip interpretation is repeated here.
				dx, dy := float64(x)+.5-cx, float64(y)+.5-cy
				u := (dx*cos+dy*sin)/sx + halfWidth
				v := (-dx*sin+dy*cos)/sy + halfHeight
				if u < 0 || v < 0 || u >= float64(sprite.SourceWidth) || v >= float64(sprite.SourceHeight) {
					continue
				}
				// Qt's fragment and fallback paths resolve exact texel-boundary
				// nearest-neighbour ties differently. Those RGB comparisons are
				// ambiguous; coverage is still checked at every destination pixel.
				nearestTies[(y-got.Rect.Min.Y)*got.Rect.Dx()+x-got.Rect.Min.X] = math.Abs(u-math.Round(u)) < 1e-7 ||
					math.Abs(v-math.Round(v)) < 1e-7
				got.Set(x, y, atlas.At(sprite.SourceX+int(u), sprite.SourceY+int(v)))
			}
		}
	}
	var different int
	first := image.Pt(-1, -1)
	for y := want.Bounds().Min.Y; y < want.Bounds().Max.Y; y++ {
		for x := want.Bounds().Min.X; x < want.Bounds().Max.X; x++ {
			actual := color.NRGBAModel.Convert(got.At(x, y)).(color.NRGBA)
			expected := color.NRGBAModel.Convert(want.At(x, y)).(color.NRGBA)
			if actual != expected && !(actual.A == expected.A && nearestTies[(y-got.Rect.Min.Y)*got.Rect.Dx()+x-got.Rect.Min.X]) {
				different++
				if first.X == -1 {
					first = image.Pt(x, y)
				}
			}
		}
	}
	if different != 0 {
		t.Fatalf(
			"sprite geometry differs from official Tiled at %d pixels; first %v: got %v, want %v",
			different,
			first,
			got.At(first.X, first.Y),
			want.At(first.X, first.Y),
		)
	}
}

func readOraclePNG(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}
