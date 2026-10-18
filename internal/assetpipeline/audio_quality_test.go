package assetpipeline_test

import (
	"encoding/binary"
	"math"
	"os"
	"testing"

	"github.com/karty-game/karty-sdk/codec/qoa"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty/internal/assetpipeline"
)

func TestTransformWAVAlignsAttacksAndFlushesTail(t *testing.T) {
	t.Parallel()

	for _, impulse := range []int{100, 2390} {
		samples := make([]int64, 2400)
		samples[impulse] = 20_000

		pcm, err := assetpipeline.TransformWAV(buildWAV(t, wavOptions{
			encoding: testWavePCM, bits: 16, channels: 1, sampleRate: 24_000,
			data: encodeIntegers(16, samples...),
		}), asset.AudioRecipe{SampleRate: 48_000, ChannelMode: asset.ChannelPreserve})
		if err != nil {
			t.Fatal(err)
		}

		peak := 0
		for i, sample := range pcm.Samples {
			if math.Abs(float64(sample)) > math.Abs(float64(pcm.Samples[peak])) {
				peak = i
			}
		}

		if pcm.Frames != 4800 || math.Abs(float64(peak-impulse*2)) > 1 || pcm.Samples[peak] < 12_000 {
			t.Fatalf("attack/tail shifted or lost: source=%d output=%d peak=%d frames=%d", impulse, peak, pcm.Samples[peak], pcm.Frames)
		}
	}
}

func TestTransformWAVPreservesShortFiniteDuration(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		inputRate, outputRate uint32
		frames, want          int
	}{
		{384_000, 24_000, 1, 1}, {44_100, 48_000, 1, 2}, {24_000, 22_050, 7, 7},
	} {
		samples := make([]int64, test.frames)
		for i := range samples {
			samples[i] = 2000
		}

		pcm, err := assetpipeline.TransformWAV(
			buildWAV(
				t,
				wavOptions{encoding: testWavePCM, bits: 16, channels: 1, sampleRate: test.inputRate, data: encodeIntegers(16, samples...)},
			),
			asset.AudioRecipe{SampleRate: test.outputRate, ChannelMode: asset.ChannelPreserve},
		)
		if err != nil {
			t.Fatal(err)
		}

		if int(pcm.Frames) != test.want {
			t.Fatalf("%d -> %d Hz: short duration %d, want %d", test.inputRate, test.outputRate, pcm.Frames, test.want)
		}

		for _, sample := range pcm.Samples {
			if math.Abs(float64(sample)-2000) > 1 {
				t.Fatal("constant endpoints lost", sample)
			}
		}
	}
}

func TestProcessedResampledLoopRetainsBoundaryContinuity(t *testing.T) {
	t.Parallel()

	samples := make([]int64, 2400)
	for i := range samples {
		samples[i] = int64(math.Round(12_000 * math.Cos(2*math.Pi*100*float64(i)/24_000+.7)))
	}

	source := buildWAV(t, wavOptions{
		encoding: testWavePCM, bits: 16, channels: 1, sampleRate: 24_000,
		data: encodeIntegers(16, samples...),
	})

	pcm, err := assetpipeline.TransformWAV(source, asset.AudioRecipe{SampleRate: 48_000, ChannelMode: asset.ChannelPreserve})
	if err != nil {
		t.Fatal(err)
	}

	if step := math.Abs(float64(pcm.Samples[0]) - float64(pcm.Samples[len(pcm.Samples)-1])); step > 512 {
		t.Fatalf("resampling broke a periodic boundary: %f", step)
	}

	baseline, err := assetpipeline.ProcessAudio(source, asset.AudioRecipe{SampleRate: 24_000, ChannelMode: asset.ChannelPreserve})
	if err != nil {
		t.Fatal(err)
	}

	_, original, err := qoa.Decode(baseline.EncodedQOA)
	if err != nil {
		t.Fatal(err)
	}

	codecStep := math.Abs(float64(original[0]) - float64(original[len(original)-1]))

	processed, err := assetpipeline.ProcessAudio(source, asset.AudioRecipe{SampleRate: 48_000, ChannelMode: asset.ChannelPreserve})
	if err != nil {
		t.Fatal(err)
	}

	_, decoded, err := qoa.Decode(processed.EncodedQOA)
	if err != nil {
		t.Fatal(err)
	}

	step := math.Abs(float64(decoded[0]) - float64(decoded[len(decoded)-1]))
	// A nonzero first sample already has QOA predictor startup error. Bound
	// the conversion's additional step against that independently encoded source.
	if step > codecStep+256 {
		t.Fatalf("conversion increased the codec boundary step: %f PCM units, original=%f", step, codecStep)
	}
	// Distinguish a continuous boundary from the old causal zero lead-in.
	if math.Abs(float64(decoded[0])) < 4000 {
		t.Fatal("loop starts with a resampler latency gap")
	}
}

func TestMediaLabPackagedGainBudget(t *testing.T) {
	t.Parallel()
	// Actual authored fixtures and their selected recipes must fit at the
	// simultaneous voice cap with the music lane, including QOA overshoot.
	var peaks [2]float64

	for index, path := range []string{"../../samples/media-lab/assets/sounds/effects/machine-gun.wav", "../../samples/media-lab/assets/sounds/music/loop.wav"} {
		data := readAudioFixture(t, path)

		processed, err := assetpipeline.ProcessAudio(data, asset.AudioRecipe{SampleRate: 48_000, ChannelMode: asset.ChannelPreserve})
		if err != nil {
			t.Fatal(err)
		}

		_, pcm, err := qoa.Decode(processed.EncodedQOA)
		if err != nil {
			t.Fatal(err)
		}

		for _, sample := range pcm {
			peaks[index] = math.Max(peaks[index], math.Abs(float64(sample))/32768)
		}
	}

	if sum := float64(asset.MaxSimultaneousSoundVoices)*peaks[0] + peaks[1]; sum > .95 {
		t.Fatalf(
			"Media Lab lacks mix headroom at %d voices: effect=%f music=%f sum=%f",
			asset.MaxSimultaneousSoundVoices,
			peaks[0],
			peaks[1],
			sum,
		)
	}
}

// Keep the fixture read separate from WAV helpers, which intentionally generate
// test-only inputs rather than reaching into the sample distribution/cache.
func readAudioFixture(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) < 44 || binary.LittleEndian.Uint16(data[20:]) != 1 {
		t.Fatal("sample fixture is not PCM WAV")
	}

	return data
}
