// world-camera-materials generates original, seamless sample albedo artwork.
// Run from the CLI repository root through mise run generate-world-camera-materials.
package main

import (
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

const (
	textureSize        = 64
	marbleVeinSkew     = 3
	marbleVeinPeriod   = 23
	mosaicSize         = 8
	checkerPeriod      = 2
	cofferTrim         = 4
	cofferInset        = 7
	panelSize          = 32
	brickHeight        = 16
	groutWidth         = 2
	opaqueAlpha        = 255
	rippleSecondWeight = 0.5

	grainColumnWeight = 13
	grainRowWeight    = 7
	grainLevels       = 5
	floorShadeStep    = 12
	wallShadeStep     = 8
	wallShadeLevels   = 3

	floorGroutRed, floorGroutGreen, floorGroutBlue                   = 39, 52, 62
	floorBaseRed, floorBaseGreen, floorBaseBlue                      = 92, 121, 137
	wallGroutRed, wallGroutGreen, wallGroutBlue                      = 62, 52, 46
	wallHighlightRed, wallHighlightGreen, wallHighlightBlue          = 197, 139, 103
	wallBaseRed, wallBaseGreen, wallBaseBlue                         = 153, 94, 67
	ceilingGroutRed, ceilingGroutGreen, ceilingGroutBlue             = 91, 104, 111
	ceilingHighlightRed, ceilingHighlightGreen, ceilingHighlightBlue = 228, 231, 222
	ceilingBaseRed, ceilingBaseGreen, ceilingBaseBlue                = 182, 195, 191
)

var ErrMaterial = errors.New("unknown material")

func main() {
	only := flag.String("only", "", "regenerate one named sample material")

	flag.Parse()

	if err := generate(*only); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(only string) error {
	generated := false

	for _, name := range []string{"floor", "wall", "ceiling", "z-court-marble", "z-court-mosaic", "z-court-paving", "z-court-plaster", "z-court-coffer", "z-court-sky", "z-court-water"} {
		if only != "" && name != only {
			continue
		}

		generated = true

		texture := image.NewNRGBA(image.Rect(0, 0, textureSize, textureSize))
		for y := range textureSize {
			for x := range textureSize {
				texture.SetNRGBA(x, y, pixel(name, x, y))
			}
		}

		path := filepath.Join("samples", "world-camera", "levels", "showcase", name+".png")

		output, err := os.Create(path)
		if err != nil {
			return err
		}

		encodeErr := png.Encode(output, texture)
		closeErr := output.Close()

		if encodeErr != nil {
			return encodeErr
		}

		if closeErr != nil {
			return closeErr
		}
	}

	if !generated {
		return fmt.Errorf("%w %q", ErrMaterial, only)
	}

	return nil
}

func pixel(name string, column, row int) color.NRGBA {
	grain := uint8((column*grainColumnWeight + row*grainRowWeight) % grainLevels)

	switch name {
	case "z-court-marble", "z-court-mosaic", "z-court-paving", "z-court-plaster", "z-court-coffer", "z-court-sky", "z-court-water":
		return courtPixel(name, column, row, grain)
	default:
		return galleryPixel(name, column, row, grain)
	}
}

func courtPixel(name string, column, row int, grain uint8) color.NRGBA {
	switch name {
	case "z-court-marble":
		return courtMarblePixel(column, row, grain)
	case "z-court-mosaic":
		return courtMosaicPixel(column, row, grain)
	case "z-court-paving":
		return courtPavingPixel(column, row, grain)
	case "z-court-plaster":
		return courtPlasterPixel(column, row)
	case "z-court-coffer":
		return courtCofferPixel(column, row, grain)
	case "z-court-water":
		return courtWaterPixel(column, row)
	default:
		return grainTint(117, 181, 212, grain)
	}
}

func courtWaterPixel(column, row int) color.NRGBA {
	// Integer harmonics repeat seamlessly across the texture. The low contrast
	// keeps generated height/normal maps shallow while suggesting still water.
	x := 2 * math.Pi * float64(column) / textureSize
	y := 2 * math.Pi * float64(row) / textureSize
	ripple := math.Sin(x+2*y) + rippleSecondWeight*math.Sin(3*x-y)
	caustic := math.Pow(math.Max(0, math.Sin(2*x+y)), 8)

	shade := int(math.Round(7*ripple + 12*caustic))
	if column%brickHeight == 0 || row%brickHeight == 0 {
		shade -= 4 // A faint submerged tile seam, rather than a dark grout line.
	}

	return color.NRGBA{
		R: uint8(45 + shade), G: uint8(141 + shade), B: uint8(146 + shade), A: opaqueAlpha,
	}
}

func courtMarblePixel(column, row int, grain uint8) color.NRGBA {
	vein := (column + row/marbleVeinSkew + row*row/textureSize) % marbleVeinPeriod
	if vein < groutWidth {
		return grainTint(133, 154, 145, grain)
	}

	return grainTint(217, 222, 210, grain)
}

func courtMosaicPixel(column, row int, grain uint8) color.NRGBA {
	if column%mosaicSize == 0 || row%mosaicSize == 0 {
		return grainTint(59, 69, 65, 0)
	}

	if (column/mosaicSize+row/mosaicSize)%checkerPeriod == 0 {
		return grainTint(38, 112, 106, grain)
	}

	return grainTint(219, 219, 200, grain)
}

func courtPavingPixel(column, row int, grain uint8) color.NRGBA {
	if column%panelSize < groutWidth || row%panelSize < groutWidth {
		return grainTint(93, 102, 97, 0)
	}

	return grainTint(190, 203, 194, grain)
}

// Pale Roman limestone courses with bevels, pores and shallow mineral mottling.
// All details are authored pixels; Materialize derives the relief from this tile.
func courtPlasterPixel(column, row int) color.NRGBA {
	const (
		blockWidth, courseHeight        = 32, 16
		staggerPeriod                   = 2
		groutRed, groutGreen, groutBlue = 202, 200, 191
		hashShift                       = 13
		shadeLevels, shadeMidpoint      = 13, 6
	)

	course := row / courseHeight
	blockColumn := (column + (course%staggerPeriod)*blockWidth/staggerPeriod) % blockWidth

	courseRow := row % courseHeight
	if blockColumn == 0 || courseRow == 0 {
		return color.NRGBA{R: groutRed, G: groutGreen, B: groutBlue, A: opaqueAlpha}
	}

	hash := uint32(column*1973 + row*9277 + 89173)
	hash ^= hash >> hashShift
	hash *= 1274126177
	shade := int(hash%shadeLevels) - shadeMidpoint
	waveX, waveY := 2*math.Pi*float64(column)/textureSize, 2*math.Pi*float64(row)/textureSize

	shade += int(math.Round(3*math.Sin(waveX) + 2*math.Cos(2*waveY) + 2*math.Sin(waveX+waveY)))
	if blockColumn == 1 || courseRow == 1 {
		shade += 7
	}

	if blockColumn == blockWidth-1 || courseRow == courseHeight-1 {
		shade -= 10
	}

	if blockColumn > 2 && courseRow > 2 && hash%53 == 0 {
		shade -= 23
	}

	return color.NRGBA{
		R: uint8(min(max(238+shade, 0), 255)),
		G: uint8(min(max(236+shade, 0), 255)),
		B: uint8(min(max(229+shade, 0), 255)), A: opaqueAlpha,
	}
}

func courtCofferPixel(column, row int, grain uint8) color.NRGBA {
	insetX, insetY := column%panelSize, row%panelSize
	if insetX < cofferTrim || insetY < cofferTrim || insetX > panelSize-cofferTrim-1 || insetY > panelSize-cofferTrim-1 {
		return grainTint(177, 189, 178, grain)
	}

	if insetX < cofferInset || insetY < cofferInset || insetX > panelSize-cofferInset-1 || insetY > panelSize-cofferInset-1 {
		return grainTint(74, 105, 94, 0)
	}

	return grainTint(209, 221, 207, grain)
}

func galleryPixel(name string, column, row int, grain uint8) color.NRGBA {
	switch name {
	case "floor":
		if column%panelSize < groutWidth || row%panelSize < groutWidth {
			return color.NRGBA{R: floorGroutRed, G: floorGroutGreen, B: floorGroutBlue, A: opaqueAlpha}
		}

		shade := uint8(((column/panelSize)+(row/panelSize))%2) * floorShadeStep

		return color.NRGBA{
			R: floorBaseRed + shade + grain,
			G: floorBaseGreen + shade + grain,
			B: floorBaseBlue + shade + grain,
			A: opaqueAlpha,
		}
	case "wall":
		brickColumn := (column + (row/brickHeight)%2*brickHeight) % panelSize
		if brickColumn < groutWidth || row%brickHeight < groutWidth {
			return color.NRGBA{R: wallGroutRed, G: wallGroutGreen, B: wallGroutBlue, A: opaqueAlpha}
		}

		if brickColumn == groutWidth || row%brickHeight == groutWidth {
			return color.NRGBA{R: wallHighlightRed, G: wallHighlightGreen, B: wallHighlightBlue, A: opaqueAlpha}
		}

		shade := uint8((column/panelSize+row/brickHeight)%wallShadeLevels) * wallShadeStep

		return color.NRGBA{
			R: wallBaseRed + shade + grain,
			G: wallBaseGreen + shade + grain,
			B: wallBaseBlue + shade + grain,
			A: opaqueAlpha,
		}
	default: // Ceiling panels.
		if column%panelSize < groutWidth || row%panelSize < groutWidth {
			return color.NRGBA{R: ceilingGroutRed, G: ceilingGroutGreen, B: ceilingGroutBlue, A: opaqueAlpha}
		}

		if column%panelSize == groutWidth || row%panelSize == groutWidth {
			return color.NRGBA{
				R: ceilingHighlightRed, G: ceilingHighlightGreen, B: ceilingHighlightBlue, A: opaqueAlpha,
			}
		}

		return color.NRGBA{R: ceilingBaseRed + grain, G: ceilingBaseGreen + grain, B: ceilingBaseBlue + grain, A: opaqueAlpha}
	}
}

func grainTint(red, green, blue, grain uint8) color.NRGBA {
	return color.NRGBA{R: red + grain, G: green + grain, B: blue + grain, A: opaqueAlpha}
}
