package source

import (
	"fmt"
	"io"
	"io/fs"
	"math"

	"github.com/karty-game/tiled"
)

const MaxDocumentBytes = tiled.MaxDocumentBytes
const MaxTotalDocumentBytes = tiled.MaxTotalDocumentBytes
const MaxDocuments = tiled.MaxDocuments
const MaxMapDimension = tiled.MaxMapDimension
const MaxLayers = tiled.MaxLayers
const MaxLayerDepth = tiled.MaxLayerDepth
const MaxSprites = tiled.MaxSprites
const MaxObjects = tiled.MaxObjects
const MaxAnimationFrames = tiled.MaxAnimationFrames
const MaxTotalAnimationFrames = tiled.MaxTotalAnimationFrames
const MaxImageDimension = tiled.MaxImageDimension
const MaxCoordinate = tiled.MaxCoordinate

var ErrLimit = tiled.ErrLimit

type loadLimits struct{ documents, bytes, layers, objects, points int }

func (limits *loadLimits) read(fsys fs.FS, name string, value any) error {
	if limits.documents >= MaxDocuments {
		return ErrLimit
	}
	f, err := fsys.Open(name)
	if err != nil {
		return err
	}
	b, err := io.ReadAll(io.LimitReader(f, MaxDocumentBytes+1))
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if len(b) > MaxDocumentBytes || limits.bytes > MaxTotalDocumentBytes-len(b) {
		return fmt.Errorf("document %q: %w", name, ErrLimit)
	}
	limits.documents++
	limits.bytes += len(b)
	if !boundedJSONDepth(b) {
		return fmt.Errorf("document %q nesting: %w", name, ErrLimit)
	}
	return decodeDocument(b, value)
}

// Reject excessive JSON nesting before allocating recursive Tiled structures.
func boundedJSONDepth(b []byte) bool {
	depth := 0
	quoted, escaped := false, false
	for _, c := range b {
		if quoted {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				quoted = false
			}
			continue
		}
		if c == '"' {
			quoted = true
			continue
		}
		if c == '{' || c == '[' {
			depth++
			if depth > 64 {
				return false
			}
		}
		if c == '}' || c == ']' {
			depth--
		}
	}
	return true
}

func finite(v float64) bool     { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func coordinate(v float64) bool { return finite(v) && math.Abs(v) <= MaxCoordinate }

func (limits *loadLimits) validateLayers(layers []Layer, depth int) error {
	if depth > MaxLayerDepth {
		return ErrLimit
	}
	for _, l := range layers {
		limits.layers++
		limits.objects += len(l.Objects)
		if limits.layers > MaxLayers || limits.objects > MaxObjects {
			return ErrLimit
		}
		if !coordinate(l.X) || !coordinate(l.Y) || !coordinate(l.OffsetX) || !coordinate(l.OffsetY) ||
			l.Opacity != nil && (!finite(*l.Opacity) || *l.Opacity < 0 || *l.Opacity > 1) {
			return fmt.Errorf("invalid Tiled layer transform %q", l.Name)
		}
		switch l.Type {
		case "group":
			if err := limits.validateLayers(l.Layers, depth+1); err != nil {
				return err
			}
		case "tilelayer":
			if l.Width <= 0 || l.Height <= 0 || l.Width > MaxMapDimension || l.Height > MaxMapDimension ||
				len(l.Data) != l.Width*l.Height ||
				l.Encoding != "" ||
				l.Compression != "" ||
				len(l.Chunks) != 0 {
				return fmt.Errorf("unsupported or incomplete Tiled tile layer %q", l.Name)
			}
		case "objectgroup":
			if l.DrawOrder != "" && l.DrawOrder != "topdown" && l.DrawOrder != "index" {
				return fmt.Errorf("unsupported object draw order %q", l.DrawOrder)
			}
		default:
			return fmt.Errorf("unsupported Tiled layer %q", l.Type)
		}
		for _, o := range l.Objects {
			if o.Template != "" {
				return fmt.Errorf("external Tiled object templates are unsupported")
			}
			if !coordinate(o.X) || !coordinate(o.Y) || !coordinate(o.Width) || !coordinate(o.Height) || !finite(o.Rotation) ||
				o.Width < 0 ||
				o.Height < 0 {
				return fmt.Errorf("invalid Tiled object transform %q", o.Name)
			}
			limits.points += len(o.Polygon) + len(o.Polyline)
			if limits.points > MaxObjects {
				return ErrLimit
			}
			for _, p := range o.Polygon {
				if !coordinate(p.X) || !coordinate(p.Y) {
					return fmt.Errorf("invalid Tiled polygon point")
				}
			}
		}
	}
	return nil
}

func validateTileset(t *Tileset) error {
	if t.FirstGID == 0 || t.FirstGID > 0x0fffffff || t.TileCount < 0 || t.TileCount > MaxSprites || len(t.Tiles) > MaxSprites ||
		t.TileWidth < 0 ||
		t.TileHeight < 0 ||
		t.TileWidth > MaxImageDimension ||
		t.TileHeight > MaxImageDimension ||
		!coordinate(t.TileOffset.X) ||
		!coordinate(t.TileOffset.Y) {
		return fmt.Errorf("invalid Tiled tileset bounds")
	}
	if t.Image != "" &&
		(t.Columns <= 0 || t.Columns > MaxSprites || t.TileCount <= 0 || t.TileWidth <= 0 || t.TileHeight <= 0 || t.Margin < 0 || t.Spacing < 0 || t.Margin > MaxImageDimension || t.Spacing > MaxImageDimension || t.ImageWidth <= 0 || t.ImageHeight <= 0 || t.ImageWidth > MaxImageDimension || t.ImageHeight > MaxImageDimension) {
		return fmt.Errorf("invalid Tiled atlas metadata")
	}
	if t.TileRenderSize != "" && t.TileRenderSize != "tile" && t.TileRenderSize != "grid" {
		return fmt.Errorf("unsupported Tiled tile render size %q", t.TileRenderSize)
	}
	if t.ObjectAlignment != "" && t.ObjectAlignment != "unspecified" && t.ObjectAlignment != "bottomleft" && t.ObjectAlignment != "bottom" {
		return fmt.Errorf("unsupported Tiled object alignment %q", t.ObjectAlignment)
	}
	if t.Image != "" {
		id := t.TileCount - 1
		x := t.Margin + min(id, t.Columns-1)*(t.TileWidth+t.Spacing)
		y := t.Margin + (id/t.Columns)*(t.TileHeight+t.Spacing)
		if x > t.ImageWidth-t.TileWidth || y > t.ImageHeight-t.TileHeight {
			return fmt.Errorf("Tiled atlas cells exceed declared image bounds")
		}
	}
	ids := make(map[int]bool, len(t.Tiles))
	for _, tile := range t.Tiles {
		if tile.ID < 0 || tile.ID >= MaxSprites || ids[tile.ID] || uint64(t.FirstGID)+uint64(tile.ID) > 0x0fffffff {
			return fmt.Errorf("invalid or duplicate Tiled tile ID")
		}
		ids[tile.ID] = true
		if tile.Image != "" &&
			(tile.ImageWidth <= 0 || tile.ImageHeight <= 0 || tile.ImageWidth > MaxImageDimension || tile.ImageHeight > MaxImageDimension) {
			return fmt.Errorf("invalid Tiled tile image dimensions")
		}
		if t.Image != "" && tile.ID >= t.TileCount {
			return fmt.Errorf("Tiled atlas tile ID exceeds tile count")
		}
		if len(tile.Animation) > MaxAnimationFrames {
			return ErrLimit
		}
	}
	for _, tile := range t.Tiles {
		for _, frame := range tile.Animation {
			present := ids[frame.TileID] || t.Image != "" && frame.TileID >= 0 && frame.TileID < t.TileCount
			if !present || frame.Duration <= 0 || frame.Duration > 60000 {
				return fmt.Errorf("invalid Tiled animation frame")
			}
		}
	}
	return nil
}

func tilesetEnd(t *Tileset) uint64 {
	span := max(1, t.TileCount)
	for _, tile := range t.Tiles {
		span = max(span, tile.ID+1)
	}
	return uint64(t.FirstGID) + uint64(span) - 1
}
