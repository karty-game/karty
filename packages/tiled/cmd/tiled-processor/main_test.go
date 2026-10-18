package main

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/karty-game/tiled"
)

func request(files memoryFS) []byte {
	out := []byte{'K', 'P', 'R', '1', 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(out[4:], uint32(len(files)))
	for name, data := range files {
		out = binary.LittleEndian.AppendUint16(out, uint16(len(name)))
		out = binary.LittleEndian.AppendUint32(out, uint32(len(data)))
		out = append(out, name...)
		out = append(out, data...)
	}
	return out
}
func TestProcessorCompilesMetadataAndPublishesOnlyCompleteOutput(t *testing.T) {
	files := memoryFS{"map.tmj": []byte(`{"orientation":"orthogonal","width":1,"height":1,"tilewidth":16,"tileheight":16,"layers":[]}`)}
	var output bytes.Buffer
	if err := run(bytes.NewReader(request(files)), &output); err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeRequest(output.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	m, err := tiled.Decode(decoded["map.bin"])
	if err != nil || m.Width != 1 {
		t.Fatal(m, err)
	}
	files["z-bad.tmj"] = []byte("{")
	output.Reset()
	if err := run(bytes.NewReader(request(files)), &output); err == nil || output.Len() != 0 {
		t.Fatal("partial compilation was published")
	}
}
func TestProcessorRejectsPathsDuplicatesAndTrailingBytes(t *testing.T) {
	for _, name := range []string{"../escape.tmj", "/absolute.tmj", "x\\y.tmj", "C:map.tmj"} {
		if _, err := decodeRequest(request(memoryFS{name: []byte("{}")})); err == nil {
			t.Fatal("accepted path", name)
		}
	}
	good := request(memoryFS{"map.tmj": []byte("{}")})
	for i := 0; i < len(good); i++ {
		if _, err := decodeRequest(good[:i]); err == nil {
			t.Fatal("accepted partial request", i)
		}
	}
	if _, err := decodeRequest(append(good, 0)); err == nil {
		t.Fatal("accepted trailing input")
	}
	duplicate := append(append([]byte{}, good...), good[8:]...)
	binary.LittleEndian.PutUint32(duplicate[4:], 2)
	if _, err := decodeRequest(duplicate); err == nil {
		t.Fatal("accepted duplicate input paths")
	}
}
