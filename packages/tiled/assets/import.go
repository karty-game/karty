// Package assets imports the PNG image rectangles described by a Tiled map.
// It has no renderer dependency; callers choose how imported pixels are stored.
package assets

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"io/fs"
	"strings"

	"github.com/karty-game/tiled"
)

const (
	MaxEncodedImageBytes = 32 << 20
	MaxEncodedBytes      = 128 << 20
	MaxImageDimension    = 8192
	MaxImagePixels       = 16 << 20
	// Sources are charged eight bytes per pixel, including eight-bit PNGs, so
	// this budget remains safe for sixteen-bit RGBA PNG decode allocations.
	MaxDecodedSourceBytes = 256 << 20
	MaxOutputBytes        = 256 << 20
)

// ErrLimit identifies an encoded-data or pixel-allocation limit rejection.
var ErrLimit = errors.New("Tiled asset resource limit exceeded")

// Input is one distinct image rectangle, including an unplaced animation frame.
// Rectangle and SourceBounds use decoded source-image coordinates. Pixels is a
// separate zero-origin canvas containing exactly Rectangle; flips remain map
// transform metadata. Key is tiled.ImageKey for this source rectangle.
// The callback owns Pixels and may retain it after Import returns.
type Input struct {
	Key, Source             string
	Rectangle, SourceBounds image.Rectangle
	Pixels                  *image.NRGBA
}

type limits struct {
	encodedImage, encodedTotal    int64
	dimension                     int
	pixels, decodedSource, output int64
}

var defaultLimits = limits{MaxEncodedImageBytes, MaxEncodedBytes, MaxImageDimension, MaxImagePixels, MaxDecodedSourceBytes, MaxOutputBytes}

type usage struct{ encoded, decodedSource, output int64 }

// Import decodes each full source path once, then emits each distinct image key
// once across static sprites and their animation frames. emit returns the
// caller's nonempty output name; the result maps image keys to those names.
// Actual PNG dimensions and source/output budgets are checked before allocation.
// Source paths must be valid relative fs paths without backslashes, colons or NUL.
// The caller's FS defines physical and symlink confinement. A callback error
// stops import; earlier callback side effects are not undone.
func Import(fsys fs.FS, level *tiled.Map, emit func(Input) (string, error)) (map[string]string, error) {
	return importWithLimits(fsys, level, emit, defaultLimits)
}
func importWithLimits(fsys fs.FS, level *tiled.Map, emit func(Input) (string, error), caps limits) (map[string]string, error) {
	if fsys == nil || level == nil || emit == nil {
		return nil, fmt.Errorf("Tiled asset import requires a filesystem, map and callback")
	}
	aliases := map[string]string{}
	loaded := map[string]image.Image{}
	used := usage{}
	importImage := func(sprite tiled.Sprite) error {
		if !fs.ValidPath(sprite.Image) || strings.ContainsAny(sprite.Image, "\\:\x00") {
			return fmt.Errorf("invalid Tiled image path %q", sprite.Image)
		}
		key := tiled.ImageKey(sprite)
		if _, ok := aliases[key]; ok {
			return nil
		}
		im := loaded[sprite.Image]
		if im == nil {
			var e error
			im, e = loadPNG(fsys, sprite.Image, &used, caps)
			if e != nil {
				return e
			}
			loaded[sprite.Image] = im
		}
		rect, e := sourceRectangle(im.Bounds(), sprite)
		if e != nil {
			return e
		}
		outputBytes := int64(rect.Dx()) * int64(rect.Dy()) * 4
		if outputBytes > caps.output-used.output {
			return fmt.Errorf("%w: emitted pixels for %s exceed %d bytes", ErrLimit, key, caps.output)
		}
		used.output += outputBytes
		pixels := image.NewNRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
		draw.Draw(pixels, pixels.Bounds(), im, rect.Min, draw.Src)
		name, e := emit(Input{Key: key, Source: sprite.Image, Rectangle: rect, SourceBounds: im.Bounds(), Pixels: pixels})
		if e != nil {
			return fmt.Errorf("emit Tiled image %s: %w", key, e)
		}
		if name == "" {
			return fmt.Errorf("emit Tiled image %s returned an empty output name", key)
		}
		aliases[key] = name
		return nil
	}
	for _, sprite := range level.Sprites {
		if e := importImage(sprite); e != nil {
			return nil, e
		}
		for _, frame := range sprite.Frames {
			if e := importImage(frame.Sprite); e != nil {
				return nil, e
			}
		}
	}
	return aliases, nil
}

// encodedReader counts the physical source stream once, including trailing
// bytes. DecodeConfig's small prefix is replayed without charging it twice.
type encodedReader struct {
	r          io.Reader
	imageBytes int64
	used       *usage
	caps       limits
	limitErr   error
}

func (r *encodedReader) Read(p []byte) (int, error) {
	if r.limitErr != nil {
		return 0, r.limitErr
	}
	if len(p) == 0 {
		return 0, nil
	}
	remaining := min(r.caps.encodedImage-r.imageBytes, r.caps.encodedTotal-r.used.encoded)
	// One-byte lookahead distinguishes exact-limit EOF from an oversized file.
	if int64(len(p)) > remaining+1 {
		p = p[:int(remaining+1)]
	}
	n, e := r.r.Read(p)
	r.imageBytes += int64(n)
	r.used.encoded += int64(n)
	if int64(n) > remaining {
		r.limitErr = fmt.Errorf("%w: encoded PNG stream exceeds per-image or aggregate byte budget", ErrLimit)
		return int(remaining), r.limitErr
	}
	return n, e
}
func loadPNG(fsys fs.FS, name string, used *usage, caps limits) (out image.Image, err error) {
	f, e := fsys.Open(name)
	if e != nil {
		return nil, fmt.Errorf("Tiled image %s: %w", name, e)
	}
	defer func() {
		if e := f.Close(); err == nil && e != nil {
			err = fmt.Errorf("close Tiled image %s: %w", name, e)
		}
	}()
	bounded := &encodedReader{r: f, used: used, caps: caps}
	var prefix bytes.Buffer
	config, e := png.DecodeConfig(io.TeeReader(bounded, &prefix))
	if bounded.limitErr != nil {
		return nil, fmt.Errorf("Tiled image %s: %w", name, bounded.limitErr)
	}
	if e != nil {
		return nil, fmt.Errorf("Tiled PNG config %s: %w", name, e)
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > caps.dimension || config.Height > caps.dimension {
		return nil, fmt.Errorf("%w: PNG dimensions %dx%d for %s exceed %d", ErrLimit, config.Width, config.Height, name, caps.dimension)
	}
	pixels := int64(config.Width) * int64(config.Height)
	if pixels > caps.pixels {
		return nil, fmt.Errorf("%w: PNG pixel count %d for %s exceeds %d", ErrLimit, pixels, name, caps.pixels)
	}
	sourceBytes := pixels * 8
	if sourceBytes > caps.decodedSource-used.decodedSource {
		return nil, fmt.Errorf("%w: decoded PNG sources exceed %d bytes", ErrLimit, caps.decodedSource)
	}
	used.decodedSource += sourceBytes
	out, e = png.Decode(io.MultiReader(bytes.NewReader(prefix.Bytes()), bounded))
	if bounded.limitErr != nil {
		return nil, fmt.Errorf("Tiled image %s: %w", name, bounded.limitErr)
	}
	if e != nil {
		return nil, fmt.Errorf("Tiled image %s: %w", name, e)
	}
	// Account for all encoded bytes, even when a PNG decoder stops at IEND.
	if _, e = io.Copy(io.Discard, bounded); e != nil {
		return nil, fmt.Errorf("Tiled image %s: %w", name, e)
	}
	return out, nil
}
func sourceRectangle(bounds image.Rectangle, s tiled.Sprite) (image.Rectangle, error) {
	x, y, w, h := s.SourceX, s.SourceY, s.SourceWidth, s.SourceHeight
	// Subtraction after checking the origin avoids overflowing x+w or y+h.
	if x < 0 || y < 0 || w <= 0 || h <= 0 || x > bounds.Dx() || y > bounds.Dy() || w > bounds.Dx()-x || h > bounds.Dy()-y {
		return image.Rectangle{}, fmt.Errorf(
			"Tiled source rectangle %d,%d,%d,%d exceeds %s (%dx%d)",
			x,
			y,
			w,
			h,
			s.Image,
			bounds.Dx(),
			bounds.Dy(),
		)
	}
	return image.Rect(bounds.Min.X+x, bounds.Min.Y+y, bounds.Min.X+x+w, bounds.Min.Y+y+h), nil
}
