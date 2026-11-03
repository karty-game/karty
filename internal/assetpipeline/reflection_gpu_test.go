package assetpipeline

import (
	"image"
	"image/color"
	"os"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/worldreflection"
	"github.com/karty-game/karty/internal/sdk"
)

func reflectionCaptureFixture(t *testing.T) worldreflection.Bundle {
	t.Helper()

	pixels := image.NewNRGBA(image.Rect(0, 0, worldreflection.FaceSize*worldreflection.Faces, worldreflection.FaceSize))

	colors := [6]color.NRGBA{{0, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}, {128, 0, 0, 255}, {80, 80, 80, 255}, {255, 255, 255, 255}}
	for face, value := range colors {
		for row := range worldreflection.FaceSize {
			for column := range worldreflection.FaceSize {
				pixel := value
				if face == 0 && (row+column)%2 == 0 {
					pixel = colors[5]
				}

				pixels.SetNRGBA(face*worldreflection.FaceSize+column, row, pixel)
			}
		}
	}

	encoded, _, err := qoi.Encode(pixels, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceSRGB})
	if err != nil {
		t.Fatal(err)
	}

	return worldreflection.Bundle{Producer: worldreflection.Producer, InputSHA256: strings.Repeat("0", 64), Sectors: []int{0},
		Probes: []worldreflection.Probe{{ID: 0, Origin: [3]float32{1, 1, 1}, Low: [3]float32{}, High: [3]float32{2, 2, 2}, Image: encoded}}}
}

func TestReflectionMipsFilterLinearRGBWithinEachFace(t *testing.T) {
	t.Parallel()
	capture := reflectionCaptureFixture(t)

	request, err := prepareReflectionMips(t.Context(), capture)
	if err != nil {
		t.Fatal(err)
	}

	var bytes int
	for level, pixels := range request.Levels {
		bytes += len(pixels)

		if level == 0 {
			continue
		}

		size := worldreflection.FaceSize >> level
		for row := range size {
			for column := range size {
				first := (row*size*6 + column) * 4
				second := (row*size*6 + size + column) * 4

				if [4]byte(pixels[first:first+4]) != [4]byte{188, 188, 188, 255} {
					t.Fatal("checker was averaged in encoded sRGB rather than linear light", level, pixels[first:first+4])
				}

				if [4]byte(pixels[second:second+4]) != [4]byte{0, 255, 0, 255} {
					t.Fatal("neighbor face contaminated independent mip filtering", level, pixels[second:second+4])
				}
			}
		}
	}

	if bytes != worldreflection.GPUImageBytes {
		t.Fatal("mip memory accounting changed", bytes)
	}
}

func TestReflectionGPUActualEncoderKeepsOneArrayAndWarmCache(t *testing.T) {
	encoder := os.Getenv("KARTY_TEST_KTX")
	if encoder == "" {
		t.Skip("explicit official encoder not selected")
	}

	t.Setenv("KARTY_KTX", encoder)
	capture := reflectionCaptureFixture(t)
	root := t.TempDir()

	prepared, first, err := ProcessReflectionGPU(t.Context(), root, sdk.Manifest{}, capture)
	if err != nil || first.CacheHit {
		t.Fatal("cold prepared reflection", err)
	}

	if len(prepared.Metadata.Probes[0].Image) != 0 || len(capture.Probes[0].Image) == 0 {
		t.Fatal("prepared array duplicated QOI or mutated its offline capture")
	}

	_, warm, err := ProcessReflectionGPU(t.Context(), root, sdk.Manifest{}, capture)
	if err != nil || !warm.CacheHit || warm.CacheKey != first.CacheKey {
		t.Fatal("warm reflection array cache", err)
	}
}
