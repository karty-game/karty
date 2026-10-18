package main

import (
	"image"
	"math"
	"path/filepath"
)

func uprightProjectionScale() float64 { return math.Cos(math.Pi / 6) }

func standingSprite(id, texture string, x, y, z, width, projectedHeight float64, alpha string) {
	im := load(filepath.Join(target, texture+".png"))
	if projectedHeight == 0 {
		projectedHeight = width * float64(im.Bounds().Dy()) / float64(im.Bounds().Dx())
	}
	sprite(id, texture, x, y, z, width, projectedHeight/uprightProjectionScale(), "upright", alpha)
	s := contents[len(contents)-1]["actor"].(record)["sprite"].(record)
	// The bottom rendered texel meets the ground, tabletop or wick at z.
	s["origin_y"] = standingAlphaBase(im, alpha)
}

func standingAlphaBase(im image.Image, alpha string) float64 {
	threshold := uint32(0x8000)
	if alpha == "blend" {
		threshold = 1
	}
	for y := im.Bounds().Max.Y - 1; y >= im.Bounds().Min.Y; y-- {
		for x := im.Bounds().Min.X; x < im.Bounds().Max.X; x++ {
			_, _, _, value := im.At(x, y).RGBA()
			if value >= threshold {
				return float64(y+1-im.Bounds().Min.Y) / float64(im.Bounds().Dy())
			}
		}
	}
	panic("standing sprite has no visible alpha")
}
