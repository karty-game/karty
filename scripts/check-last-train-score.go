//go:build ignore

// Check the actual packaged score, including its lossy codec and loop seam.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"

	"github.com/karty-game/karty-sdk/codec/qoa"
	"github.com/karty-game/karty-sdk/format/cartridge"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	source, err := os.ReadFile("samples/last-train/assets/audio/last-carriage.wav")
	must(err)
	// The authored master is canonical PCM16 stereo WAV. Decode it directly:
	// the public one-shot WAV helper intentionally enforces the shorter SFX limit.
	if len(source) < 44 || string(source[:4]) != "RIFF" || string(source[8:16]) != "WAVEfmt " ||
		binary.LittleEndian.Uint32(source[16:]) != 16 || binary.LittleEndian.Uint16(source[20:]) != 1 ||
		binary.LittleEndian.Uint16(source[22:]) != 2 || binary.LittleEndian.Uint32(source[24:]) != 48000 ||
		binary.LittleEndian.Uint16(source[34:]) != 16 || string(source[36:40]) != "data" ||
		int(binary.LittleEndian.Uint32(source[40:])) != len(source)-44 || (len(source)-44)%4 != 0 {
		panic("authored master must be canonical 48 kHz stereo PCM16 WAV")
	}
	samples := make([]int16, (len(source)-44)/2)
	for i := range samples {
		samples[i] = int16(binary.LittleEndian.Uint16(source[44+i*2:]))
	}
	frames := uint32(len(samples) / 2)
	paths, err := filepath.Glob("samples/last-train/dist/web/content/*.kaud")
	must(err)
	found := false
	for _, path := range paths {
		encoded, err := os.ReadFile(path)
		must(err)
		media, err := cartridge.NewMediaReader(bytes.NewReader(encoded), cartridge.MediaKindQOAAudio, int64(len(encoded)))
		must(err)
		decoder, err := qoa.NewStreamDecoder(media)
		must(err)
		metadata := decoder.Metadata()
		if metadata.Frames != frames || metadata.SampleRate != 48000 || uint16(metadata.Channels) != 2 {
			continue
		}
		found = true
		decoded, err := io.ReadAll(decoder)
		must(err)
		must(media.Verify())
		if len(decoded) != len(samples)*2 {
			panic("decoded score length mismatch")
		}
		var signal, errorPower, peak, maxError float64
		var means [2]float64
		var first, last [2]int16
		for index, sample := range samples {
			actual := int16(binary.LittleEndian.Uint16(decoded[index*2:]))
			difference := float64(actual) - float64(sample)
			signal += float64(sample) * float64(sample)
			errorPower += difference * difference
			maxError = max(maxError, math.Abs(difference)/32768)
			peak = max(peak, math.Abs(float64(actual))/32768)
			means[index%2] += float64(actual) / 32768 / float64(frames)
			if index < 2 {
				first[index] = actual
			}
			last[index%2] = actual
		}
		snr := 10 * math.Log10(signal/errorPower)
		seam := [2]float64{math.Abs(float64(first[0])-float64(last[0])) / 32768, math.Abs(float64(first[1])-float64(last[1])) / 32768}
		if snr < 35 || peak >= .5 || max(seam[0], seam[1]) > .002 || math.Max(math.Abs(means[0]), math.Abs(means[1])) > .0001 {
			panic(fmt.Sprintf("score quality bounds failed: SNR=%f peak=%f seam=%v DC=%v", snr, peak, seam, means))
		}
		report := map[string]any{
			"sourceSHA256":         fmt.Sprintf("%x", sha256.Sum256(source)),
			"sidecarSHA256":        fmt.Sprintf("%x", sha256.Sum256(encoded)),
			"sidecar":              path,
			"seconds":              float64(frames) / float64(48000),
			"decodedPeak":          peak,
			"decodedDC":            means,
			"decodedBoundaryStep":  seam,
			"signalToCodecErrorDB": snr,
			"maxCodecError":        maxError,
			"physicalListening":    false,
		}
		data, err := json.MarshalIndent(report, "", "  ")
		must(err)
		must(os.MkdirAll("dist/validation/last-train/audio", 0750))
		must(os.WriteFile("dist/validation/last-train/audio/score.json", append(data, '\n'), 0600))
		fmt.Println(string(data))
	}
	if !found {
		panic("build the web sample first: packaged score missing or format differs")
	}
}
