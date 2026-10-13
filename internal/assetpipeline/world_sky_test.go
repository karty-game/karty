package assetpipeline

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io"
	"testing"
)

func TestWorldSkyPNGConversionPreservesLinearPremultipliedAlpha(t *testing.T) {
	t.Parallel()

	image := image.NewNRGBA(image.Rect(0, 0, 3, 1))
	image.SetNRGBA(0, 0, color.NRGBA{R: 128, G: 255, B: 0, A: 128})
	image.SetNRGBA(1, 0, color.NRGBA{R: 255, G: 255, B: 255, A: 0})
	image.SetNRGBA(2, 0, color.NRGBA{R: 255, G: 128, B: 0, A: 255})

	var input bytes.Buffer
	if err := png.Encode(&input, image); err != nil {
		t.Fatal(err)
	}

	first, err := CompileWorldSkyImage(input.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	second, err := CompileWorldSkyImage(input.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	if first.Width != 3 || first.Height != 1 || !bytes.Equal(first.Packed, second.Packed) {
		t.Fatal("dimensions changed or gzip is nondeterministic")
	}

	reader, err := gzip.NewReader(bytes.NewReader(first.Packed))
	if err != nil {
		t.Fatal(err)
	}

	defer reader.Close()

	actual, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	expected := []byte{28, 128, 0, 128, 0, 0, 0, 0, 255, 55, 0, 255}
	if !bytes.Equal(actual, expected) || !reader.ModTime.IsZero() || reader.Name != "" || reader.Comment != "" || reader.OS != 255 {
		t.Fatalf("pixel or gzip contract changed: %v %+v", actual, reader.Header)
	}
}

func TestWorldSkyPNGRejectsDecodedBoundsBeforePixels(t *testing.T) {
	t.Parallel()

	var input bytes.Buffer
	if err := png.Encode(&input, image.NewNRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}

	for _, dimensions := range [][2]uint32{{4097, 1}, {1, 4097}, {4096, 4096}, {0, 1}} {
		malformed := bytes.Clone(input.Bytes())
		binary.BigEndian.PutUint32(malformed[16:20], dimensions[0])
		binary.BigEndian.PutUint32(malformed[20:24], dimensions[1])
		binary.BigEndian.PutUint32(malformed[29:33], crc32.ChecksumIEEE(malformed[12:29]))

		if _, err := CompileWorldSkyImage(malformed); err == nil {
			t.Fatal("oversized PNG header accepted", dimensions)
		}
	}
}

func TestWorldSkyPNGRejectsAnimation(t *testing.T) {
	t.Parallel()

	var input bytes.Buffer
	if err := png.Encode(&input, image.NewNRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}

	chunk := make([]byte, 20)
	binary.BigEndian.PutUint32(chunk[:4], 8)
	copy(chunk[4:8], "acTL")
	binary.BigEndian.PutUint32(chunk[16:], crc32.ChecksumIEEE(chunk[4:16]))

	const headerEnd = 33

	animated := append(bytes.Clone(input.Bytes()[:headerEnd]), chunk...)
	animated = append(animated, input.Bytes()[headerEnd:]...)

	if _, err := CompileWorldSkyImage(animated); !errors.Is(err, ErrAnimatedImage) {
		t.Fatalf("APNG silently accepted first frame: %v", err)
	}
}
