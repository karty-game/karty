package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"reflect"
	"testing"

	"github.com/karty-game/tiled"
)

func TestProcessorBuildsChunksWithoutChangingAuthoredMapOrDynamicObjects(t *testing.T) {
	pixels := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	pixels.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	pixels.SetNRGBA(1, 0, color.NRGBA{G: 255, A: 255})
	pixels.SetNRGBA(0, 1, color.NRGBA{B: 255, A: 255})
	pixels.SetNRGBA(1, 1, color.NRGBA{R: 255, G: 255, A: 255})
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, pixels); err != nil {
		t.Fatal(err)
	}
	authored := []byte(
		`{"orientation":"orthogonal","width":2,"height":2,"tilewidth":2,"tileheight":2,"properties":[{"name":"intro","type":"string","value":"keep this"}],"tilesets":[{"firstgid":1,"tilewidth":2,"tileheight":2,"tilecount":1,"columns":1,"image":"tile.png","imagewidth":2,"imageheight":2}],"layers":[{"name":"Ground","type":"tilelayer","width":2,"height":2,"data":[1,2147483649,1,1]},{"name":"Scenery","type":"objectgroup","objects":[{"id":7,"name":"prop","gid":1,"x":1,"y":3,"width":2,"height":2,"rotation":13,"visible":true}]},{"name":"Gameplay","type":"objectgroup","visible":false,"objects":[{"id":8,"name":"clue","class":"clue","point":true,"x":1,"y":3,"properties":[{"name":"message","type":"string","value":"keep clue"}]}]}]}`,
	)
	files := memoryFS{
		"map.tmj":  authored,
		"tile.png": imageData.Bytes(),
		"tiled-build.json": []byte(
			`{"version":1,"map":"map.tmj","level":"village","staticLayers":["Ground"],"chunkSize":4,"pixelScale":1}`,
		),
	}
	var output bytes.Buffer
	if err := run(bytes.NewReader(request(files)), &output); err != nil {
		t.Fatal(err)
	}
	compiled, err := decodeRequest(output.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	original, err := tiled.Decode(compiled["map.bin"])
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := tiled.Decode(compiled["map.render.bin"])
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original.Objects, rendered.Objects) || !reflect.DeepEqual(original.Properties, rendered.Properties) ||
		!reflect.DeepEqual(original.LayerNames, rendered.LayerNames) ||
		!bytes.Equal(files["map.tmj"], authored) {
		t.Fatal("chunking changed authored gameplay metadata")
	}
	if len(rendered.Sprites) != 2 || !reflect.DeepEqual(rendered.Sprites[0], original.Sprites[4]) {
		t.Fatal("dynamic scenery/transform changed", rendered.Sprites)
	}
	var catalog assetCatalog
	if err := json.Unmarshal(compiled["assets.json"], &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Textures) != 1 || catalog.Textures[0].Key != tiled.ImageKey(rendered.Sprites[1]) {
		t.Fatal("generated texture cannot resolve compiled chunk", catalog)
	}
	generated, err := png.Decode(bytes.NewReader(compiled[catalog.Textures[0].Source]))
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			sourceX := x % 2
			if x >= 2 {
				sourceX = 1 - sourceX
			}
			if color.NRGBAModel.Convert(generated.At(x, y)) != color.NRGBAModel.Convert(pixels.At(sourceX, y)) {
				t.Fatal("chunk lost original resolution/flip pixels", x, y)
			}
		}
	}
	var repeated bytes.Buffer
	if err := run(bytes.NewReader(request(files)), &repeated); err != nil || !bytes.Equal(output.Bytes(), repeated.Bytes()) {
		t.Fatal("processor output not deterministic", err)
	}
	files["tile.png"] = []byte("broken")
	repeated.Reset()
	if err := run(bytes.NewReader(request(files)), &repeated); err == nil || repeated.Len() != 0 {
		t.Fatal("failed chunk build published partial output", err)
	}
}
