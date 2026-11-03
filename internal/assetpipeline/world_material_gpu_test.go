package assetpipeline

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty-sdk/format/worldmaterialgpu"
	"github.com/karty-game/karty/internal/sdk"
)

func gpuMaterialFixture(t *testing.T) (worldmaterial.Pair, map[uint32]*image.NRGBA) {
	t.Helper()

	layout, err := worldmaterial.NewLayout([]uint32{7, 9})
	if err != nil {
		t.Fatal(err)
	}

	pixels := [3]*image.NRGBA{}
	for index := range pixels {
		pixels[index] = image.NewNRGBA(image.Rect(0, 0, layout.Width, layout.Height))
		for offset := 3; offset < len(pixels[index].Pix); offset += 4 {
			pixels[index].Pix[offset] = 255
		}
	}

	tiles := make(map[uint32]*image.NRGBA)

	for slot, rect := range layout.Materials {
		for row := range rect.Height {
			for column := range rect.Width {
				pixels[0].SetNRGBA(rect.X+column, rect.Y+row, color.NRGBA{byte(slot * 255), 90, 120, 255})
				pixels[1].SetNRGBA(rect.X+column, rect.Y+row, color.NRGBA{190, 100, 13, 17})
				pixels[2].SetNRGBA(rect.X+column, rect.Y+row, color.NRGBA{42, 173, 0, 255})
			}
		}

		tiles[rect.MaterialID] = flatSSBump(256, 256)
		for offset := 0; offset < len(tiles[rect.MaterialID].Pix); offset += 4 {
			copy(tiles[rect.MaterialID].Pix[offset:offset+4], []byte{10, 20, 30, byte(70 + slot)})
		}
	}

	layout.Materials[0].Strengths = &worldmaterial.Strengths{Normal: 0, Height: 1, AO: 1, Rim: 1}
	encoded := [3][]byte{}

	for index := range pixels {
		space := qoi.ColorspaceLinear
		if index == 0 {
			space = qoi.ColorspaceSRGB
		}

		encoded[index], _, err = qoi.Encode(pixels[index], qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: space})
		if err != nil {
			t.Fatal(err)
		}
	}

	return worldmaterial.Pair{Layout: layout, Albedo: encoded[0], Data: encoded[1], Reflectance: encoded[2]}, tiles
}

func TestPreparedGPUChannelsAndMipSemantics(t *testing.T) {
	t.Parallel()

	manifest := sdk.Manifest{}

	manifest.Assets.Capabilities.Runtime = []asset.Capability{worldmaterialgpu.Feature}
	if !materialReflectanceEnabled(manifest) {
		t.Fatal("prepared material capability did not generate metallic/smoothness")
	}

	base, tiles := gpuMaterialFixture(t)
	before := bytes.Clone(tiles[7].Pix)

	arrays, err := PrepareWorldMaterialGPU(t.Context(), base, tiles)
	if err != nil {
		t.Fatal(err)
	}

	if !arrays[0].SRGB || arrays[1].SRGB || arrays[2].SRGB {
		t.Fatal("incorrect transfer functions")
	}

	for level := range 9 {
		dimension := max(1, 256>>level)

		layerBytes := dimension * dimension * 4
		for slot := range 2 {
			if got := arrays[1].Levels[level][slot*layerBytes : slot*layerBytes+4]; !bytes.Equal(got, []byte{10, 20, 30, byte(70 + slot)}) {
				t.Fatal("SSBump energy or independent emission changed", level, slot, got)
			}

			if got := arrays[2].Levels[level][slot*layerBytes+2 : slot*layerBytes+4]; !bytes.Equal(got, []byte{42, 173}) {
				t.Fatal("height/AO leaked into reflectance or controls quantized", level, got)
			}
		}

		if got := arrays[2].Levels[level][:2]; !bytes.Equal(got, []byte{128, 128}) {
			t.Fatal("normal strength not baked", level, got)
		}

		if arrays[0].Levels[level][0] != 0 || arrays[0].Levels[level][layerBytes] != 255 {
			t.Fatal("material layers mixed")
		}
	}

	if !bytes.Equal(before, tiles[7].Pix) {
		t.Fatal("source SSBump mutated")
	}

	delete(tiles, 9)

	if _, err := PrepareWorldMaterialGPU(t.Context(), base, tiles); err == nil {
		t.Fatal("mandatory SSBump missing")
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := PrepareWorldMaterialGPU(ctx, base, nil); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation lost", err)
	}
}

// This test executes a real explicit encoder. Ordinary tests remain independent
// of developer tools and networks; set KARTY_TEST_KTX for encoder qualification.
func TestMaterialKTX2ActualEncoderAndWarmCache(t *testing.T) {
	encoder := os.Getenv("KARTY_TEST_KTX")
	if encoder == "" {
		t.Skip("KARTY_TEST_KTX not selected")
	}

	t.Setenv("KARTY_KTX", encoder)
	base, tiles := gpuMaterialFixture(t)

	arrays, err := PrepareWorldMaterialGPU(t.Context(), base, tiles)
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	for _, request := range arrays {
		encoded, first, err := EncodeMaterialKTX2(t.Context(), root, sdk.Manifest{}, request)
		if err != nil {
			t.Fatal(err)
		}

		warm, cached, err := EncodeMaterialKTX2(t.Context(), root, sdk.Manifest{}, request)
		if err != nil || !cached.CacheHit || first.CacheHit || first.CacheKey != cached.CacheKey || !bytes.Equal(encoded, warm) {
			t.Fatal("encoder cache mismatch", err)
		}

		checkKTX2DecodedPixels(t, encoder, encoded, request)
	}

	changed := arrays[1]
	changed.Levels = append([][]byte(nil), changed.Levels...)
	changed.Levels[0] = bytes.Clone(changed.Levels[0])
	changed.Levels[0][0]++

	_, cached, err := EncodeMaterialKTX2(t.Context(), root, sdk.Manifest{}, changed)
	if err != nil || cached.CacheHit {
		t.Fatal("input change reused old encoder cache", err)
	}
}

func checkKTX2DecodedPixels(t *testing.T, encoder string, encoded []byte, request KTX2EncodeRequest) {
	t.Helper()
	directory := t.TempDir()

	input := filepath.Join(directory, "material.ktx2")
	if err := os.WriteFile(input, encoded, 0o600); err != nil {
		t.Fatal(err)
	}

	for _, selection := range [][2]int{{0, 0}, {8, 1}} {
		level, layer := selection[0], selection[1]
		output := filepath.Join(directory, "decoded.raw")

		// #nosec G204 G702 -- This opt-in qualification runs only the caller-selected encoder on generated fixture paths.
		command := exec.CommandContext(t.Context(), encoder, "extract", "--transcode", "rgba8", "--raw",
			"--level", strconv.Itoa(level), "--layer", strconv.Itoa(layer), input, output)
		if logs, err := command.CombinedOutput(); err != nil {
			t.Fatal("actual KTX transcoding", err, string(logs))
		}

		decoded, err := os.ReadFile(output)
		dimension := max(1, 256>>level)

		layerBytes := dimension * dimension * 4
		if err != nil || len(decoded) != layerBytes {
			t.Fatal("decoded dimensions", err, len(decoded))
		}

		want := request.Levels[level][layer*layerBytes : layer*layerBytes+4]
		for channel := range 4 {
			delta := int(decoded[channel]) - int(want[channel])
			if delta < -4 || delta > 4 {
				t.Fatal("UASTC changed packed channels or material/mip order", selection, decoded[:4], want)
			}
		}
	}
}
