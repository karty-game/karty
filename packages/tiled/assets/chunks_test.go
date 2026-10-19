package assets

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/karty-game/tiled"
)

func chunkFixture(t *testing.T) (fstest.MapFS, *tiled.Map) {
	t.Helper()
	im := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for i, red := range []uint8{10, 20, 30, 40} {
		im.SetNRGBA(i%2, i/2, color.NRGBA{R: red, A: 255})
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, im); err != nil {
		t.Fatal(err)
	}
	return fstest.MapFS{"tile.png": {Data: encoded.Bytes()}}, &tiled.Map{LayerNames: []string{"Ground", "Actors"},
		Sprites: []tiled.Sprite{{Image: "tile.png", SourceWidth: 2, SourceHeight: 2, Width: 2, Height: 2, Opacity: 1}}}
}

func TestStaticChunksPreservePixelsAcrossAllFlipFlagsAndBoundaries(t *testing.T) {
	files, level := chunkFixture(t)
	// Independent pixel oracle for Tiled's eight orthogonal flags.
	want := [8][4]uint8{{10, 20, 30, 40}, {20, 10, 40, 30}, {30, 40, 10, 20}, {40, 30, 20, 10},
		{10, 30, 20, 40}, {30, 10, 40, 20}, {20, 40, 10, 30}, {40, 20, 30, 10}}
	for flags := range 8 {
		sprite := level.Sprites[0]
		sprite.X, sprite.Y = -1, -1
		sprite.FlipX, sprite.FlipY, sprite.Diagonal = flags&1 != 0, flags&2 != 0, flags&4 != 0
		level.Sprites = []tiled.Sprite{sprite}
		got := image.NewNRGBA(image.Rect(-2, -2, 2, 2))
		chunks, err := BakeStaticLayers(files, level, []string{"Ground"}, ChunkOptions{Size: 2}, func(chunk StaticChunk) (string, error) {
			r := chunk.Pixels.Bounds().Add(image.Pt(chunk.X, chunk.Y))
			draw.Draw(got, r, chunk.Pixels, image.Point{}, draw.Src)
			return fmt.Sprintf("chunk-%d-%d.png", chunk.X, chunk.Y), nil
		})
		if err != nil || len(chunks) != 4 {
			t.Fatal("tile crossing negative/positive chunk boundaries failed", flags, len(chunks), err)
		}
		for i, red := range want[flags] {
			if pixel := got.NRGBAAt(-1+i%2, -1+i/2); pixel.R != red || pixel.A != 255 {
				t.Fatalf("flip%d pixel%d: got%v wantred%d", flags, i, pixel, red)
			}
		}
	}
}

func TestStaticChunksRetainAlphaPainterOrderAndLeaveDynamicLayersUntouched(t *testing.T) {
	files, level := chunkFixture(t)
	overlay := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	overlay.SetNRGBA(0, 0, color.NRGBA{B: 255, A: 128})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, overlay); err != nil {
		t.Fatal(err)
	}
	files["overlay.png"] = &fstest.MapFile{Data: encoded.Bytes()}
	base := level.Sprites[0]
	upper, actor := base, base
	upper.Image, upper.SortY = "overlay.png", 1
	actor.Layer, actor.ObjectID, actor.Frames = 1, 7, []tiled.Frame{{Duration: 100}}
	level.Sprites = []tiled.Sprite{upper, actor, base}
	before := append([]tiled.Sprite(nil), level.Sprites...)
	var got *image.NRGBA
	_, err := BakeStaticLayers(files, level, []string{"Ground"}, ChunkOptions{Size: 2}, func(chunk StaticChunk) (string, error) {
		got = chunk.Pixels
		return "chunk.png", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if pixel := got.NRGBAAt(0, 0); pixel.R != 4 || pixel.B != 128 || pixel.A != 255 {
		t.Fatal("alpha or Y painter order changed", pixel)
	}
	if got.NRGBAAt(1, 1).R != 40 || !reflect.DeepEqual(before, level.Sprites) {
		t.Fatal("transparent pixels or source/dynamic map changed")
	}
}

func TestStaticChunkBoundsAndUnsupportedDynamicContent(t *testing.T) {
	files, level := chunkFixture(t)
	emit := func(StaticChunk) (string, error) { return "chunk.png", nil }
	for _, mutate := range []func(*tiled.Sprite){
		func(s *tiled.Sprite) { s.Frames = []tiled.Frame{{Duration: 100}} },
		func(s *tiled.Sprite) { s.ObjectID = 1 },
		func(s *tiled.Sprite) { s.Properties = tiled.Properties{{Name: "windAmplitude", Value: float64(1)}} },
		func(s *tiled.Sprite) { s.X = .5 },
		func(s *tiled.Sprite) { s.Rotation = 90 },
		func(s *tiled.Sprite) { s.Opacity = .5 },
		func(s *tiled.Sprite) { s.Width = 3 },
	} {
		invalid := *level
		invalid.Sprites = append([]tiled.Sprite(nil), level.Sprites...)
		mutate(&invalid.Sprites[0])
		if _, err := BakeStaticLayers(files, &invalid, []string{"Ground"}, ChunkOptions{Size: 2}, emit); err == nil {
			t.Fatal("unsupported background was silently flattened", invalid.Sprites[0])
		}
	}
	for _, size := range []int{0, 1025} {
		if _, err := BakeStaticLayers(files, level, []string{"Ground"}, ChunkOptions{Size: size}, emit); err == nil {
			t.Fatal("invalid chunk dimension accepted", size)
		}
	}
	if _, err := BakeStaticLayers(files, level, []string{"Missing"}, ChunkOptions{Size: 2}, emit); err == nil {
		t.Fatal("unknown layer accepted")
	}
	if _, err := BakeStaticLayers(
		files,
		level,
		[]string{"Ground"},
		ChunkOptions{Size: 2},
		func(StaticChunk) (string, error) { return "../escape.png", nil },
	); err == nil {
		t.Fatal("unsafe output image path accepted")
	}
	// Plan limits reject pathological cross-chunk expansion before PNG decoding.
	large := *level
	large.Sprites = []tiled.Sprite{{Image: "not-opened.png", SourceWidth: 512, SourceHeight: 512,
		Width: 512, Height: 512, Opacity: 1}}
	if _, err := BakeStaticLayers(fstest.MapFS{}, &large, []string{"Ground"}, ChunkOptions{Size: 1}, emit); !errors.Is(err, ErrLimit) {
		t.Fatal("spatial plan exceeded the bounded chunk count", err)
	}
}

func TestStaticChunksRejectRepeatedOverdrawBeforeImageDecode(t *testing.T) {
	level := &tiled.Map{LayerNames: []string{"Ground"}}
	for range MaxStaticRasterBytes/(512*512*4) + 1 {
		level.Sprites = append(level.Sprites, tiled.Sprite{Image: "not-opened.png", SourceWidth: 512, SourceHeight: 512,
			Width: 512, Height: 512, Opacity: 1})
	}
	_, err := BakeStaticLayers(fstest.MapFS{}, level, []string{"Ground"}, ChunkOptions{Size: 512},
		func(StaticChunk) (string, error) { t.Fatal("over-budget input reached output"); return "", nil })
	if !errors.Is(err, ErrLimit) {
		t.Fatal("overlapping placements bypassed the cumulative work limit", err)
	}
}

func TestStaticChunksPreserveSourceResolutionAndDeduplicateBeforeRasterAllocation(t *testing.T) {
	im := image.NewNRGBA(image.Rect(0, 0, 1024, 1024))
	im.SetNRGBA(131, 129, color.NRGBA{R: 197, A: 255})
	im.SetNRGBA(132, 129, color.NRGBA{B: 211, A: 255})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, im); err != nil {
		t.Fatal(err)
	}
	files := fstest.MapFS{"high-resolution.png": {Data: encoded.Bytes()}}
	level := &tiled.Map{LayerNames: []string{"Ground"}}
	for index := range 117 {
		level.Sprites = append(level.Sprites, tiled.Sprite{Image: "high-resolution.png", SourceWidth: 1024, SourceHeight: 1024,
			X: float64(index * 256), Width: 256, Height: 256, Opacity: 1})
	}
	emitted := 0
	placements, err := BakeStaticLayers(
		files,
		level,
		[]string{"Ground"},
		ChunkOptions{Size: 256, PixelScale: 4},
		func(chunk StaticChunk) (string, error) {
			emitted++
			if chunk.Pixels.Rect.Dx() != 1024 || !bytes.Equal(chunk.Pixels.Pix, im.Pix) {
				t.Fatal("source-resolution neighboring detail was resampled or lost")
			}
			return "shared-ground.png", nil
		},
	)
	if err != nil || emitted != 1 || len(placements) != 117 {
		t.Fatal("repeated terrain was not shared before emission", emitted, len(placements), err)
	}
	for _, sprite := range placements {
		if sprite.Image != "shared-ground.png" || sprite.SourceWidth != 1024 || sprite.Width != 256 {
			t.Fatal("source/world resolution or image sharing changed", sprite)
		}
	}
}
