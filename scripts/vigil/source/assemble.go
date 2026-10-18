//go:build ignore

// Assemble the Blender renders without modifying any pose or registration.
package main

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) != 3 {
		panic("usage: go run assemble.go FRAMES_DIRECTORY ART_DIRECTORY")
	}
	source, target := os.Args[1], os.Args[2]
	walk := image.NewNRGBA(image.Rect(0, 0, 24*192, 8*192))
	idle := image.NewNRGBA(image.Rect(0, 0, 8*192, 192))
	for row := range 8 {
		for phase := range 24 {
			frame := read(filepath.Join(source, fmt.Sprintf("walk-%d-%02d.png", row, phase)))
			draw.Draw(walk, image.Rect(phase*192, row*192, (phase+1)*192, (row+1)*192), frame, image.Point{}, draw.Src)
		}
		frame := read(filepath.Join(source, fmt.Sprintf("idle-%d.png", row)))
		draw.Draw(idle, image.Rect(row*192, 0, (row+1)*192, 192), frame, image.Point{}, draw.Src)
	}
	write(filepath.Join(target, "pilgrim-walk.png"), walk)
	write(filepath.Join(target, "pilgrim-idle.png"), idle)
	metadata, err := os.ReadFile(filepath.Join(source, "pilgrim-render.json"))
	must(err)
	must(os.WriteFile(filepath.Join(target, "pilgrim-render.json"), metadata, 0644))
}
func read(path string) image.Image {
	file, err := os.Open(path)
	must(err)
	defer file.Close()
	img, err := png.Decode(file)
	must(err)
	if img.Bounds() != image.Rect(0, 0, 192, 192) {
		panic("render is not 192x192: " + path)
	}
	return img
}
func write(path string, img image.Image) {
	file, err := os.Create(path)
	must(err)
	must(png.Encode(file, img))
	must(file.Close())
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
