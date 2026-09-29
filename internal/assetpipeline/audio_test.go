package assetpipeline_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/karty-game/karty-sdk/codec/qoa"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty/internal/assetpipeline"
)

const (
	testWavePCM        = 0x0001
	testWaveFloat      = 0x0003
	testWaveExtensible = 0xfffe
)

func TestDecodeWAVNormalizesSupportedSamples(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		encoding uint16
		bits     uint16
		data     []byte
		want     []int16
	}{
		{name: "pcm8", encoding: testWavePCM, bits: 8, data: []byte{0, 128, 255}, want: []int16{-32768, 0, 32512}},
		{
			name: "pcm16", encoding: testWavePCM, bits: 16,
			data: encodeIntegers(16, -32768, -1, 0, 32767), want: []int16{-32768, -1, 0, 32767},
		},
		{
			name: "pcm24", encoding: testWavePCM, bits: 24,
			data: encodeIntegers(24, -8388608, -384, 0, 384, 8388607), want: []int16{-32768, -2, 0, 2, 32767},
		},
		{
			name: "pcm32", encoding: testWavePCM, bits: 32,
			data: encodeIntegers(32, math.MinInt32, -98304, 0, 98304, math.MaxInt32), want: []int16{-32768, -2, 0, 2, 32767},
		},
		{
			name: "float32", encoding: testWaveFloat, bits: 32,
			data: encodeFloats(-1.5, -0.5, 0, 0.5, 1.5), want: []int16{-32768, -16384, 0, 16384, 32767},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			pcm, err := assetpipeline.DecodeWAV(buildWAV(t, wavOptions{
				encoding: test.encoding, bits: test.bits, channels: 1, sampleRate: 48_000, data: test.data,
			}))
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(pcm.Samples, test.want) {
				t.Fatalf("samples = %v, want %v", pcm.Samples, test.want)
			}

			if pcm.Frames != uint32(len(test.want)) || pcm.Channels != 1 || pcm.SourceBitDepth != test.bits {
				t.Fatalf("metadata = %+v", pcm)
			}

			if pcm.SourceFloat != (test.encoding == testWaveFloat) {
				t.Fatalf("SourceFloat = %v", pcm.SourceFloat)
			}
		})
	}
}

func TestDecodeWAVHandlesUnknownOddChunkAndExtensibleFormats(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		subformat uint16
		data      []byte
		want      []int16
	}{
		{name: "pcm", subformat: testWavePCM, data: encodeIntegers(16, -1234, 5678), want: []int16{-1234, 5678}},
		{name: "float", subformat: testWaveFloat, data: encodeFloats(-0.25, 0.25), want: []int16{-8192, 8192}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			pcm, err := assetpipeline.DecodeWAV(buildWAV(t, wavOptions{
				encoding: testWaveExtensible, extensibleSubformat: test.subformat,
				bits:     map[uint16]uint16{testWavePCM: 16, testWaveFloat: 32}[test.subformat],
				channels: 1, sampleRate: 48_000, data: test.data,
				beforeFormat: []wavChunk{{id: "JUNK", data: []byte{1, 2, 3}}},
			}))
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(pcm.Samples, test.want) {
				t.Fatalf("samples = %v, want %v", pcm.Samples, test.want)
			}
		})
	}
}

func TestDecodeWAVRejectsMalformedAndUnsupportedInput(t *testing.T) {
	t.Parallel()

	valid := buildWAV(t, wavOptions{encoding: testWavePCM, bits: 16, channels: 1, sampleRate: 48_000, data: encodeIntegers(16, 1)})
	rifx := append([]byte(nil), valid...)
	copy(rifx[:4], "RIFX")

	rf64 := append([]byte(nil), valid...)
	copy(rf64[:4], "RF64")

	inconsistent := append([]byte(nil), valid...)
	binary.LittleEndian.PutUint32(inconsistent[28:32], 1)

	truncatedChunk := append([]byte(nil), valid...)
	binary.LittleEndian.PutUint32(truncatedChunk[40:44], math.MaxUint32)

	tests := []struct {
		name string
		wav  []byte
	}{
		{name: "rifx", wav: rifx},
		{name: "rf64", wav: rf64},
		{name: "compressed", wav: buildWAV(t, wavOptions{encoding: 6, bits: 8, channels: 1, sampleRate: 8_000, data: []byte{0}})},
		{
			name: "duplicate fmt",
			wav: buildWAV(t, wavOptions{
				encoding: testWavePCM, bits: 16, channels: 1, sampleRate: 48_000,
				data: encodeIntegers(16, 1), duplicateFormat: true,
			}),
		},
		{
			name: "duplicate data",
			wav: buildWAV(t, wavOptions{
				encoding: testWavePCM, bits: 16, channels: 1, sampleRate: 48_000,
				data: encodeIntegers(16, 1), duplicateData: true,
			}),
		},
		{name: "inconsistent rates", wav: inconsistent},
		{
			name: "nonfinite float",
			wav: buildWAV(t, wavOptions{
				encoding: testWaveFloat, bits: 32, channels: 1, sampleRate: 48_000,
				data: encodeFloats(float32(math.NaN())),
			}),
		},
		{name: "truncated chunk", wav: truncatedChunk},
		{
			name: "unaligned data",
			wav: buildWAV(t, wavOptions{
				encoding: testWavePCM, bits: 16, channels: 1, sampleRate: 48_000, data: []byte{1},
			}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := assetpipeline.DecodeWAV(test.wav)
			if !errors.Is(err, assetpipeline.ErrInvalidWAV) {
				t.Fatalf("DecodeWAV() error = %v", err)
			}
		})
	}
}

func TestDecodeWAVEnforcesDurationBound(t *testing.T) {
	t.Parallel()

	data := make([]byte, 22_050*31)

	_, err := assetpipeline.DecodeWAV(buildWAV(t, wavOptions{
		encoding: testWavePCM, bits: 8, channels: 1, sampleRate: 22_050, data: data,
	}))
	if !errors.Is(err, assetpipeline.ErrAudioBounds) {
		t.Fatalf("DecodeWAV() error = %v", err)
	}
}

func TestTransformWAVDownmixesBeforeResampling(t *testing.T) {
	t.Parallel()

	pcm, err := assetpipeline.TransformWAV(buildWAV(t, wavOptions{
		encoding: testWavePCM, bits: 16, channels: 2, sampleRate: 48_000,
		data: encodeIntegers(16, 1, 0, -1, 0, 32767, -32768),
	}), asset.AudioRecipe{SampleRate: 48_000, ChannelMode: asset.ChannelMono})
	if err != nil {
		t.Fatal(err)
	}

	want := []int16{1, -1, -1}
	if pcm.Channels != 1 || pcm.Frames != 3 || !reflect.DeepEqual(pcm.Samples, want) {
		t.Fatalf("downmixed PCM = %+v, want %v", pcm, want)
	}
}

func TestTransformWAVResamplingDurationAndAliasRejection(t *testing.T) {
	t.Parallel()

	const inputRate = 48_000

	samples := make([]int64, inputRate)
	for index := range samples {
		samples[index] = int64(math.Round(30_000 * math.Sin(2*math.Pi*18_000*float64(index)/inputRate)))
	}

	pcm, err := assetpipeline.TransformWAV(buildWAV(t, wavOptions{
		encoding: testWavePCM, bits: 16, channels: 1, sampleRate: inputRate,
		data: encodeIntegers(16, samples...),
	}), asset.AudioRecipe{SampleRate: 24_000, ChannelMode: asset.ChannelPreserve})
	if err != nil {
		t.Fatal(err)
	}

	if pcm.Frames != 24_000 || len(pcm.Samples) != 24_000 {
		t.Fatalf("resampled shape = %+v", pcm)
	}

	var squares float64
	for _, sample := range pcm.Samples[256 : len(pcm.Samples)-256] {
		squares += float64(sample) * float64(sample)
	}

	rms := math.Sqrt(squares / float64(len(pcm.Samples)-512))
	if rms > 1 {
		t.Fatalf("out-of-band alias RMS = %.6f, want <= 1 PCM unit", rms)
	}
}

func TestTransformWAVIsDeterministicGolden(t *testing.T) {
	t.Parallel()

	samples := make([]int64, 441)
	for index := range samples {
		samples[index] = int64((index*7919)%60_001 - 30_000)
	}

	source := buildWAV(t, wavOptions{
		encoding: testWavePCM, bits: 24, channels: 1, sampleRate: 44_100,
		data: encodeIntegers(24, samples...),
	})
	recipe := asset.AudioRecipe{SampleRate: 48_000, ChannelMode: asset.ChannelPreserve}

	first, err := assetpipeline.TransformWAV(source, recipe)
	if err != nil {
		t.Fatal(err)
	}

	second, err := assetpipeline.TransformWAV(source, recipe)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(first, second) || first.Frames != 480 {
		t.Fatalf("resampling is not deterministic or has wrong duration: first=%d second=%d", first.Frames, second.Frames)
	}

	digest := sha256.Sum256(encodePCM16(first.Samples))

	const wantDigest = "36b8a8c074d2d7caa2e8d5b605c7188f27e73f55b658103393bb3301364c721c"
	if got := hex.EncodeToString(digest[:]); got != wantDigest {
		t.Fatalf("golden digest = %s, want %s", got, wantDigest)
	}
}

func TestTransformWAVRejectsInvalidRecipe(t *testing.T) {
	t.Parallel()

	source := buildWAV(t, wavOptions{encoding: testWavePCM, bits: 16, channels: 1, sampleRate: 48_000, data: encodeIntegers(16, 1)})
	if _, err := assetpipeline.TransformWAV(source, asset.AudioRecipe{SampleRate: 96_000, ChannelMode: asset.ChannelPreserve}); err == nil {
		t.Fatal("TransformWAV() accepted an invalid recipe")
	}
}

func TestProcessAudioMatchesOfficialQOAReference(t *testing.T) {
	t.Parallel()

	input := []int64{
		0, 1000, -1000, 2000, -2000, 3000, -3000, 4000, -4000, 5000,
		-5000, 6000, -6000, 7000, -7000, 8000, -8000, 9000, -9000, 0,
	}
	source := buildWAV(t, wavOptions{
		encoding: testWavePCM, bits: 16, channels: 1, sampleRate: 48_000,
		data: encodeIntegers(16, input...),
	})

	result, err := assetpipeline.ProcessAudio(source, asset.AudioRecipe{
		SampleRate: 48_000, ChannelMode: asset.ChannelPreserve,
	})
	if err != nil {
		t.Fatal(err)
	}

	const referenceQOA = "716f6166000000140100bb8000140020000000000000000000000000e0004000f04ab2ef2ef1ab1d"
	if got := hex.EncodeToString(result.EncodedQOA); got != referenceQOA {
		t.Fatalf("encoded QOA = %s, want official reference %s", got, referenceQOA)
	}

	metadata, decoded, err := qoa.Decode(result.EncodedQOA)
	if err != nil {
		t.Fatal(err)
	}

	wantDecoded := []int16{
		1536, 1554, 36, 3601, -1867, 1738, -4022, 5387, -2340, 3153,
		-5744, 6900, -5157, 5847, -5231, 6172, -9595, 7583, -7514, -1590,
	}
	if metadata.Channels != 1 || metadata.SampleRate != 48_000 || metadata.Frames != 20 ||
		!reflect.DeepEqual(decoded, wantDecoded) {
		t.Fatalf("decoded output = %+v, %v", metadata, decoded)
	}

	if result.Encoding != assetpipeline.AudioEncodingQOA || result.OutputBytes != uint64(len(result.EncodedQOA)) ||
		result.SourceBytes != uint64(len(source)) || result.Metadata.SourceBitDepth != 16 ||
		result.Metadata.DecodedBytes != 40 {
		t.Fatalf("processed result = %+v", result)
	}
}

func TestProcessAudioOwnsBytesAndHasGoldenDigests(t *testing.T) {
	t.Parallel()

	source := buildWAV(t, wavOptions{
		encoding: testWavePCM, bits: 16, channels: 1, sampleRate: 48_000,
		data: encodeIntegers(16, 0, 1000, -1000, 2000, -2000, 3000, -3000, 4000, -4000, 0),
	})
	original := bytes.Clone(source)
	recipe := asset.AudioRecipe{SampleRate: 48_000, ChannelMode: asset.ChannelPreserve}

	first, err := assetpipeline.ProcessAudio(source, recipe)
	if err != nil {
		t.Fatal(err)
	}

	second, err := assetpipeline.ProcessAudio(original, recipe)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(first.EncodedQOA, second.EncodedQOA) || first.SourceDigest != second.SourceDigest ||
		first.OutputDigest != second.OutputDigest || first.RecipeDigest != second.RecipeDigest {
		t.Fatal("identical source and recipe produced different audio results")
	}

	const (
		wantSourceDigest = "497aa05de6c9e023033563eb37c96a0ad5525a4c66331d890ae40f56d5a9e84f"
		wantOutputDigest = "6a9d27827d09305ce86c1af636c0c3fa79e51f2f7098c30b05646e2fa97370cc"
		wantRecipeDigest = "7ccbf961d5acf3b1880ab0348ac07ba6e017b7d181728aeb2d01f28046e4424b"
	)

	if got := formatDigest(first.SourceDigest); got != wantSourceDigest {
		t.Fatalf("source digest = %s, want %s", got, wantSourceDigest)
	}

	if got := formatDigest(first.OutputDigest); got != wantOutputDigest {
		t.Fatalf("output digest = %s, want %s", got, wantOutputDigest)
	}

	if got := formatDigest(first.RecipeDigest); got != wantRecipeDigest {
		t.Fatalf("recipe digest = %s, want %s", got, wantRecipeDigest)
	}

	source[0] ^= 0xff

	first.EncodedQOA[0] ^= 0xff
	if bytes.Equal(source, original) || !bytes.Equal(second.EncodedQOA, append([]byte("qoaf"), second.EncodedQOA[4:]...)) {
		t.Fatal("processed audio aliases caller-owned source or another result")
	}
}

func TestProcessAudioRecipeInvalidationMonoAndResample(t *testing.T) {
	t.Parallel()

	frames := make([]int64, 441*2)
	for frame := range 441 {
		frames[frame*2] = int64((frame*127)%20_000 - 10_000)
		frames[frame*2+1] = int64(10_000 - (frame*83)%20_000)
	}

	source := buildWAV(t, wavOptions{
		encoding: testWavePCM, bits: 16, channels: 2, sampleRate: 44_100,
		data: encodeIntegers(16, frames...),
	})

	preserve, err := assetpipeline.ProcessAudio(source, asset.AudioRecipe{
		SampleRate: 44_100, ChannelMode: asset.ChannelPreserve,
	})
	if err != nil {
		t.Fatal(err)
	}

	mono, err := assetpipeline.ProcessAudio(source, asset.AudioRecipe{
		SampleRate: 44_100, ChannelMode: asset.ChannelMono,
	})
	if err != nil {
		t.Fatal(err)
	}

	resampled, err := assetpipeline.ProcessAudio(source, asset.AudioRecipe{
		SampleRate: 48_000, ChannelMode: asset.ChannelMono,
	})
	if err != nil {
		t.Fatal(err)
	}

	if preserve.Metadata.Channels != 2 || preserve.Metadata.Frames != 441 ||
		mono.Metadata.Channels != 1 || mono.Metadata.Frames != 441 ||
		resampled.Metadata.Channels != 1 || resampled.Metadata.Frames != 480 || resampled.Metadata.SampleRate != 48_000 {
		t.Fatalf("processed metadata: preserve=%+v mono=%+v resampled=%+v", preserve.Metadata, mono.Metadata, resampled.Metadata)
	}

	if preserve.RecipeDigest == mono.RecipeDigest || mono.RecipeDigest == resampled.RecipeDigest ||
		preserve.RecipeDigest == resampled.RecipeDigest {
		t.Fatal("effective audio recipe changes did not invalidate recipe identity")
	}

	for _, result := range []assetpipeline.ProcessedAudio{preserve, mono, resampled} {
		inspected, inspectErr := qoa.Inspect(result.EncodedQOA)
		if inspectErr != nil || inspected.Channels != result.Metadata.Channels ||
			inspected.SampleRate != result.Metadata.SampleRate || inspected.Frames != result.Metadata.Frames {
			t.Fatalf("SDK rejected processed audio: metadata=%+v error=%v", inspected, inspectErr)
		}
	}
}

func FuzzDecodeWAV(f *testing.F) {
	f.Add([]byte("not a wave"))
	f.Add(buildWAVForFuzz(testWavePCM, 16, 1, 48_000, encodeIntegers(16, -1, 0, 1)))
	f.Fuzz(func(t *testing.T, source []byte) {
		_, _ = assetpipeline.DecodeWAV(source)
	})
}

type wavChunk struct {
	id   string
	data []byte
}

type wavOptions struct {
	encoding            uint16
	extensibleSubformat uint16
	bits                uint16
	channels            uint16
	sampleRate          uint32
	data                []byte
	beforeFormat        []wavChunk
	duplicateFormat     bool
	duplicateData       bool
}

func buildWAV(t *testing.T, options wavOptions) []byte {
	t.Helper()

	bytesPerSample := options.bits / 8
	blockAlign := options.channels * bytesPerSample

	formatSize := 16
	if options.encoding == testWaveExtensible {
		formatSize = 40
	}

	format := make([]byte, formatSize)
	binary.LittleEndian.PutUint16(format[0:2], options.encoding)
	binary.LittleEndian.PutUint16(format[2:4], options.channels)
	binary.LittleEndian.PutUint32(format[4:8], options.sampleRate)
	binary.LittleEndian.PutUint32(format[8:12], options.sampleRate*uint32(blockAlign))
	binary.LittleEndian.PutUint16(format[12:14], blockAlign)
	binary.LittleEndian.PutUint16(format[14:16], options.bits)

	if options.encoding == testWaveExtensible {
		binary.LittleEndian.PutUint16(format[16:18], 22)
		binary.LittleEndian.PutUint16(format[18:20], options.bits)
		guid := []byte{byte(options.extensibleSubformat), 0, 0, 0, 0, 0, 0x10, 0, 0x80, 0, 0, 0xaa, 0, 0x38, 0x9b, 0x71}
		copy(format[24:40], guid)
	}

	chunks := append([]wavChunk(nil), options.beforeFormat...)

	chunks = append(chunks, wavChunk{id: "fmt ", data: format})
	if options.duplicateFormat {
		chunks = append(chunks, wavChunk{id: "fmt ", data: format})
	}

	chunks = append(chunks, wavChunk{id: "data", data: options.data})
	if options.duplicateData {
		chunks = append(chunks, wavChunk{id: "data", data: options.data})
	}

	var body bytes.Buffer
	body.WriteString("WAVE")

	for _, chunk := range chunks {
		if len(chunk.id) != 4 {
			t.Fatalf("invalid test chunk ID %q", chunk.id)
		}

		body.WriteString(chunk.id)

		if err := binary.Write(&body, binary.LittleEndian, uint32(len(chunk.data))); err != nil {
			t.Fatal(err)
		}

		body.Write(chunk.data)

		if len(chunk.data)%2 != 0 {
			body.WriteByte(0)
		}
	}

	var output bytes.Buffer
	output.WriteString("RIFF")

	if err := binary.Write(&output, binary.LittleEndian, uint32(body.Len())); err != nil {
		t.Fatal(err)
	}

	output.Write(body.Bytes())

	return output.Bytes()
}

func buildWAVForFuzz(encoding, bits, channels uint16, sampleRate uint32, data []byte) []byte {
	blockAlign := channels * (bits / 8)
	format := make([]byte, 16)
	binary.LittleEndian.PutUint16(format[0:2], encoding)
	binary.LittleEndian.PutUint16(format[2:4], channels)
	binary.LittleEndian.PutUint32(format[4:8], sampleRate)
	binary.LittleEndian.PutUint32(format[8:12], sampleRate*uint32(blockAlign))
	binary.LittleEndian.PutUint16(format[12:14], blockAlign)
	binary.LittleEndian.PutUint16(format[14:16], bits)

	body := make([]byte, 4+8+len(format)+8+len(data))
	copy(body[:4], "WAVE")
	copy(body[4:8], "fmt ")
	binary.LittleEndian.PutUint32(body[8:12], uint32(len(format)))
	copy(body[12:28], format)
	copy(body[28:32], "data")
	binary.LittleEndian.PutUint32(body[32:36], uint32(len(data)))
	copy(body[36:], data)

	output := make([]byte, 8+len(body))
	copy(output[:4], "RIFF")
	binary.LittleEndian.PutUint32(output[4:8], uint32(len(body)))
	copy(output[8:], body)

	return output
}

func encodeIntegers(bits uint16, values ...int64) []byte {
	output := make([]byte, len(values)*int(bits/8))
	for index, value := range values {
		offset := index * int(bits/8)
		switch bits {
		case 16:
			binary.LittleEndian.PutUint16(output[offset:], uint16(value))
		case 24:
			output[offset] = byte(value)
			output[offset+1] = byte(value >> 8)
			output[offset+2] = byte(value >> 16)
		case 32:
			binary.LittleEndian.PutUint32(output[offset:], uint32(value))
		default:
			panic("unsupported test bit depth")
		}
	}

	return output
}

func encodeFloats(values ...float32) []byte {
	output := make([]byte, len(values)*4)
	for index, value := range values {
		binary.LittleEndian.PutUint32(output[index*4:], math.Float32bits(value))
	}

	return output
}

func encodePCM16(samples []int16) []byte {
	output := make([]byte, len(samples)*2)
	for index, sample := range samples {
		binary.LittleEndian.PutUint16(output[index*2:], uint16(sample))
	}

	return output
}
