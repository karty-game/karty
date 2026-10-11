//go:build ignore

// Generate the Hangar's authored flipbook frames and moving pump head.
// The detailed fan rotor is separate checked-in artwork; see its .prompt.txt.
// Run from the repository root: mise exec -- go run scripts/hangar-animation-assets.go
package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
)

const directory = "samples/hangar/levels/hangar"

func save(name string, img image.Image) {
	f, err := os.Create(filepath.Join(directory, name+".png"))
	if err != nil {
		panic(err)
	}
	if err = png.Encode(f, img); err != nil {
		panic(err)
	}
	if err = f.Close(); err != nil {
		panic(err)
	}
}
func rect(img *image.NRGBA, x0, y0, x1, y1 int, c color.NRGBA) {
	draw.Draw(img, image.Rect(x0, y0, x1, y1), image.NewUniform(c), image.Point{}, draw.Src)
}
func main() {
	f, err := os.Open(filepath.Join(directory, "screen.png"))
	if err != nil {
		panic(err)
	}
	screen, err := png.Decode(f)
	if err != nil {
		panic(err)
	}
	f.Close()
	b := screen.Bounds()
	for frame := 1; frame <= 3; frame++ {
		img := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		for y := 0; y < b.Dy(); y++ {
			for x := 0; x < b.Dx(); x++ {
				shift := 0
				if y%(17+frame*4) < 3+frame {
					shift = frame * 9
				}
				sx := (x + shift) % b.Dx()
				c := color.NRGBAModel.Convert(screen.At(b.Min.X+sx, b.Min.Y+y)).(color.NRGBA)
				if y%(23+frame*3) < 2 {
					c.R = byte(min(255, int(c.R)+110))
					c.B = byte(min(255, int(c.B)+90))
				}
				if frame == 2 && y%7 < 2 {
					c.R /= 3
					c.G /= 3
					c.B /= 3
				}
				img.SetNRGBA(x, y, c)
			}
		}
		save([]string{"", "screen-glitch-1", "screen-glitch-2", "screen-glitch-3"}[frame], img)
	}
	piston := image.NewNRGBA(image.Rect(0, 0, 64, 128))
	rect(piston, 26, 24, 38, 124, color.NRGBA{145, 157, 132, 255})
	rect(piston, 9, 8, 55, 36, color.NRGBA{87, 104, 70, 255})
	rect(piston, 13, 12, 51, 18, color.NRGBA{161, 162, 91, 255})
	rect(piston, 13, 27, 51, 32, color.NRGBA{42, 54, 34, 255})
	save("piston", piston)
}
