package source

import (
	"fmt"
	"math"
	"testing"
	"testing/fstest"

	"github.com/karty-game/tiled"
)

func fixture(layers, set string) fstest.MapFS {
	return fstest.MapFS{
		"maps/village.tmj": {
			Data: []byte(
				fmt.Sprintf(
					`{"orientation":"orthogonal","width":2,"height":1,"tilewidth":32,"tileheight":32,"tilesets":[{"firstgid":1,"source":"../art/tiles.tsj"}],"layers":%s}`,
					layers,
				),
			),
		},
		"art/tiles.tsj": {Data: []byte(set)},
	}
}

const collection = `{"tilewidth":32,"tileheight":32,"objectalignment":"bottom","tilerendersize":"grid","tiles":[{"id":0,"image":"grass.png","imagewidth":128,"imageheight":128},{"id":1,"image":"house.png","imagewidth":256,"imageheight":224}]}`

func TestImageCollectionLayersAndAuthoredObjects(t *testing.T) {
	f := fixture(
		`[{"name":"Ground","type":"tilelayer","width":2,"height":1,"data":[1,0]},{"name":"Scenery","type":"objectgroup","offsetx":4,"offsety":8,"opacity":0.5,"objects":[{"id":7,"gid":2,"x":100,"y":200,"width":128,"height":112}]},{"name":"Collisions","type":"objectgroup","visible":false,"objects":[{"name":"bank","class":"collision","x":20,"y":30,"width":32,"height":64}]}]`,
		collection,
	)
	m, e := Load(f, "maps/village.tmj")
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Sprites) != 2 || len(m.Objects) != 1 {
		t.Fatalf("lost editor layers: %+v", m)
	}
	g, h := m.Sprites[0], m.Sprites[1]
	if g.Image != "art/grass.png" || g.Width != 32 || g.Height != 32 || g.SourceWidth != 128 {
		t.Fatalf("grid scaling: %+v", g)
	}
	if h.X != 40 || h.Y != 96 || h.SortY != 208 || h.Opacity != .5 || h.ObjectID != 7 {
		t.Fatalf("authored foot alignment: %+v", h)
	}
	if m.Objects[0].Class != "collision" || m.Objects[0].X != 20 {
		t.Fatal("hidden editor collision metadata was discarded")
	}
}

func TestAtlasRectanglesAndAllGIDFlags(t *testing.T) {
	set := `{"tilewidth":16,"tileheight":24,"columns":2,"tilecount":4,"margin":2,"spacing":1,"image":"atlas.png","imagewidth":37,"imageheight":53}`
	for flags := uint32(0); flags < 16; flags++ {
		gid := uint32(4) | flags<<28
		f := fixture(fmt.Sprintf(`[{"type":"tilelayer","width":2,"height":1,"data":[%d,0]}]`, gid), set)
		m, e := Load(f, "maps/village.tmj")
		if e != nil {
			t.Fatal(e)
		}
		s := m.Sprites[0]
		if s.SourceX != 19 || s.SourceY != 27 || s.SourceWidth != 16 || s.SourceHeight != 24 {
			t.Fatalf("tileset metadata rectangle: %+v", s)
		}
		if s.FlipX != (flags&8 != 0) || s.FlipY != (flags&4 != 0) || s.Diagonal != (flags&2 != 0) {
			t.Fatalf("GID flags %x: %+v", flags, s)
		}
	}
}

func TestGroupOffsetsVisibilityAndObjectRotation(t *testing.T) {
	f := fixture(
		`[{"type":"group","offsetx":10,"offsety":20,"layers":[{"name":"Scenery","type":"objectgroup","objects":[{"gid":2,"x":100,"y":200,"width":128,"height":112,"rotation":90}]}]},{"type":"tilelayer","visible":false,"width":2,"height":1,"data":[1,1]}]`,
		collection,
	)
	m, e := Load(f, "maps/village.tmj")
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Sprites) != 1 {
		t.Fatal("hidden tiles rendered")
	}
	s := m.Sprites[0]
	if math.Abs(s.X-102) > 1e-8 || math.Abs(s.Y-164) > 1e-8 {
		t.Fatalf("rotation around authored feet: %+v", s)
	}
}

func TestMalformedOrEscapingResourcesAreRejected(t *testing.T) {
	for _, set := range []string{`{"tiles":[{"id":0,"image":"../../outside.png","imagewidth":32,"imageheight":32}]}`, `{"tiles":[{"id":0,"image":"https://example.com/x.png","imagewidth":32,"imageheight":32}]}`, `{"tiles":[]}`} {
		if _, e := Load(fixture(`[{"type":"tilelayer","width":2,"height":1,"data":[1,0]}]`, set), "maps/village.tmj"); e == nil {
			t.Fatal("invalid tileset accepted")
		}
	}
	if _, e := Load(fixture(`[{"type":"tilelayer","width":2,"height":1,"data":[1]}]`, collection), "maps/village.tmj"); e == nil {
		t.Fatal("incomplete tile layer accepted")
	}
}

// Source corners TL, TR, BL, BR mapped to destination corner indices. This
// fixed table describes Tiled's diagonal-first GID operation independently of
// the centered-sprite rotation/scale decomposition used by Transform.
func TestFlipTransformsMatchTiledCornerOrderForNonSquareImages(t *testing.T) {
	cornerOrder := [8][4]int{{0, 1, 2, 3}, {1, 0, 3, 2}, {2, 3, 0, 1}, {3, 2, 1, 0}, {0, 2, 1, 3}, {1, 3, 0, 2}, {2, 0, 3, 1}, {3, 1, 2, 0}}
	for flags := range 8 {
		for _, angle := range []float64{0, 30, -90} {
			s := Sprite{X: 100, Y: 200, SourceWidth: 16, SourceHeight: 24, Width: 40, Height: 90, Rotation: angle,
				FlipX: flags&1 != 0, FlipY: flags&2 != 0, Diagonal: flags&4 != 0}
			sx, sy, a := tiled.Transform(s)
			cx, cy := tiled.Center(s)
			left, top, width, height := 100.0, 200.0, 40.0, 90.0
			if s.Diagonal {
				top, width, height = 250, 90, 40
			}
			destination := [4]Point{
				{X: left, Y: top},
				{X: left + width, Y: top},
				{X: left, Y: top + height},
				{X: left + width, Y: top + height},
			}
			for i, p := range []Point{{X: -8, Y: -12}, {X: 8, Y: -12}, {X: -8, Y: 12}, {X: 8, Y: 12}} {
				want := destination[cornerOrder[flags][i]]
				// The parser has already rotated the nominal center for objects;
				// rotate the remaining corner displacement around that center.
				dx, dy := want.X-120, want.Y-245
				b := angle * math.Pi / 180
				want.X, want.Y = 120+dx*math.Cos(b)-dy*math.Sin(b), 245+dx*math.Sin(b)+dy*math.Cos(b)
				actualX := cx + p.X*sx*math.Cos(a) - p.Y*sy*math.Sin(a)
				actualY := cy + p.X*sx*math.Sin(a) + p.Y*sy*math.Cos(a)
				if math.Abs(actualX-want.X) > 1e-8 || math.Abs(actualY-want.Y) > 1e-8 {
					t.Fatalf("flags %d angle %g corner %d: got %g,%g want %+v", flags, angle, i, actualX, actualY, want)
				}
			}
		}
	}
}

func TestScaledTileOffsetsFollowObjectAlignmentAndRotation(t *testing.T) {
	for _, alignment := range []string{"bottom", "bottomleft"} {
		for _, diagonal := range []bool{false, true} {
			gid := uint32(1)
			if diagonal {
				gid |= 0x20000000
			}
			set := fmt.Sprintf(
				`{"tilewidth":16,"tileheight":24,"objectalignment":%q,"tileoffset":{"x":3,"y":-2},"tiles":[{"id":0,"image":"plant.png","imagewidth":16,"imageheight":24}]}`,
				alignment,
			)
			layers := fmt.Sprintf(
				`[{"type":"objectgroup","objects":[{"gid":%d,"x":100,"y":200,"width":40,"height":90,"rotation":90}]}]`,
				gid,
			)
			m, err := Load(fixture(layers, set), "maps/village.tmj")
			if err != nil {
				t.Fatal(err)
			}
			cx, cy := tiled.Center(m.Sprites[0])
			// Source offset (3,-2) scales to (7.5,-7.5). Nominal local
			// center is (20,-45), or (0,-45) for bottom alignment.
			localX, localY := 27.5, -52.5
			if alignment == "bottom" {
				localX -= 20
			}
			if diagonal {
				localX += 25
				localY += 25
			}
			if math.Abs(cx-(100-localY)) > 1e-8 || math.Abs(cy-(200+localX)) > 1e-8 {
				t.Fatalf("%s diagonal=%v center=(%g,%g)", alignment, diagonal, cx, cy)
			}
		}
	}
}

func TestGridSizedTileOffsetsScaleToGridDimensions(t *testing.T) {
	set := `{"tilewidth":16,"tileheight":24,"tilerendersize":"grid","tileoffset":{"x":3,"y":-2},"tiles":[{"id":0,"image":"plant.png","imagewidth":16,"imageheight":24}]}`
	m, err := Load(fixture(`[{"type":"tilelayer","width":2,"height":1,"data":[1,0]}]`, set), "maps/village.tmj")
	if err != nil {
		t.Fatal(err)
	}
	s := m.Sprites[0]
	if math.Abs(s.X-6) > 1e-8 || math.Abs(s.Y+8.0/3) > 1e-8 {
		t.Fatalf("source offsets were not scaled to 32x32 grid: %+v", s)
	}
}
