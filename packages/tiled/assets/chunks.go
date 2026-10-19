package assets

import (
	"crypto/sha256"
	"fmt"
	"image"
	"image/draw"
	"io/fs"
	"math"
	"sort"
	"strings"

	"github.com/karty-game/tiled"
)

// StaticChunk is one bounded, transparent image in a background layer. X/Y are
// world pixels. The callback stores Pixels and returns its relative image path.
type StaticChunk struct {
	Layer, X, Y int
	Pixels      *image.NRGBA
}

// ChunkOptions keeps source resolution explicit: Size is in world pixels and
// PixelScale is the integer source-pixels/world-pixel ratio (zero selects one).
// The resulting image must fit 1024x1024 pixels.
type ChunkOptions struct{ Size, PixelScale int }

const maxStaticChunks = 1024
const maxChunkPlacements = 1 << 20

// MaxStaticRasterBytes bounds cumulative placement work independently of image
// deduplication and overlapping output. Charge it before decoding or rasterizing.
const MaxStaticRasterBytes = 4 * MaxOutputBytes

type chunkKey struct{ layer, x, y int }

// BakeStaticLayers composes explicitly selected background tile layers. Never
// select an actor/Y-sorted layer: objects, wind, animation, rotation, fractional
// placement and layer opacity are rejected. Source PNG alpha and all orthogonal
// flips survive without downsampling: geometry times PixelScale must retain every
// source pixel (integer replication is allowed). Identical placement patterns share an emitted image. Input
// maps stay unchanged. Import's source limits and MaxOutputBytes apply; chunks
// are rasterized sequentially so repeated terrain cannot allocate a map atlas.
func BakeStaticLayers(
	fsys fs.FS,
	level *tiled.Map,
	names []string,
	options ChunkOptions,
	emit func(StaticChunk) (string, error),
) ([]tiled.Sprite, error) {
	if options.PixelScale == 0 {
		options.PixelScale = 1
	}
	if level == nil || emit == nil || len(level.LayerNames) > tiled.MaxLayers || len(names) > tiled.MaxLayers ||
		options.Size < 1 || options.PixelScale < 1 || options.PixelScale > 1024 || options.Size > 1024/options.PixelScale ||
		len(level.Sprites) > tiled.MaxSprites {
		return nil, fmt.Errorf("invalid static Tiled chunk inputs")
	}
	selected, err := staticLayerIndices(level, names)
	if err != nil {
		return nil, err
	}
	static := &tiled.Map{}
	var rasterBytes int64
	for _, sprite := range level.Sprites {
		if !selected[sprite.Layer] {
			continue
		}
		if err := validateStaticSprite(sprite, options.PixelScale); err != nil {
			return nil, err
		}
		work := int64(sprite.Width*float64(options.PixelScale)) * int64(sprite.Height*float64(options.PixelScale)) * 4
		if work > MaxStaticRasterBytes-rasterBytes {
			return nil, fmt.Errorf("%w: static Tiled placement raster work", ErrLimit)
		}
		rasterBytes += work
		static.Sprites = append(static.Sprites, sprite)
	}
	// Match the runtime's stable layer/Y painter order before planning chunks.
	sort.SliceStable(static.Sprites, func(i, j int) bool {
		a, b := static.Sprites[i], static.Sprites[j]
		return a.Layer < b.Layer || a.Layer == b.Layer && a.SortY < b.SortY
	})
	plan, keys, err := planStaticChunks(static.Sprites, options.Size)
	if err != nil {
		return nil, err
	}
	pixels := map[string]*image.NRGBA{}
	_, err = Import(fsys, static, func(input Input) (string, error) {
		pixels[input.Key] = input.Pixels
		return input.Key, nil
	})
	if err != nil {
		return nil, err
	}
	return emitStaticChunks(static.Sprites, plan, keys, pixels, options, rasterBytes, emit)
}

func staticLayerIndices(level *tiled.Map, names []string) (map[int]bool, error) {
	selected := map[int]bool{}
	for _, name := range names {
		found := false
		for index, candidate := range level.LayerNames {
			if candidate == name {
				selected[index], found = true, true
			}
		}
		if !found {
			return nil, fmt.Errorf("unknown static Tiled layer %q", name)
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("static Tiled chunk layers are empty")
	}
	return selected, nil
}

func validateStaticSprite(s tiled.Sprite, scale int) error {
	if s.ObjectID != 0 || len(s.Frames) != 0 || s.Properties.Number("windAmplitude", 0) != 0 || s.Rotation != 0 || s.Opacity != 1 ||
		s.Width <= 0 || s.Height <= 0 || s.Width*float64(scale) > 1024 || s.Height*float64(scale) > 1024 ||
		s.Width*float64(scale) < float64(s.SourceWidth) || s.Height*float64(scale) < float64(s.SourceHeight) ||
		math.Mod(s.Width*float64(scale), float64(s.SourceWidth)) != 0 || math.Mod(s.Height*float64(scale), float64(s.SourceHeight)) != 0 ||
		s.SourceWidth <= 0 || s.SourceHeight <= 0 || s.SourceWidth > 1024 || s.SourceHeight > 1024 ||
		math.IsNaN(s.X) || math.IsNaN(s.Y) || math.Abs(s.X) > tiled.MaxCoordinate || math.Abs(s.Y) > tiled.MaxCoordinate ||
		s.X != math.Trunc(s.X) || s.Y != math.Trunc(s.Y) || s.Width != math.Trunc(s.Width) || s.Height != math.Trunc(s.Height) ||
		math.IsNaN(s.SortY) || math.Abs(s.SortY) > tiled.MaxCoordinate {
		return fmt.Errorf("static Tiled layer %d requires integer, nonanimated background tiles matching the pixel scale", s.Layer)
	}
	return nil
}

func staticSpriteBounds(sprite tiled.Sprite) image.Rectangle {
	cx, cy := tiled.Center(sprite)
	sx, sy, angle := tiled.Transform(sprite)
	cosine, sine := math.Cos(angle), math.Sin(angle)
	halfW, halfH := float64(sprite.SourceWidth)/2, float64(sprite.SourceHeight)/2
	extentX := math.Abs(halfW*sx*cosine) + math.Abs(halfH*sy*sine)
	extentY := math.Abs(halfW*sx*sine) + math.Abs(halfH*sy*cosine)
	return image.Rect(int(math.Round(cx-extentX)), int(math.Round(cy-extentY)), int(math.Round(cx+extentX)), int(math.Round(cy+extentY)))
}

func planStaticChunks(sprites []tiled.Sprite, size int) (map[chunkKey][]int, []chunkKey, error) {
	plan := map[chunkKey][]int{}
	placements := 0
	for index, sprite := range sprites {
		bounds := staticSpriteBounds(sprite)
		for y := int(math.Floor(float64(bounds.Min.Y)/float64(size))) * size; y < bounds.Max.Y; y += size {
			for x := int(math.Floor(float64(bounds.Min.X)/float64(size))) * size; x < bounds.Max.X; x += size {
				key := chunkKey{sprite.Layer, x, y}
				if _, exists := plan[key]; !exists && len(plan) >= maxStaticChunks || placements >= maxChunkPlacements {
					return nil, nil, fmt.Errorf("%w: static Tiled chunk count or placement references", ErrLimit)
				}
				plan[key] = append(plan[key], index)
				placements++
			}
		}
	}
	keys := make([]chunkKey, 0, len(plan))
	for key := range plan {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		return a.layer < b.layer || a.layer == b.layer && (a.y < b.y || a.y == b.y && a.x < b.x)
	})
	return plan, keys, nil
}

func drawStaticSprite(destination *image.NRGBA, sprite tiled.Sprite, pixels *image.NRGBA, scale int) {
	sprite.X *= float64(scale)
	sprite.Y *= float64(scale)
	sprite.Width *= float64(scale)
	sprite.Height *= float64(scale)
	bounds := staticSpriteBounds(sprite).Intersect(destination.Rect)
	cx, cy := tiled.Center(sprite)
	sx, sy, angle := tiled.Transform(sprite)
	cosine, sine := math.Cos(angle), math.Sin(angle)
	halfW, halfH := float64(sprite.SourceWidth)/2, float64(sprite.SourceHeight)/2
	tile := image.NewNRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dx, dy := float64(x)+.5-cx, float64(y)+.5-cy
			u, v := (dx*cosine+dy*sine)/sx+halfW, (-dx*sine+dy*cosine)/sy+halfH
			if u >= 0 && v >= 0 && u < float64(pixels.Rect.Dx()) && v < float64(pixels.Rect.Dy()) {
				tile.SetNRGBA(x, y, pixels.NRGBAAt(int(u), int(v)))
			}
		}
	}
	r := bounds.Intersect(destination.Rect)
	draw.Draw(destination, r, tile, r.Min, draw.Over)
}

func staticPattern(sprites []tiled.Sprite, indices []int, key chunkKey) [32]byte {
	hash := sha256.New()
	for _, index := range indices {
		s := sprites[index]
		fmt.Fprintf(hash, "%q/%d/%d/%d/%d/%g/%g/%g/%g/%t/%t/%t;", s.Image, s.SourceX, s.SourceY, s.SourceWidth, s.SourceHeight,
			s.X-float64(key.x), s.Y-float64(key.y), s.Width, s.Height, s.FlipX, s.FlipY, s.Diagonal)
	}
	var sum [32]byte
	copy(sum[:], hash.Sum(nil))
	return sum
}

func emitStaticChunks(sprites []tiled.Sprite, plan map[chunkKey][]int, keys []chunkKey, pixels map[string]*image.NRGBA,
	options ChunkOptions, rasterBytes int64, emit func(StaticChunk) (string, error)) ([]tiled.Sprite, error) {
	contents := map[[32]byte]string{}
	patterns := map[[32]byte]string{}
	result := make([]tiled.Sprite, 0, len(keys))
	pixelSize := options.Size * options.PixelScale
	var outputBytes int64
	for _, key := range keys {
		pattern := staticPattern(sprites, plan[key], key)
		name, exists := patterns[pattern]
		if !exists {
			charge := int64(pixelSize) * int64(pixelSize) * 4
			if charge > MaxOutputBytes-outputBytes {
				return nil, fmt.Errorf("%w: unique static Tiled chunk pixels", ErrLimit)
			}
			if charge > MaxStaticRasterBytes-rasterBytes {
				return nil, fmt.Errorf("%w: static Tiled chunk raster work", ErrLimit)
			}
			rasterBytes += charge
			x, y := key.x*options.PixelScale, key.y*options.PixelScale
			chunk := image.NewNRGBA(image.Rect(x, y, x+pixelSize, y+pixelSize))
			for _, index := range plan[key] {
				sprite := sprites[index]
				drawStaticSprite(chunk, sprite, pixels[tiled.ImageKey(sprite)], options.PixelScale)
			}
			chunk.Rect = chunk.Rect.Sub(chunk.Rect.Min)
			content := sha256.Sum256(chunk.Pix)
			name, exists = contents[content]
			if !exists {
				var err error
				name, err = emit(StaticChunk{Layer: key.layer, X: key.x, Y: key.y, Pixels: chunk})
				if err != nil {
					return nil, err
				}
				if !fs.ValidPath(name) || strings.ContainsAny(name, "\\:\x00") {
					return nil, fmt.Errorf("invalid static Tiled chunk image path %q", name)
				}
				outputBytes += charge
				contents[content] = name
			}
			patterns[pattern] = name
		}
		result = append(result, tiled.Sprite{Image: name, SourceWidth: pixelSize, SourceHeight: pixelSize,
			X: float64(key.x), Y: float64(key.y), Width: float64(options.Size), Height: float64(options.Size),
			Opacity: 1, Layer: key.layer, SortY: float64(key.y + options.Size)})
	}
	return result, nil
}
