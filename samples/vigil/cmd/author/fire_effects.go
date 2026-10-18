package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

const fireChannelMax = 255

// Spell artwork is independent of the grounded candle flipbook. Direction is
// baked into the image so every airborne card retains true upright depth.
func prepareFireEffectImages() {
	save("fire-core", fireEffectImage(fireCorePixel))
	save("fire-flash", fireEffectImage(fireFlashPixel))
	save("fire-ember", fireEffectImage(fireEmberPixel))

	for direction := range 8 {
		const quarterTurnSteps = 4
		angle := float64(direction) * math.Pi / quarterTurnSteps
		tailX, tailY := math.Sin(angle), -math.Cos(angle)

		save(fmt.Sprintf("fire-trail-%d", direction), fireEffectImage(func(x, y float64) color.NRGBA {
			return fireTrailPixel(x*tailX+y*tailY, -x*tailY+y*tailX)
		}))
	}

	for phase := range 2 {
		save(fmt.Sprintf("fire-smoke-%d", phase), fireEffectImage(func(x, y float64) color.NRGBA {
			return fireSmokePixel(x, y, phase)
		}))
	}
}

func fireEffectImage(pixel func(float64, float64) color.NRGBA) *image.NRGBA {
	const (
		size        = 96
		pixelCenter = .5
	)

	texture := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			texture.SetNRGBA(x, y, pixel((float64(x)+pixelCenter)*2/size-1, (float64(y)+pixelCenter)*2/size-1))
		}
	}

	return texture
}

func fireCorePixel(x, y float64) color.NRGBA {
	angle := math.Atan2(y, x)
	radius := math.Hypot(x, y) / (.68 + .045*math.Sin(angle*5) + .025*math.Cos(angle*9))

	if radius >= 1 {
		return color.NRGBA{}
	}

	heat := math.Pow(1-radius, .45)

	return color.NRGBA{R: fireChannelMax, G: uint8(100 + 155*heat), B: uint8(18 + 232*heat*heat), A: uint8(255 * min(1, (1-radius)*5))}
}

//nolint:mnd // Numeric control points define the authored taper, turbulence and heat gradient.
func fireTrailPixel(along, across float64) color.NRGBA {
	if along < -.24 || along > .96 {
		return color.NRGBA{}
	}

	progress := (along + .24) / 1.2
	center := .065 * math.Sin(progress*11) * progress
	width := .29 * math.Pow(1-progress, .7) * (.82 + .18*math.Sin(progress*19))
	radius := math.Abs(across-center) / max(.001, width)

	if radius >= 1 {
		return color.NRGBA{}
	}

	end := min(1, (along+.24)*18)
	heat := (1 - radius) * (1 - .65*progress)
	alpha := min(1, (1-radius)*4) * math.Pow(1-progress, .55) * end

	return color.NRGBA{R: fireChannelMax, G: uint8(68 + 177*heat), B: uint8(8 + 130*heat*heat), A: uint8(225 * alpha)}
}

func fireFlashPixel(x, y float64) color.NRGBA {
	angle := math.Atan2(y, x)
	radius := math.Hypot(x, y) / (.70 + .10*math.Sin(angle*7) + .045*math.Sin(angle*13))

	if radius >= 1 {
		return color.NRGBA{}
	}

	heat := math.Pow(1-radius, .6)

	return color.NRGBA{R: fireChannelMax, G: uint8(83 + 172*heat), B: uint8(13 + 231*heat), A: uint8(245 * min(1, (1-radius)*4))}
}

func fireEmberPixel(x, y float64) color.NRGBA {
	radius := math.Hypot(x/.45, y/.72)
	if radius >= 1 {
		return color.NRGBA{}
	}

	heat := 1 - radius

	return color.NRGBA{R: fireChannelMax, G: uint8(90 + 165*heat), B: uint8(18 + 200*heat*heat), A: uint8(255 * min(1, heat*4))}
}

//nolint:mnd // Numeric control points define the painted smoke silhouette, grain and two opacity phases.
func fireSmokePixel(horizontal, vertical float64, phase int) color.NRGBA {
	angle := math.Atan2(vertical, horizontal)
	radius := math.Hypot(horizontal/.83, vertical/.9) / (1 + .10*math.Sin(angle*5) + .07*math.Cos(angle*9))

	if radius >= 1 {
		return color.NRGBA{}
	}

	grain := .70 + .18*math.Sin(horizontal*12+math.Sin(vertical*9)) + .12*math.Cos(vertical*15-horizontal*7)
	alpha := math.Pow(1-radius, .8) * grain * float64(66-phase*43)
	shade := uint8(58 + 24*(1-radius))

	return color.NRGBA{R: shade + 8, G: shade + 3, B: shade, A: uint8(alpha)}
}
