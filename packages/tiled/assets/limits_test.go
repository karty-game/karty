package assets

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/karty-game/tiled"
)

func configOnly(width, height uint32) []byte {
	data := make([]byte, 13)
	binary.BigEndian.PutUint32(data, width)
	binary.BigEndian.PutUint32(data[4:], height)
	data[8] = 8
	data[9] = 6
	var out bytes.Buffer
	out.WriteString("\x89PNG\r\n\x1a\n")
	_ = binary.Write(&out, binary.BigEndian, uint32(13))
	out.WriteString("IHDR")
	out.Write(data)
	_ = binary.Write(&out, binary.BigEndian, crc32.ChecksumIEEE(append([]byte("IHDR"), data...)))
	return out.Bytes()
}
func tinyPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	im := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			im.SetNRGBA(x, y, color.NRGBA{R: 20, G: 40, B: 60, A: 255})
		}
	}
	var b bytes.Buffer
	if e := png.Encode(&b, im); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func fixtureLevel(names ...string) *tiled.Map {
	m := &tiled.Map{}
	for _, name := range names {
		m.Sprites = append(m.Sprites, tiled.Sprite{Image: name, SourceWidth: 1, SourceHeight: 1})
	}
	return m
}
func TestActualPNGHeaderBoundsAreRejectedBeforePixelDecode(t *testing.T) {
	for _, test := range []struct {
		w, h   uint32
		reason string
	}{{MaxImageDimension + 1, 1, "dimensions"}, {8192, 2049, "pixel count"}} {
		files := fstest.MapFS{"large.png": {Data: configOnly(test.w, test.h)}}
		called := false
		_, e := Import(files, fixtureLevel("large.png"), func(Input) (string, error) { called = true; return "output", nil })
		if !errors.Is(e, ErrLimit) || !strings.Contains(e.Error(), test.reason) || called {
			t.Fatalf("config-only oversized image reached pixel decode/emission: %v", e)
		}
	}
}
func TestEncodedLimitsIncludeTrailingBytesAndAllowExactLimit(t *testing.T) {
	raw := tinyPNG(t, 1, 1)
	caps := defaultLimits
	caps.encodedImage = int64(len(raw) + 8)
	caps.encodedTotal = caps.encodedImage
	for _, extra := range []int{8, 9} {
		files := fstest.MapFS{"image.png": {Data: append(append([]byte(nil), raw...), make([]byte, extra)...)}}
		called := false
		_, e := importWithLimits(
			files,
			fixtureLevel("image.png"),
			func(Input) (string, error) { called = true; return "output", nil },
			caps,
		)
		if extra == 8 {
			if e != nil || !called {
				t.Fatalf("exact encoded limit rejected: %v", e)
			}
		} else if !errors.Is(e, ErrLimit) || called {
			t.Fatalf("oversized encoded stream emitted: %v", e)
		}
	}
}
func TestAggregateEncodedLimitIsCheckedAcrossUniqueSources(t *testing.T) {
	raw := tinyPNG(t, 1, 1)
	files := fstest.MapFS{"a.png": {Data: raw}, "b.png": {Data: raw}}
	caps := defaultLimits
	caps.encodedImage = int64(len(raw))
	caps.encodedTotal = int64(len(raw)*2 - 1)
	calls := 0
	_, e := importWithLimits(files, fixtureLevel("a.png", "b.png"), func(Input) (string, error) { calls++; return "output", nil }, caps)
	if !errors.Is(e, ErrLimit) || calls != 1 {
		t.Fatalf("aggregate encoded budget not enforced: calls%d err%v", calls, e)
	}
}

type countSources struct {
	fs.FS
	opens int
}

func (f *countSources) Open(name string) (fs.File, error) { f.opens++; return f.FS.Open(name) }
func TestRepeatedSpritesFlipsAndFramesDoNotSpendBudgetsTwice(t *testing.T) {
	raw := tinyPNG(t, 1, 1)
	caps := defaultLimits
	caps.encodedImage = int64(len(raw))
	caps.encodedTotal = int64(len(raw))
	caps.decodedSource = 8
	caps.output = 4
	m := fixtureLevel("image.png")
	base := m.Sprites[0]
	for i := range 32 {
		s := base
		s.FlipX = i%2 == 0
		s.FlipY = i%3 == 0
		s.Frames = []tiled.Frame{{Sprite: base, Duration: 100}}
		m.Sprites = append(m.Sprites, s)
	}
	files := &countSources{FS: fstest.MapFS{"image.png": {Data: raw}}}
	calls := 0
	aliases, e := importWithLimits(files, m, func(Input) (string, error) { calls++; return "output", nil }, caps)
	if e != nil || calls != 1 || files.opens != 1 || len(aliases) != 1 {
		t.Fatalf("dedup spent resource budgets twice: calls%d opens%d err%v", calls, files.opens, e)
	}
}
func TestDecodedSourceBudgetRejectsNextHeaderBeforeDecode(t *testing.T) {
	files := fstest.MapFS{"a.png": {Data: tinyPNG(t, 2, 2)}, "b.png": {Data: configOnly(2, 2)}}
	caps := defaultLimits
	caps.decodedSource = 2 * 2 * 8
	calls := 0
	_, e := importWithLimits(files, fixtureLevel("a.png", "b.png"), func(Input) (string, error) { calls++; return "output", nil }, caps)
	if !errors.Is(e, ErrLimit) || !strings.Contains(e.Error(), "decoded PNG sources") || calls != 1 {
		t.Fatalf("aggregate source pixels reached decode: calls%d err%v", calls, e)
	}
}
func TestDistinctOverlappingCropsHaveAnOutputAllocationBudget(t *testing.T) {
	files := fstest.MapFS{"image.png": {Data: tinyPNG(t, 2, 2)}}
	caps := defaultLimits
	caps.output = 2 * 2 * 4
	m := &tiled.Map{
		Sprites: []tiled.Sprite{
			{Image: "image.png", SourceWidth: 2, SourceHeight: 2},
			{Image: "image.png", SourceX: 1, SourceWidth: 1, SourceHeight: 2},
		},
	}
	calls := 0
	_, e := importWithLimits(files, m, func(Input) (string, error) { calls++; return "output", nil }, caps)
	if !errors.Is(e, ErrLimit) || calls != 1 {
		t.Fatalf("distinct crops exceeded output allocation budget: calls%d err%v", calls, e)
	}
}
