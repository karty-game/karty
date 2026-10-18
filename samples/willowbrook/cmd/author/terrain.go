package main

import (
	"crypto/sha256"
	"fmt"
	"image"
	"math"
	"os"
)

var terrainSourceHash string
var terrainMaskCounts [16]int
var terrainTiles [16]*image.NRGBA

// The atlas has fifteen nonempty dirt cases plus its grass background at 00.
// Source dimensions need not divide by four: quantize each cell independently
// and exclude two boundary pixels so adjacent atlas cells cannot bleed in.
func terrainCell(bounds image.Rectangle, mask int) image.Rectangle {
	x0 := int(math.Round(float64(mask%4*bounds.Dx()) / 4))
	y0 := int(math.Round(float64(mask/4*bounds.Dy()) / 4))
	x1 := int(math.Round(float64((mask%4+1)*bounds.Dx()) / 4))
	y1 := int(math.Round(float64((mask/4+1)*bounds.Dy()) / 4))
	return image.Rect(x0+2, y0+2, x1-2, y1-2)
}

func prepareTerrain() {
	const path = "../../scripts/willowbrook/assets/dual-grid-grass-path.png"
	data, err := os.ReadFile(path)
	must(err)
	terrainSourceHash = fmt.Sprintf("%x", sha256.Sum256(data))
	atlas := load(path)
	for mask := range 16 {
		cell := crop(atlas, terrainCell(atlas.Bounds(), mask))
		name := fmt.Sprintf("dual-%02d", mask)
		terrainTiles[mask] = resize(cell, 128, 128)
		save(name, terrainTiles[mask])

	}
}
