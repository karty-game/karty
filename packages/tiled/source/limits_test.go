package source

import (
	"encoding/json"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/karty-game/tiled"
)

func TestParserRejectsUnsafeAtlasAndUnsupportedSourceFeatures(t *testing.T) {
	for _, set := range []string{
		`{"image":"atlas.png","tilewidth":32,"tileheight":32,"columns":0,"tilecount":1,"imagewidth":32,"imageheight":32}`,
		`{"image":"atlas.png","tilewidth":32,"tileheight":32,"columns":1,"tilecount":2,"imagewidth":32,"imageheight":32}`,
		`{"tiles":[{"id":0,"image":"x.png","imagewidth":32,"imageheight":32},{"id":0,"image":"y.png","imagewidth":32,"imageheight":32}]}`,
		`{"tiles":[{"id":-1,"image":"x.png","imagewidth":32,"imageheight":32}]}`,
		`{"tiles":[{"id":0,"image":"x.png","imagewidth":999999999,"imageheight":32}]}`,
		`{"source":"another.tsj"}`,
	} {
		if _, err := Load(fixture(`[{"type":"tilelayer","width":2,"height":1,"data":[1,0]}]`, set), "maps/village.tmj"); err == nil {
			t.Fatal("accepted unsafe atlas/source metadata", set)
		}
	}
	for _, layer := range []string{
		`[{"type":"tilelayer","width":2147483647,"height":2147483647,"data":[]}]`,
		`[{"type":"tilelayer","width":2,"height":1,"data":[1,0],"compression":"zlib"}]`,
		`[{"type":"tilelayer","width":2,"height":1,"data":[1,0],"opacity":2}]`,
		`[{"type":"objectgroup","objects":[{"template":"thing.tx"}]}]`,
		`[{"type":"imagelayer"}]`,
	} {
		if _, err := Load(fixture(layer, collection), "maps/village.tmj"); err == nil {
			t.Fatal("accepted unsupported or unbounded layer", layer)
		}
	}
}

func TestParserBoundsActualReadsAndNestingBeforeDecode(t *testing.T) {
	input := &countedInput{remaining: MaxDocumentBytes * 2}
	_, err := Load(input, "map.tmj")
	if !errors.Is(err, ErrLimit) || input.read > MaxDocumentBytes+1 || !input.closed {
		t.Fatal("input read was not bounded and closed", err, input.read, input.closed)
	}
	layers := `[{"type":"objectgroup"}]`
	for range MaxLayerDepth {
		layers = `[{"type":"group","layers":` + layers + `}]`
	}
	if _, err := Load(fixture(layers, collection), "maps/village.tmj"); !errors.Is(err, ErrLimit) {
		t.Fatal("excessive layer nesting accepted", err)
	}
	if boundedJSONDepth([]byte(strings.Repeat("[", 65) + strings.Repeat("]", 65))) {
		t.Fatal("excessive JSON nesting accepted")
	}
	if !boundedJSONDepth([]byte(`{"text":"[[[[[[[\\\"[[["}`)) {
		t.Fatal("JSON string content treated as nesting")
	}
}

func TestParserConfinesPathsAndAggregateDocumentBudget(t *testing.T) {
	for _, path := range []string{"../map.tmj", "/map.tmj", `C:\map.tmj`, "a\x00.tmj"} {
		if _, err := Load(fstest.MapFS{}, path); err == nil {
			t.Fatal("accepted nonportable or escaping path", path)
		}
	}
	limits := loadLimits{documents: MaxDocuments}
	if err := limits.read(fstest.MapFS{}, "map.tmj", new(Map)); !errors.Is(err, ErrLimit) {
		t.Fatal("document count limit", err)
	}
	limits = loadLimits{bytes: MaxTotalDocumentBytes}
	if err := limits.read(fstest.MapFS{"x": {Data: []byte(`{}`)}}, "x", new(Map)); !errors.Is(err, ErrLimit) {
		t.Fatal("aggregate document bytes limit", err)
	}
	if tiled.FrameAt([]Frame{{Duration: 0}}, 1) != -1 || tiled.FrameAt(make([]Frame, MaxAnimationFrames+1), 1) != -1 {
		t.Fatal("invalid manually supplied animation accepted")
	}
}

type countedInput struct {
	remaining, read int
	closed          bool
}

func (f *countedInput) Open(string) (fs.File, error) { return f, nil }
func (f *countedInput) Read(p []byte) (int, error) {
	n := min(len(p), f.remaining)
	for i := range n {
		p[i] = ' '
	}
	f.remaining -= n
	f.read += n
	return n, nil
}
func (f *countedInput) Stat() (fs.FileInfo, error) { return nil, fs.ErrInvalid }
func (f *countedInput) Close() error               { f.closed = true; return nil }

func TestParserRejectsTotalSpriteAndLayerGrowth(t *testing.T) {
	// Many small layers must not evade the flattened sprite/layer bounds.
	layer := map[string]any{"type": "tilelayer", "width": 1, "height": 1, "data": []uint32{1}}
	layers := make([]any, MaxLayers+1)
	for i := range layers {
		layers[i] = layer
	}
	data, err := json.Marshal(layers)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Load(fixture(string(data), collection), "maps/village.tmj"); !errors.Is(err, ErrLimit) {
		t.Fatal("layer growth was not bounded", err)
	}
	cells := make([]uint32, MaxSprites)
	for i := range cells {
		cells[i] = 1
	}
	layers = []any{
		map[string]any{"type": "tilelayer", "width": 256, "height": 256, "data": cells},
		layer,
	}
	data, err = json.Marshal(layers)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Load(fixture(string(data), collection), "maps/village.tmj"); !errors.Is(err, ErrLimit) {
		t.Fatal("flattened sprite growth was not bounded", err)
	}
}
