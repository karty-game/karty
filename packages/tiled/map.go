// Package tiled decodes compiled Tiled maps for engine-independent game code.
package tiled

type Property struct {
	Name, Type string
	Value      any
}
type Properties []Property

func (p Properties) Number(name string, fallback float64) float64 {
	for _, v := range p {
		if v.Name == name {
			if n, ok := v.Value.(float64); ok {
				return n
			}
		}
	}
	return fallback
}
func (p Properties) String(name, fallback string) string {
	for _, v := range p {
		if v.Name == name {
			if s, ok := v.Value.(string); ok {
				return s
			}
		}
	}
	return fallback
}

type Object struct {
	ID                            int
	Name, Class, Type             string
	Template                      string
	X, Y, Width, Height, Rotation float64
	GID                           uint32
	Point, Ellipse                bool
	Visible                       *bool
	Polygon, Polyline             []Point
	Properties                    Properties
}
type Point struct{ X, Y float64 }
type Layer struct {
	Name, Type, DrawOrder  string
	Encoding, Compression  string
	Chunks                 []any
	Width, Height          int
	X, Y, OffsetX, OffsetY float64
	Opacity                *float64
	Visible                *bool
	Data                   []uint32
	Objects                []Object
	Layers                 []Layer
	Properties             Properties
}
type Tile struct {
	ID                      int
	Image                   string
	ImageWidth, ImageHeight int
	ObjectGroup             *Layer
	Properties              Properties
	Animation               []AnimationFrame
}

// AnimationFrame uses Tiled's tileset-local ID and duration in milliseconds.
type AnimationFrame struct {
	TileID, Duration int
}
type Tileset struct {
	FirstGID                                                   uint32
	Source, Name, ObjectAlignment, TileRenderSize              string
	TileWidth, TileHeight, TileCount, Columns, Margin, Spacing int
	Image                                                      string
	ImageWidth, ImageHeight                                    int
	TileOffset                                                 Point
	Tiles                                                      []Tile
}
type Document struct {
	Orientation, RenderOrder             string
	Width, Height, TileWidth, TileHeight int
	Infinite                             bool
	Layers                               []Layer
	Tilesets                             []Tileset
	Properties                           Properties
}

// Sprite describes a placed 2D image. Source rectangles come from tileset
// metadata rather than UV coordinates authored in game code. X/Y and Width/Height
// describe the nominal rectangle; use Center and Transform together to account
// for diagonal flips of non-square images.
type Sprite struct {
	Image                                       string
	SourceX, SourceY, SourceWidth, SourceHeight int
	X, Y, Width, Height, Rotation               float64
	FlipX, FlipY, Diagonal                      bool
	Opacity                                     float64
	Layer                                       int
	SortY                                       float64
	ObjectID                                    int
	Properties                                  Properties
	Frames                                      []Frame
}
type Frame struct {
	Sprite   Sprite
	Duration int
}
type Region struct {
	Name, Class         string
	X, Y, Width, Height float64
	Polygon             []Point
	Properties          Properties
}
type Map struct {
	Document
	Sprites    []Sprite
	Objects    []Region
	LayerNames []string
}
