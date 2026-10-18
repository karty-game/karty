// Package source imports editable Tiled JSON during builds, never in cartridges.
package source

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"path"
	"strings"

	"github.com/karty-game/tiled"
)

type parsedMap struct {
	tiled.Map
	frameCount int
}
type Property = tiled.Property
type Properties = tiled.Properties
type Object = tiled.Object
type Point = tiled.Point
type Layer = tiled.Layer
type Tile = tiled.Tile
type AnimationFrame = tiled.AnimationFrame
type Tileset = tiled.Tileset
type Document = tiled.Document
type Sprite = tiled.Sprite
type Frame = tiled.Frame
type Region = tiled.Region
type Map = tiled.Map

func resource(base, name string) (string, error) {
	if name == "" || strings.ContainsAny(name, "\x00\\:") || strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("invalid Tiled resource %q", name)
	}
	p := path.Clean(path.Join(path.Dir(base), name))
	if !fs.ValidPath(p) {
		return "", fmt.Errorf("Tiled resource escapes project: %q", name)
	}
	return p, nil
}
func decodeDocument(b []byte, value any) error { return json.Unmarshal(b, value) }

func Load(fsys fs.FS, name string) (*Map, error) {
	m := &parsedMap{}
	limits := loadLimits{}
	if fsys == nil || !fs.ValidPath(name) || strings.ContainsAny(name, "\x00\\:") {
		return nil, fmt.Errorf("invalid map path")
	}
	if e := limits.read(fsys, name, &m.Document); e != nil {
		return nil, e
	}
	if m.Orientation != "orthogonal" || m.Infinite || m.Width <= 0 || m.Height <= 0 || m.Width > MaxMapDimension ||
		m.Height > MaxMapDimension ||
		m.TileWidth <= 0 ||
		m.TileHeight <= 0 ||
		m.TileWidth > MaxImageDimension ||
		m.TileHeight > MaxImageDimension ||
		!coordinate(float64(m.Width)*float64(m.TileWidth)) ||
		!coordinate(float64(m.Height)*float64(m.TileHeight)) {
		return nil, fmt.Errorf("supported Tiled maps are finite orthogonal maps up to 256x256")
	}
	if len(m.Tilesets) > MaxDocuments-1 {
		return nil, ErrLimit
	}
	if m.RenderOrder != "" && m.RenderOrder != "right-down" {
		return nil, fmt.Errorf("unsupported Tiled render order %q", m.RenderOrder)
	}
	if err := limits.validateLayers(m.Layers, 1); err != nil {
		return nil, err
	}
	for i := range m.Tilesets {
		t := &m.Tilesets[i]
		base := name
		first := t.FirstGID
		if t.Source != "" {
			p, e := resource(name, t.Source)
			if e != nil {
				return nil, e
			}
			base = p
			source := t.Source
			var loaded Tileset
			if e = limits.read(fsys, p, &loaded); e != nil {
				return nil, e
			}
			if loaded.Source != "" {
				return nil, fmt.Errorf("nested external tilesets are unsupported")
			}
			*t = loaded
			t.Source = source
			t.FirstGID = first
		}
		if err := validateTileset(t); err != nil {
			return nil, err
		}
		end := tilesetEnd(t)
		if end > 0x0fffffff {
			return nil, fmt.Errorf("Tiled tileset GID range exceeds supported bounds")
		}
		for j := 0; j < i; j++ {
			other := &m.Tilesets[j]
			if uint64(other.FirstGID) <= end && uint64(t.FirstGID) <= tilesetEnd(other) {
				return nil, fmt.Errorf("overlapping Tiled tileset GID ranges")
			}
		}
		if t.Image != "" {
			p, e := resource(base, t.Image)
			if e != nil {
				return nil, e
			}
			t.Image = p
		}
		for j := range t.Tiles {
			if t.Tiles[j].Image != "" {
				p, e := resource(base, t.Tiles[j].Image)
				if e != nil {
					return nil, e
				}
				t.Tiles[j].Image = p
			}
		}
	}
	order := 0
	if e := m.layers(m.Layers, 0, 0, 1, true, &order); e != nil {
		return nil, e
	}
	return &m.Map, nil
}

func (m *parsedMap) tile(gid uint32) (Tile, *Tileset, error) {
	var set *Tileset
	for i := range m.Tilesets {
		t := &m.Tilesets[i]
		if t.FirstGID <= gid && (set == nil || t.FirstGID > set.FirstGID) {
			set = t
		}
	}
	if set == nil {
		return Tile{}, nil, fmt.Errorf("unknown Tiled GID %d", gid)
	}
	id := int(gid - set.FirstGID)
	for _, t := range set.Tiles {
		if t.ID == id {
			if t.Image == "" && set.Image != "" {
				t.Image, t.ImageWidth, t.ImageHeight = set.Image, set.TileWidth, set.TileHeight
			}
			return t, set, nil
		}
	}
	if set.Image != "" && set.Columns > 0 && id < set.TileCount {
		return Tile{ID: id, Image: set.Image, ImageWidth: set.TileWidth, ImageHeight: set.TileHeight}, set, nil
	}
	return Tile{}, nil, fmt.Errorf("missing Tiled tile %d", gid)
}
func (m *parsedMap) placed(gid uint32, x, y, w, h, rotation, opacity float64, layer, objectID int, object bool) (Sprite, error) {
	s, e := m.basePlaced(gid, x, y, w, h, rotation, opacity, layer, objectID, object)
	if e != nil {
		return Sprite{}, e
	}
	t, set, _ := m.tile(gid & 0x0fffffff)
	if len(t.Animation) > MaxAnimationFrames || m.frameCount > MaxTotalAnimationFrames-len(t.Animation) {
		return Sprite{}, fmt.Errorf("Tiled animation exceeds 64 frames")
	}
	m.frameCount += len(t.Animation)
	for _, f := range t.Animation {
		if f.TileID < 0 || f.TileID > 0x0fffffff-int(set.FirstGID) || f.Duration <= 0 || f.Duration > 60000 {
			return Sprite{}, fmt.Errorf("invalid Tiled animation frame")
		}
		frameGID := set.FirstGID + uint32(f.TileID)
		_, frameSet, err := m.tile(frameGID)
		if err != nil || frameSet != set {
			return Sprite{}, fmt.Errorf("animation references missing local tile %d", f.TileID)
		}
		frame, err := m.basePlaced((gid&0xf0000000)|frameGID, x, y, w, h, rotation, opacity, layer, objectID, object)
		if err != nil {
			return Sprite{}, fmt.Errorf("Tiled animation: %w", err)
		}
		s.Frames = append(s.Frames, Frame{Sprite: frame, Duration: f.Duration})
	}
	return s, nil
}
func (m *parsedMap) basePlaced(gid uint32, x, y, w, h, rotation, opacity float64, layer, objectID int, object bool) (Sprite, error) {
	const flags = uint32(0xf0000000)
	t, set, e := m.tile(gid &^ flags)
	if e != nil {
		return Sprite{}, e
	}
	s := Sprite{
		Image:        t.Image,
		SourceWidth:  t.ImageWidth,
		SourceHeight: t.ImageHeight,
		X:            x,
		Y:            y,
		Width:        w,
		Height:       h,
		Rotation:     rotation,
		Opacity:      opacity,
		Layer:        layer,
		ObjectID:     objectID,
		FlipX:        gid&0x80000000 != 0,
		FlipY:        gid&0x40000000 != 0,
		Diagonal:     gid&0x20000000 != 0,
	}
	s.Properties = t.Properties
	if set.Image != "" && t.Image == set.Image {
		s.SourceX = set.Margin + (t.ID%set.Columns)*(set.TileWidth+set.Spacing)
		s.SourceY = set.Margin + (t.ID/set.Columns)*(set.TileHeight+set.Spacing)
	}
	if s.SourceWidth <= 0 || s.SourceHeight <= 0 {
		return Sprite{}, fmt.Errorf("missing tile image dimensions")
	}
	if object {
		if s.Width == 0 {
			s.Width = float64(t.ImageWidth)
		}
		if s.Height == 0 {
			s.Height = float64(t.ImageHeight)
		}
		s.SortY = y
		s.Y -= s.Height
		switch set.ObjectAlignment {
		case "bottom":
			s.X -= s.Width / 2
		case "bottomleft", "", "unspecified":
		default:
			return Sprite{}, fmt.Errorf("unsupported tile object alignment %q", set.ObjectAlignment)
		}
		// Tile offsets are source-image pixels, scaled before object rotation.
		s.X += set.TileOffset.X * s.Width / float64(s.SourceWidth)
		s.Y += set.TileOffset.Y * s.Height / float64(s.SourceHeight)
		// Tiled rotates tile objects around their object alignment point.
		if rotation != 0 {
			a := rotation * math.Pi / 180
			cx, cy := s.X+s.Width/2-x, s.Y+s.Height/2-y
			s.X = x + cx*math.Cos(a) - cy*math.Sin(a) - s.Width/2
			s.Y = y + cx*math.Sin(a) + cy*math.Cos(a) - s.Height/2
		}
	} else {
		if set.TileRenderSize != "grid" {
			s.Width = float64(t.ImageWidth)
			s.Height = float64(t.ImageHeight)
			s.Y += float64(m.TileHeight) - s.Height
		}
		s.X += set.TileOffset.X * s.Width / float64(s.SourceWidth)
		s.Y += set.TileOffset.Y * s.Height / float64(s.SourceHeight)
		s.SortY = s.Y + s.Height
	}
	if !coordinate(s.X) || !coordinate(s.Y) || !coordinate(s.Width) || !coordinate(s.Height) || !coordinate(s.SortY) ||
		!finite(s.Rotation) ||
		!finite(s.Opacity) ||
		s.Opacity < 0 ||
		s.Opacity > 1 ||
		s.Width <= 0 ||
		s.Height <= 0 {
		return Sprite{}, fmt.Errorf("invalid Tiled sprite transform")
	}
	return s, nil
}
func (m *parsedMap) layers(layers []Layer, ox, oy, alpha float64, visible bool, order *int) error {
	for _, l := range layers {
		shown := visible && (l.Visible == nil || *l.Visible)
		a := alpha
		if l.Opacity != nil {
			a *= *l.Opacity
		}
		x, y := ox+l.OffsetX+l.X*float64(m.TileWidth), oy+l.OffsetY+l.Y*float64(m.TileHeight)
		if !coordinate(x) || !coordinate(y) {
			return fmt.Errorf("Tiled group offset exceeds coordinate bounds")
		}
		if l.Type == "group" {
			if e := m.layers(l.Layers, x, y, a, shown, order); e != nil {
				return e
			}
			continue
		}
		layer := *order
		*order++
		m.LayerNames = append(m.LayerNames, l.Name)
		switch l.Type {
		case "tilelayer":
			if !shown {
				continue
			}
			if l.Width <= 0 || l.Height <= 0 || len(l.Data) != l.Width*l.Height {
				return fmt.Errorf("invalid tile layer %q", l.Name)
			}
			for i, gid := range l.Data {
				if gid == 0 {
					continue
				}
				s, e := m.placed(
					gid,
					x+float64(i%l.Width*m.TileWidth),
					y+float64(i/l.Width*m.TileHeight),
					float64(m.TileWidth),
					float64(m.TileHeight),
					0,
					a,
					layer,
					0,
					false,
				)
				if e != nil {
					return e
				}
				if len(m.Sprites) >= MaxSprites {
					return ErrLimit
				}
				m.Sprites = append(m.Sprites, s)
			}
		case "objectgroup":
			for _, o := range l.Objects {
				class := o.Class
				if class == "" {
					class = o.Type
				}
				if o.GID != 0 {
					if !shown || (o.Visible != nil && !*o.Visible) {
						continue
					}
					s, e := m.placed(o.GID, x+o.X, y+o.Y, o.Width, o.Height, o.Rotation, a, layer, o.ID, true)
					if e != nil {
						return e
					}
					s.Properties = append(append(Properties(nil), o.Properties...), s.Properties...)
					if l.DrawOrder == "index" {
						s.SortY = float64(len(m.Sprites))
					}
					if len(m.Sprites) >= MaxSprites {
						return ErrLimit
					}
					m.Sprites = append(m.Sprites, s)
				} else {
					if o.Rotation != 0 || o.Ellipse || len(o.Polyline) != 0 {
						return fmt.Errorf("unsupported geometry in Tiled object %q", o.Name)
					}
					if !coordinate(x+o.X) || !coordinate(y+o.Y) {
						return fmt.Errorf("Tiled object position exceeds coordinate bounds")
					}
					poly := append([]Point(nil), o.Polygon...)
					m.Objects = append(m.Objects, Region{Name: o.Name, Class: class, X: x + o.X, Y: y + o.Y,
						Width: o.Width, Height: o.Height, Polygon: poly, Properties: o.Properties})
				}
			}
		case "imagelayer":
			return fmt.Errorf("image layers are outside the supported Tiled subset")
		default:
			return fmt.Errorf("unsupported Tiled layer %q", l.Type)
		}
	}
	return nil
}
