package main

import (
	"fmt"
	"image"
	"image/draw"
	"math"
	"sort"
)

const characterDirections = 8

// The original study sheet remains an extraction regression fixture. Active
// character frames are rendered as complete bodies from the retained model.
func characterFrames(sheet image.Image) ([64]*image.NRGBA, error) {
	var frames [64]*image.NRGBA
	rows, err := characterCuts(sheet, sheet.Bounds(), false)
	if err != nil {
		return frames, err
	}
	var regions [64]image.Rectangle
	var anchors [64]float64
	canvas := max((sheet.Bounds().Dx()+7)/8, (sheet.Bounds().Dy()+7)/8)
	for row := range characterDirections {
		band := image.Rect(sheet.Bounds().Min.X, rows[row], sheet.Bounds().Max.X, rows[row+1])
		columns, err := characterCuts(sheet, band, true)
		if err != nil {
			return frames, fmt.Errorf("character row %d: %w", row, err)
		}
		for pose := range characterDirections {
			region := opaqueCharacterBounds(sheet, image.Rect(columns[pose], band.Min.Y, columns[pose+1], band.Max.Y))
			if region.Empty() {
				return frames, fmt.Errorf("empty character row %d pose %d", row, pose)
			}
			regions[row*8+pose] = region
			anchor := characterBodyAnchor(sheet, region)
			anchors[row*8+pose] = anchor
			canvas = max(canvas, region.Dx()+4, int(math.Ceil(float64(region.Dy()+2)/.98)))
			canvas = max(canvas, int(math.Ceil(2*(anchor-float64(region.Min.X))+4)), int(math.Ceil(2*(float64(region.Max.X)-anchor)+4)))
		}
	}
	sole := int(math.Round(float64(canvas) * .98))
	for i, region := range regions {
		frame := image.NewNRGBA(image.Rect(0, 0, canvas, canvas))
		x := int(math.Round(float64(canvas)/2 - (anchors[i] - float64(region.Min.X))))
		to := image.Rect(x, sole-region.Dy(), x+region.Dx(), sole)
		// Only translation: preserve the source pixels and the relative pose.
		draw.Draw(frame, to, sheet, region.Min, draw.Src)
		frames[i] = frame
	}
	return frames, nil
}

// Follow the body axis instead of the full outline: a sword, shield or extended
// boot must not move the entire pilgrim. The midpoint balances the hood and
// torso, retaining source lean without following either silhouette extreme.
func characterBodyAnchor(sheet image.Image, region image.Rectangle) float64 {
	return (characterBandCenter(sheet, region, 0, .18) + characterBandCenter(sheet, region, .3, .6)) / 2
}

func characterBandCenter(sheet image.Image, region image.Rectangle, from, to float64) float64 {
	var centers []float64
	for y := region.Min.Y + int(float64(region.Dy())*from); y < region.Min.Y+int(float64(region.Dy())*to); y++ {
		left, right := region.Max.X, region.Min.X
		for x := region.Min.X; x < region.Max.X; x++ {
			_, _, _, alpha := sheet.At(x, y).RGBA()
			if alpha >= 0x8000 {
				left, right = min(left, x), max(right, x+1)
			}
		}
		if right > left {
			centers = append(centers, float64(left+right)/2)
		}
	}
	if len(centers) == 0 {
		panic("character pose has no head or torso alpha")
	}
	sort.Float64s(centers)
	return (centers[(len(centers)-1)/2] + centers[len(centers)/2]) / 2
}

// Find an empty cut near each proportional division. The search stays within
// one third of a cell, so a malformed sheet fails instead of combining poses.
func characterCuts(sheet image.Image, region image.Rectangle, vertical bool) ([9]int, error) {
	var cuts [9]int
	start, end := region.Min.Y, region.Max.Y
	if vertical {
		start, end = region.Min.X, region.Max.X
	}
	cuts[0], cuts[8] = start, end
	for i := 1; i < 8; i++ {
		ideal := start + (end-start)*i/8
		found := false
		for distance := 0; distance <= (end-start)/24 && !found; distance++ {
			for _, at := range [...]int{ideal - distance, ideal + distance} {
				if at <= cuts[i-1] || at >= end {
					continue
				}
				empty := true
				if vertical {
					for y := region.Min.Y; y < region.Max.Y; y++ {
						_, _, _, alpha := sheet.At(at, y).RGBA()
						if alpha >= 0x8000 {
							empty = false
							break
						}
					}
				} else {
					for x := region.Min.X; x < region.Max.X; x++ {
						_, _, _, alpha := sheet.At(x, at).RGBA()
						if alpha >= 0x8000 {
							empty = false
							break
						}
					}
				}
				if empty {
					cuts[i], found = at, true
					break
				}
			}
		}
		if !found {
			return cuts, fmt.Errorf("character atlas has no transparent gutter near division %d", i)
		}
	}
	return cuts, nil
}

func opaqueCharacterBounds(sheet image.Image, region image.Rectangle) image.Rectangle {
	lo, hi := region.Max, region.Min
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			_, _, _, alpha := sheet.At(x, y).RGBA()
			if alpha >= 0x8000 {
				lo.X, lo.Y = min(lo.X, x), min(lo.Y, y)
				hi.X, hi.Y = max(hi.X, x+1), max(hi.Y, y+1)
			}
		}
	}
	if hi.X < lo.X || hi.Y < lo.Y {
		return image.Rectangle{}
	}
	return image.Rectangle{Min: lo, Max: hi}
}
