package assetpipeline_test

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	sdkqoi "github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty/internal/assetpipeline"
)

func TestProcessImageAcceptsContentDetectedFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		format assetpipeline.ImageSourceFormat
		source []byte
	}{
		{name: "png", format: assetpipeline.ImageSourcePNG, source: encodePNG(t, testImage())},
		{name: "jpeg", format: assetpipeline.ImageSourceJPEG, source: encodeJPEG(t, testImage())},
		{name: "webp", format: assetpipeline.ImageSourceWebP, source: decodeBase64(t, losslessWebP)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result, err := assetpipeline.ProcessImage(test.source, imageRecipe(0, 0, asset.ImageFilterNearest))
			if err != nil {
				t.Fatal(err)
			}

			if result.Metadata.SourceFormat != test.format || result.Encoding != assetpipeline.ImageEncodingQOI {
				t.Fatalf("result metadata = %+v, encoding = %q", result.Metadata, result.Encoding)
			}

			if !bytes.HasPrefix(result.EncodedQOI, []byte(sdkqoi.Magic)) {
				t.Fatalf("output does not have QOI magic: %x", result.EncodedQOI[:min(4, len(result.EncodedQOI))])
			}
		})
	}
}

func TestProcessImageAspectFitAndNoUpscale(t *testing.T) {
	t.Parallel()

	source := image.NewNRGBA(image.Rect(0, 0, 8, 4))
	for index := range source.Pix {
		source.Pix[index] = byte(index)
	}

	encoded := encodePNG(t, source)

	tests := []struct {
		name       string
		maxWidth   uint32
		maxHeight  uint32
		wantWidth  uint32
		wantHeight uint32
	}{
		{name: "both bounds", maxWidth: 3, maxHeight: 3, wantWidth: 3, wantHeight: 1},
		{name: "height bound", maxHeight: 2, wantWidth: 4, wantHeight: 2},
		{name: "width bound", maxWidth: 6, wantWidth: 6, wantHeight: 3},
		{name: "no upscale", maxWidth: 80, maxHeight: 40, wantWidth: 8, wantHeight: 4},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result, err := assetpipeline.ProcessImage(encoded, imageRecipe(test.maxWidth, test.maxHeight, asset.ImageFilterNearest))
			if err != nil {
				t.Fatal(err)
			}

			if result.Metadata.Width != test.wantWidth || result.Metadata.Height != test.wantHeight {
				t.Fatalf("dimensions = %dx%d, want %dx%d", result.Metadata.Width, result.Metadata.Height, test.wantWidth, test.wantHeight)
			}
		})
	}
}

func TestProcessImageFiltersAndStraightAlpha(t *testing.T) {
	t.Parallel()

	source := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	source.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	source.SetNRGBA(1, 0, color.NRGBA{B: 255, A: 0})
	encoded := encodePNG(t, source)

	nearest, err := assetpipeline.ProcessImage(encoded, imageRecipe(1, 1, asset.ImageFilterNearest))
	if err != nil {
		t.Fatal(err)
	}

	smooth, err := assetpipeline.ProcessImage(encoded, imageRecipe(1, 1, asset.ImageFilterSmooth))
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Equal(nearest.EncodedQOI, smooth.EncodedQOI) {
		t.Fatal("nearest and Lanczos recipes produced identical output")
	}

	_, decoded, err := sdkqoi.Decode(smooth.EncodedQOI)
	if err != nil {
		t.Fatal(err)
	}

	pixel, ok := color.NRGBAModel.Convert(decoded.At(0, 0)).(color.NRGBA)
	if !ok {
		t.Fatal("decoded QOI pixel is not NRGBA")
	}

	if pixel.R < 240 || pixel.B > 8 || pixel.A == 0 || pixel.A == 255 {
		t.Fatalf("smooth straight-alpha pixel = %#v", pixel)
	}
}

func TestProcessImageConverts16BitPNGExplicitly(t *testing.T) {
	t.Parallel()

	source := image.NewNRGBA64(image.Rect(0, 0, 1, 1))
	source.SetNRGBA64(0, 0, color.NRGBA64{R: 0x1234, G: 0x5678, B: 0x9abc, A: 0xdef0})

	result, err := assetpipeline.ProcessImage(encodePNG(t, source), imageRecipe(0, 0, asset.ImageFilterNearest))
	if err != nil {
		t.Fatal(err)
	}

	if result.Metadata.SourceBitDepth != 16 || result.Metadata.BitDepth != 8 || !result.Metadata.PrecisionReduced {
		t.Fatalf("metadata = %+v", result.Metadata)
	}

	_, decoded, err := sdkqoi.Decode(result.EncodedQOI)
	if err != nil {
		t.Fatal(err)
	}

	pixel, ok := color.NRGBAModel.Convert(decoded.At(0, 0)).(color.NRGBA)
	if !ok {
		t.Fatal("decoded QOI pixel is not NRGBA")
	}

	if pixel != (color.NRGBA{R: 0x12, G: 0x56, B: 0x9a, A: 0xde}) {
		t.Fatalf("normalized pixel = %#v", pixel)
	}
}

func TestProcessImageRejectsAnimationsAndMalformedInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source []byte
		want   error
	}{
		{name: "unknown", source: []byte("not an image"), want: assetpipeline.ErrImageFormat},
		{name: "truncated PNG", source: []byte("\x89PNG\r\n\x1a\n"), want: assetpipeline.ErrImageFormat},
		{name: "APNG", source: appendPNGAnimationChunk(t, encodePNG(t, testImage())), want: assetpipeline.ErrAnimatedImage},
		{name: "animated WebP flag", source: animatedWebP(), want: assetpipeline.ErrAnimatedImage},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := assetpipeline.ProcessImage(test.source, imageRecipe(0, 0, asset.ImageFilterNearest))
			if err == nil {
				t.Fatal("ProcessImage() accepted invalid input")
			}

			if !errors.Is(err, test.want) {
				t.Fatalf("ProcessImage() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestProcessImageEnforcesSourceAndDecodedBounds(t *testing.T) {
	t.Parallel()

	oversized := make([]byte, asset.MaxSourceAssetBytes+1)

	_, err := assetpipeline.ProcessImage(oversized, imageRecipe(1, 1, asset.ImageFilterNearest))
	if !errors.Is(err, assetpipeline.ErrImageSourceTooLarge) {
		t.Fatalf("oversized source error = %v", err)
	}

	header := pngWithDimensions(t, asset.MaxSourceImageDimension+1, 1)

	_, err = assetpipeline.ProcessImage(header, imageRecipe(1, 1, asset.ImageFilterNearest))
	if !errors.Is(err, assetpipeline.ErrImageBounds) {
		t.Fatalf("oversized dimensions error = %v", err)
	}

	valid := encodePNG(t, testImage())
	invalidRecipe := imageRecipe(0, 0, asset.ImageFilterNearest)

	invalidRecipe.BitDepth = 16
	if _, err := assetpipeline.ProcessImage(valid, invalidRecipe); err == nil {
		t.Fatal("ProcessImage() accepted invalid recipe")
	}
}

func TestProcessImageIsDeterministicAndOwnsOutput(t *testing.T) {
	t.Parallel()

	source := encodePNG(t, testImage())
	original := bytes.Clone(source)
	recipe := imageRecipe(2, 1, asset.ImageFilterSmooth)

	first, err := assetpipeline.ProcessImage(source, recipe)
	if err != nil {
		t.Fatal(err)
	}

	second, err := assetpipeline.ProcessImage(source, recipe)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(first.EncodedQOI, second.EncodedQOI) || first.SourceDigest != second.SourceDigest ||
		first.OutputDigest != second.OutputDigest || first.RecipeDigest != second.RecipeDigest {
		t.Fatal("identical source and recipe produced different results")
	}

	const goldenOutputSHA256 = "7a46a000d2c841e88f381773a6baf9bb2974fc7354bfe85ceb7704250b130d62"
	if got := formatDigest(first.OutputDigest); got != goldenOutputSHA256 {
		t.Fatalf("output digest = %s, want %s", got, goldenOutputSHA256)
	}

	source[0] ^= 0xff
	if !bytes.Equal(first.EncodedQOI, second.EncodedQOI) || bytes.Equal(source, original) {
		t.Fatal("result unexpectedly aliases source")
	}

	changed, err := assetpipeline.ProcessImage(original, imageRecipe(1, 1, asset.ImageFilterSmooth))
	if err != nil {
		t.Fatal(err)
	}

	if first.RecipeDigest == changed.RecipeDigest {
		t.Fatal("effective recipe change did not change recipe digest")
	}
}

func imageRecipe(maxWidth, maxHeight uint32, filter asset.ImageFilter) asset.ImageRecipe {
	return asset.ImageRecipe{MaxWidth: maxWidth, MaxHeight: maxHeight, Filter: filter, BitDepth: 8}
}

func testImage() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))

	colors := []color.NRGBA{
		{R: 255, A: 255}, {G: 255, A: 128}, {B: 255, A: 64},
		{R: 40, G: 80, B: 120, A: 255}, {R: 10, G: 20, B: 30, A: 0}, {R: 250, G: 240, B: 230, A: 200},
	}
	for index, value := range colors {
		img.SetNRGBA(index%3, index/3, value)
	}

	return img
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()

	var output bytes.Buffer
	if err := png.Encode(&output, img); err != nil {
		t.Fatal(err)
	}

	return output.Bytes()
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()

	var output bytes.Buffer
	if err := jpeg.Encode(&output, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}

	return output.Bytes()
}

func decodeBase64(t *testing.T, value string) []byte {
	t.Helper()

	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}

	return decoded
}

func appendPNGAnimationChunk(t *testing.T, source []byte) []byte {
	t.Helper()

	const signatureAndIHDR = 8 + 12 + 13

	chunk := pngChunk("acTL", make([]byte, 8))

	output := append([]byte{}, source[:signatureAndIHDR]...)
	output = append(output, chunk...)

	return append(output, source[signatureAndIHDR:]...)
}

func animatedWebP() []byte {
	chunk := make([]byte, 18)
	copy(chunk[:4], "VP8X")
	binary.LittleEndian.PutUint32(chunk[4:8], 10)
	chunk[8] = 1 << 1
	output := append([]byte("RIFF\x00\x00\x00\x00WEBP"), chunk...)
	binary.LittleEndian.PutUint32(output[4:8], uint32(len(output)-8))

	return output
}

func pngWithDimensions(t *testing.T, width, height int) []byte {
	t.Helper()

	data := make([]byte, 13)
	binary.BigEndian.PutUint32(data[0:4], uint32(width))
	binary.BigEndian.PutUint32(data[4:8], uint32(height))
	data[8] = 8
	data[9] = 6
	output := append([]byte("\x89PNG\r\n\x1a\n"), pngChunk("IHDR", data)...)

	return append(output, pngChunk("IEND", nil)...)
}

func pngChunk(kind string, data []byte) []byte {
	chunk := make([]byte, 12+len(data))
	binary.BigEndian.PutUint32(chunk[:4], uint32(len(data)))
	copy(chunk[4:8], kind)
	copy(chunk[8:], data)
	checksum := crc32.ChecksumIEEE(chunk[4 : 8+len(data)])
	binary.BigEndian.PutUint32(chunk[8+len(data):], checksum)

	return chunk
}

func formatDigest(digest [32]byte) string {
	const hex = "0123456789abcdef"

	output := make([]byte, 64)
	for index, value := range digest {
		output[index*2] = hex[value>>4]
		output[index*2+1] = hex[value&0x0f]
	}

	return string(output)
}

const losslessWebP = "UklGRrIBAABXRUJQVlA4TKUBAAAvSsAYAA8w//M///MfeJAkbXvaSG7m8Q3GfYSBJekwQztm/IcZlgwnmWImn2BK7aFmBtnVir6q//8VOkFE/xm4baTIu8c48ArEo6+B3zFKYln3pqClSCKX0begFTAXFOLXHSyF8cCNcZEG4OywuA4KVVfJCiArU7GAgJI8+lJP/OKMT/fBAjevg1cYB7YVkFuWga2lyPi5I0HFy5YTpWIHg0RZpkniRVW9odHAKOwosWuOGdxIyn2OvaCDvhg/we6TwadPBPbqBV58MsLmMJ8yZnOWk8SRz4N+QoyPL+MnamzMvcE1rHNEr91F9GKZPVUcS9w7PhhH36suB9qPeYb/oLk6cuTiJ0wOK3m5h1cKjW6EVZCYMK7dxcKCBdgP9HkKr9gkAO2P8GKZGWVdIAatQa+1IDpt6qyorVwdy01xdW8Jkfk6xjEXmVQQ+HQdFr6OKhIN34dXWq0+0qr6EJSCeeVLH9+gvGTLyqM65PQ44ihzlTXxQKjKbAvshXgir7Lil9w4L2bvMycmjQcqXaMCO6BlY28i+FOLzbfI1vEqxAhotocAAA=="
