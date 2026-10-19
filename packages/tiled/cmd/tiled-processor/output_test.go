package main

import (
	"errors"
	"image"
	"image/png"
	"testing"
)

func TestGeneratedPNGEncodingStopsAtRemainingBudget(t *testing.T) {
	writer := boundedOutput{limit: 32}
	if err := png.Encode(&writer, image.NewNRGBA(image.Rect(0, 0, 8, 8))); !errors.Is(err, errOutputLimit) {
		t.Fatal("PNG encoder ignored output budget", err)
	}
	if writer.Len() > writer.limit {
		t.Fatal("PNG encoder retained bytes beyond its budget")
	}
	before := writer.Len()
	if _, err := writer.Write(make([]byte, 33)); !errors.Is(err, errOutputLimit) || writer.Len() != before {
		t.Fatal("rejected write partially mutated output", err)
	}
	files := memoryFS{"assets.json": []byte("before")}
	if err := setOutput(files, "assets.json", []byte("after")); err != nil {
		t.Fatal("catalog replacement rejected", err)
	}
	if err := setOutput(files, "map.bin", []byte("map")); err != nil {
		t.Fatal(err)
	}
	if err := setOutput(files, "map.bin", []byte("duplicate")); !errors.Is(err, errOutputLimit) {
		t.Fatal("duplicate generated map overwritten", err)
	}
}
