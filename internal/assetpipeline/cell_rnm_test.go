package assetpipeline

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/karty-game/karty-sdk/codec/ktx2"
	"github.com/karty-game/karty/internal/sdk"
)

func TestCellRNMActualEncoderPreservesThreeLocalLayers(t *testing.T) {
	encoder := os.Getenv("KARTY_TEST_KTX")
	if encoder == "" {
		t.Skip("KARTY_TEST_KTX not selected")
	}

	t.Setenv("KARTY_KTX", encoder)

	pixels := make([]byte, 2*3*3*4)
	for index := range pixels {
		pixels[index] = byte(index*53 + index/7)
	}

	root := t.TempDir()

	encoded, first, err := EncodeCellRNM(t.Context(), root, sdk.Manifest{}, 2, 3, pixels)
	if err != nil {
		t.Fatal(err)
	}

	info, err := ktx2.InspectRGBA8(encoded)
	if err != nil || info.Layers != 3 || info.Width != 2 || info.Height != 3 || len(info.Levels) != 1 ||
		info.RGBABytes != uint64(len(pixels)) {
		t.Fatal(info, err)
	}

	warm, cached, err := EncodeCellRNM(t.Context(), root, sdk.Manifest{}, 2, 3, pixels)
	if err != nil || first.CacheHit || !cached.CacheHit || first.CacheKey != cached.CacheKey || !bytes.Equal(encoded, warm) {
		t.Fatal("cell encoder cache mismatch", err)
	}

	input := filepath.Join(t.TempDir(), "cell.ktx2")
	if err := os.WriteFile(input, encoded, 0o600); err != nil {
		t.Fatal(err)
	}

	for layer := range 3 {
		output := filepath.Join(t.TempDir(), "decoded.raw")
		// #nosec G204 G702 -- Opt-in actual pinned encoder on generated bounded fixture files.
		command := exec.CommandContext(t.Context(), encoder, "extract", "--raw", "--layer", strconv.Itoa(layer), input, output)

		command.Env = ktxEncoderEnvironment(os.Environ())
		if log, err := command.CombinedOutput(); err != nil {
			t.Fatal(err, string(log))
		}

		decoded, err := os.ReadFile(output)
		if err != nil || !bytes.Equal(decoded, pixels[layer*24:(layer+1)*24]) {
			t.Fatal("lossless cell bytes changed", layer, err)
		}
	}

	pixels[0]++

	_, changed, err := EncodeCellRNM(t.Context(), root, sdk.Manifest{}, 2, 3, pixels)
	if err != nil || changed.CacheHit || first.CacheKey == changed.CacheKey {
		t.Fatal("cell pixel mutation reused cached output", err)
	}
}

func TestCellRNMRejectsInvalidDimensionsOrPixels(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		width, height uint32
		pixels        []byte
	}{{}, {1, 1, []byte{1}}, {2049, 1, make([]byte, 2049*12)}} {
		if _, _, err := EncodeCellRNM(t.Context(), t.TempDir(), sdk.Manifest{}, test.width, test.height, test.pixels); err == nil {
			t.Fatal("invalid cell source accepted")
		}
	}
}
