package tiled

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"math"
	"strings"
	"unicode/utf8"
)

const MaxCompiledBytes = 64 << 20
const maxCompiledAllocation = 128 << 20
const maxProperties = 256

type mapWriter struct {
	data    []byte
	invalid bool
}

func (w *mapWriter) add(n int) []byte {
	if w.invalid || n < 0 || n > MaxCompiledBytes-len(w.data) {
		w.invalid = true
		return nil
	}
	at := len(w.data)
	w.data = append(w.data, make([]byte, n)...)
	return w.data[at:]
}
func (w *mapWriter) u8(v byte) {
	if b := w.add(1); b != nil {
		b[0] = v
	}
}
func (w *mapWriter) u32(v uint32) {
	if b := w.add(4); b != nil {
		binary.LittleEndian.PutUint32(b, v)
	}
}
func (w *mapWriter) integer(v, max int) {
	if v < 0 || v > max {
		w.invalid = true
		return
	}
	w.u32(uint32(v))
}
func (w *mapWriter) number(v float64) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		w.invalid = true
		return
	}
	if b := w.add(8); b != nil {
		binary.LittleEndian.PutUint64(b, math.Float64bits(v))
	}
}
func (w *mapWriter) str(s string, max int) {
	if len(s) > max || !utf8.ValidString(s) {
		w.invalid = true
		return
	}
	w.u32(uint32(len(s)))
	copy(w.add(len(s)), s)
}
func (w *mapWriter) properties(properties Properties) {
	w.integer(len(properties), maxProperties)
	for _, p := range properties {
		w.str(p.Name, 1024)
		w.str(p.Type, 256)
		switch v := p.Value.(type) {
		case nil:
			w.u8(0)
		case bool:
			if v {
				w.u8(2)
			} else {
				w.u8(1)
			}
		case float64:
			w.u8(3)
			w.number(v)
		case string:
			w.u8(4)
			w.str(v, 65536)
		default:
			w.invalid = true
		}
	}
}
func (w *mapWriter) sprite(s Sprite) {
	if !validSprite(s) {
		w.invalid = true
	}
	w.str(s.Image, 1024)
	for _, v := range []int{s.SourceX, s.SourceY, s.SourceWidth, s.SourceHeight} {
		w.integer(v, MaxImageDimension)
	}
	for _, v := range []float64{s.X, s.Y, s.Width, s.Height, s.Rotation, s.Opacity, s.SortY} {
		w.number(v)
	}
	w.integer(s.Layer, MaxLayers-1)
	w.integer(s.ObjectID, math.MaxInt32)
	flags := byte(0)
	if s.FlipX {
		flags |= 1
	}
	if s.FlipY {
		flags |= 2
	}
	if s.Diagonal {
		flags |= 4
	}
	w.u8(flags)
	w.properties(s.Properties)
}

// Encode compiles the flattened map. Editable layer/tileset documents stay build-side.
func Encode(m *Map) ([]byte, error) {
	if m == nil {
		return nil, fmt.Errorf("nil Tiled map")
	}
	w := mapWriter{data: make([]byte, 16)}
	copy(w.data, "KTM1")
	binary.LittleEndian.PutUint16(w.data[4:], 1)
	w.integer(m.Width, MaxMapDimension)
	w.integer(m.Height, MaxMapDimension)
	w.integer(m.TileWidth, MaxImageDimension)
	w.integer(m.TileHeight, MaxImageDimension)
	w.properties(m.Properties)
	w.integer(len(m.LayerNames), MaxLayers)
	for _, s := range m.LayerNames {
		w.str(s, 1024)
	}
	w.integer(len(m.Sprites), MaxSprites)
	for _, s := range m.Sprites {
		w.sprite(s)
		w.integer(len(s.Frames), MaxAnimationFrames)
		for _, f := range s.Frames {
			if len(f.Sprite.Frames) != 0 {
				w.invalid = true
			}
			w.sprite(f.Sprite)
			w.integer(f.Duration, 60000)
			if f.Duration == 0 {
				w.invalid = true
			}
		}
	}
	w.integer(len(m.Objects), MaxObjects)
	for _, o := range m.Objects {
		w.str(o.Name, 1024)
		w.str(o.Class, 1024)
		for _, v := range []float64{o.X, o.Y, o.Width, o.Height} {
			w.number(v)
		}
		w.integer(len(o.Polygon), MaxObjects)
		for _, p := range o.Polygon {
			w.number(p.X)
			w.number(p.Y)
		}
		w.properties(o.Properties)
	}
	if w.invalid {
		return nil, fmt.Errorf("invalid compiled Tiled map: %w", ErrLimit)
	}
	binary.LittleEndian.PutUint32(w.data[8:], uint32(len(w.data)-16))
	if _, err := decodeMap(w.data, false); err != nil {
		return nil, err
	}
	return w.data, nil
}

// Load reads only a compiled KTM1 record. JSON belongs to the source subpackage.
func Load(fsys fs.FS, name string) (*Map, error) {
	if fsys == nil || !fs.ValidPath(name) || strings.ContainsAny(name, "\\:\x00") {
		return nil, fmt.Errorf("invalid compiled Tiled path")
	}
	f, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxCompiledBytes+1))
	closeErr := f.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return Decode(data)
}
func Decode(data []byte) (*Map, error) {
	if _, err := decodeMap(data, false); err != nil {
		return nil, err
	}
	return decodeMap(data, true)
}

type mapReader struct {
	data                       []byte
	at, budget, frames, points int
	invalid, build             bool
}

func (r *mapReader) take(n int) []byte {
	if r.invalid || n < 0 || n > len(r.data)-r.at {
		r.invalid = true
		return nil
	}
	b := r.data[r.at : r.at+n]
	r.at += n
	return b
}
func (r *mapReader) u8() byte {
	if b := r.take(1); b != nil {
		return b[0]
	}
	return 0
}
func (r *mapReader) u32() uint32 {
	if b := r.take(4); b != nil {
		return binary.LittleEndian.Uint32(b)
	}
	return 0
}
func (r *mapReader) integer(max int) int {
	v := r.u32()
	if uint64(v) > uint64(max) {
		r.invalid = true
		return 0
	}
	return int(v)
}
func (r *mapReader) reserve(n int) {
	if n < 0 || n > maxCompiledAllocation-r.budget {
		r.invalid = true
		return
	}
	r.budget += n
}
func (r *mapReader) count(max, minBytes, itemBytes int) int {
	n := r.integer(max)
	if r.invalid || n > (len(r.data)-r.at)/minBytes {
		r.invalid = true
		return 0
	}
	r.reserve(n * itemBytes)
	if r.invalid {
		return 0
	}
	return n
}
func (r *mapReader) number() float64 {
	b := r.take(8)
	if b == nil {
		return 0
	}
	v := math.Float64frombits(binary.LittleEndian.Uint64(b))
	if math.IsNaN(v) || math.IsInf(v, 0) {
		r.invalid = true
	}
	return v
}
func (r *mapReader) text(max int) []byte {
	n := r.integer(max)
	b := r.take(n)
	if !utf8.Valid(b) {
		r.invalid = true
	}
	r.reserve(n)
	return b
}
func (r *mapReader) str(max int) string {
	b := r.text(max)
	if r.build {
		return string(b)
	}
	return ""
}
func (r *mapReader) path() string {
	b := r.text(1024)
	if !fs.ValidPath(string(b)) || strings.ContainsAny(string(b), "\\:\x00") {
		r.invalid = true
	}
	if r.build {
		return string(b)
	}
	return ""
}
func (r *mapReader) properties() Properties {
	n := r.count(maxProperties, 9, 64)
	var out Properties
	if r.build && n > 0 {
		out = make(Properties, n)
	}
	for i := 0; i < n; i++ {
		p := Property{Name: r.str(1024), Type: r.str(256)}
		switch r.u8() {
		case 0:
		case 1:
			p.Value = false
		case 2:
			p.Value = true
		case 3:
			p.Value = r.number()
		case 4:
			p.Value = r.str(65536)
		default:
			r.invalid = true
		}
		if r.build {
			out[i] = p
		}
	}
	return out
}
func validCoordinate(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && math.Abs(v) <= MaxCoordinate
}
func validSprite(s Sprite) bool {
	return fs.ValidPath(s.Image) && !strings.ContainsAny(s.Image, "\\:\x00") && validSpriteNumbers(s)
}
func validSpriteNumbers(s Sprite) bool {
	return validCoordinate(s.X) && validCoordinate(s.Y) && validCoordinate(s.Width) && validCoordinate(s.Height) &&
		validCoordinate(s.SortY) &&
		s.Width > 0 &&
		s.Height > 0 &&
		s.SourceWidth > 0 &&
		s.SourceHeight > 0 &&
		s.SourceX >= 0 && s.SourceX <= MaxImageDimension-s.SourceWidth &&
		s.SourceY >= 0 && s.SourceY <= MaxImageDimension-s.SourceHeight &&
		s.Opacity >= 0 &&
		s.Opacity <= 1
}
func (r *mapReader) sprite() Sprite {
	s := Sprite{
		Image:        r.path(),
		SourceX:      r.integer(MaxImageDimension),
		SourceY:      r.integer(MaxImageDimension),
		SourceWidth:  r.integer(MaxImageDimension),
		SourceHeight: r.integer(MaxImageDimension),
		X:            r.number(),
		Y:            r.number(),
		Width:        r.number(),
		Height:       r.number(),
		Rotation:     r.number(),
		Opacity:      r.number(),
		SortY:        r.number(),
		Layer:        r.integer(MaxLayers - 1),
		ObjectID:     r.integer(math.MaxInt32),
	}
	flags := r.u8()
	if flags&^7 != 0 || !validSpriteNumbers(s) {
		r.invalid = true
	}
	s.FlipX, s.FlipY, s.Diagonal = flags&1 != 0, flags&2 != 0, flags&4 != 0
	s.Properties = r.properties()
	return s
}
func decodeMap(data []byte, build bool) (*Map, error) {
	r := mapReader{data: data, at: 16, build: build}
	if len(data) < 16 || len(data) > MaxCompiledBytes || string(data[:4]) != "KTM1" || binary.LittleEndian.Uint16(data[4:]) != 1 ||
		binary.LittleEndian.Uint16(data[6:]) != 0 ||
		uint64(binary.LittleEndian.Uint32(data[8:])) != uint64(len(data)-16) ||
		binary.LittleEndian.Uint32(data[12:]) != 0 {
		return nil, fmt.Errorf("incompatible compiled Tiled header")
	}
	m := &Map{
		Document: Document{
			Orientation: "orthogonal",
			RenderOrder: "right-down",
			Width:       r.integer(MaxMapDimension),
			Height:      r.integer(MaxMapDimension),
			TileWidth:   r.integer(MaxImageDimension),
			TileHeight:  r.integer(MaxImageDimension),
		},
	}
	if m.Width == 0 || m.Height == 0 || m.TileWidth == 0 || m.TileHeight == 0 || !validCoordinate(float64(m.Width)*float64(m.TileWidth)) ||
		!validCoordinate(float64(m.Height)*float64(m.TileHeight)) {
		r.invalid = true
	}
	m.Properties = r.properties()
	n := r.count(MaxLayers, 4, 16)
	layers := n
	if build && n > 0 {
		m.LayerNames = make([]string, n)
	}
	for i := 0; i < n; i++ {
		s := r.str(1024)
		if build {
			m.LayerNames[i] = s
		}
	}
	n = r.count(MaxSprites, 93, 256)
	if build && n > 0 {
		m.Sprites = make([]Sprite, n)
	}
	for i := 0; i < n; i++ {
		s := r.sprite()
		if s.Layer >= layers {
			r.invalid = true
		}
		frames := r.count(MaxAnimationFrames, 93, 256)
		r.frames += frames
		if r.frames > MaxTotalAnimationFrames {
			r.invalid = true
		}
		if build && frames > 0 {
			s.Frames = make([]Frame, frames)
		}
		for j := 0; j < frames; j++ {
			f := Frame{Sprite: r.sprite(), Duration: r.integer(60000)}
			if f.Duration == 0 || f.Sprite.Layer >= layers {
				r.invalid = true
			}
			if build {
				s.Frames[j] = f
			}
		}
		if build {
			m.Sprites[i] = s
		}
	}
	n = r.count(MaxObjects, 48, 128)
	if build && n > 0 {
		m.Objects = make([]Region, n)
	}
	for i := 0; i < n; i++ {
		o := Region{Name: r.str(1024), Class: r.str(1024), X: r.number(), Y: r.number(), Width: r.number(), Height: r.number()}
		if !validCoordinate(o.X) || !validCoordinate(o.Y) || !validCoordinate(o.Width) || !validCoordinate(o.Height) || o.Width < 0 ||
			o.Height < 0 {
			r.invalid = true
		}
		points := r.count(MaxObjects, 16, 16)
		r.points += points
		if r.points > MaxObjects {
			r.invalid = true
		}
		if build && points > 0 {
			o.Polygon = make([]Point, points)
		}
		for j := 0; j < points; j++ {
			p := Point{r.number(), r.number()}
			if !validCoordinate(p.X) || !validCoordinate(p.Y) {
				r.invalid = true
			}
			if build {
				o.Polygon[j] = p
			}
		}
		o.Properties = r.properties()
		if build {
			m.Objects[i] = o
		}
	}
	if r.invalid || r.at != len(data) {
		return nil, fmt.Errorf("malformed or oversized compiled Tiled map: %w", ErrLimit)
	}
	return m, nil
}
