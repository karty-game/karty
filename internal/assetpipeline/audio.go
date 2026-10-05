package assetpipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/resample"
	"github.com/karty-game/karty-sdk/codec/qoa"
	"github.com/karty-game/karty-sdk/format/asset"
)

const (
	waveFormatPCM            = 0x0001
	waveFormatIEEEFloat      = 0x0003
	waveFormatExtensible     = 0xfffe
	waveFormatBaseSize       = 16
	waveFormatExtendedSize   = 18
	waveFormatExtensibleSize = 40
	waveSubformatOffset      = 24
	waveExtensibleExtraSize  = 22
	waveHeaderSize           = 12
	waveChunkHeaderSize      = 8
	waveRIFFSizeFieldBytes   = 8
	waveChunkAlignment       = 2
	waveMonoChannels         = 1
	waveStereoChannels       = 2
	waveBitDepth8            = 8
	waveBitDepth16           = 16
	waveBitDepth24           = 24
	waveBitDepth32           = 32
	bitsPerByte              = 8
	pcm16BytesPerSample      = 2
	pcm16Scale               = 32_768
	pcm8Zero                 = 128
	audioRecipeCapacity      = 128
	maxSourceSampleRate      = 384_000
	AudioEncodingQOA         = "qoa"
	wavNormalizerRevision    = "wav-pcm16@1"
	resamplerRevision        = "cwbudde/algo-dsp:quality-best@v0.7.1"
	waveSubformatPCM         = "\x01\x00\x00\x00\x00\x00\x10\x00\x80\x00\x00\xaa\x00\x38\x9b\x71"
	waveSubformatFloat       = "\x03\x00\x00\x00\x00\x00\x10\x00\x80\x00\x00\xaa\x00\x38\x9b\x71"
)

var (
	ErrInvalidWAV              = errors.New("invalid WAV")
	ErrAudioBounds             = errors.New("audio exceeds processing bounds")
	ErrAudioOutput             = errors.New("encoded QOA output failed validation")
	errResampledChannelLengths = errors.New("resample audio: channel lengths differ")
)

// PCM16 is bounded, interleaved signed PCM ready for QOA encoding.
type PCM16 struct {
	SampleRate     uint32
	Channels       uint16
	Frames         uint32
	Samples        []int16
	SourceBitDepth uint16
	SourceFloat    bool
}

// AudioMetadata records source and processed audio properties for reports and
// validated cache metadata.
type AudioMetadata struct {
	SourceSampleRate uint32 `json:"sourceSampleRate"`
	SourceChannels   uint16 `json:"sourceChannels"`
	SourceFrames     uint32 `json:"sourceFrames"`
	SourceBitDepth   uint16 `json:"sourceBitDepth"`
	SourceFloat      bool   `json:"sourceFloat"`
	SampleRate       uint32 `json:"sampleRate"`
	Channels         uint8  `json:"channels"`
	Frames           uint32 `json:"frames"`
	DecodedBytes     uint64 `json:"decodedBytes"`
}

// ProcessedAudio is an owned deterministic qoa@1 processor result.
type ProcessedAudio struct {
	Encoding     string
	EncodedQOA   []byte
	SourceBytes  uint64
	OutputBytes  uint64
	SourceDigest [sha256.Size]byte
	OutputDigest [sha256.Size]byte
	RecipeDigest [sha256.Size]byte
	Metadata     AudioMetadata
}

type waveFormat struct {
	encoding      uint16
	channels      uint16
	sampleRate    uint32
	byteRate      uint32
	blockAlign    uint16
	bitsPerSample uint16
}

// ProcessAudio snapshots WAV source bytes, applies the canonical qoa@1
// transform recipe, encodes QOA through the hardened public SDK adapter, and
// validates the complete output before returning it.
func ProcessAudio(source []byte, recipe asset.AudioRecipe) (ProcessedAudio, error) {
	return processAudio(source, recipe, asset.MaxSoundDurationSeconds, asset.MaxDecodedSoundBytes, false)
}

// ProcessStreamAudio encodes long-form WAV input while retaining the same
// deterministic transform as sound effects. Source bytes remain bounded by the
// public asset contract; decoded duration uses the streaming-audio limit.
func ProcessStreamAudio(source []byte, recipe asset.AudioRecipe) (ProcessedAudio, error) {
	return processAudio(source, recipe, asset.MaxAudioStreamDurationSeconds, uint64(asset.MaxSourceAssetBytes)*4, true)
}

func processAudio(
	source []byte,
	recipe asset.AudioRecipe,
	maxDuration uint64,
	maxDecodedBytes uint64,
	stream bool,
) (ProcessedAudio, error) {
	if len(source) > asset.MaxSourceAssetBytes {
		return ProcessedAudio{}, ErrAudioBounds
	}

	if err := recipe.Validate(); err != nil {
		return ProcessedAudio{}, fmt.Errorf("validate audio recipe: %w", err)
	}

	snapshot := bytes.Clone(source)

	sourceFormat, sourceData, err := inspectWAV(snapshot)
	if err != nil {
		return ProcessedAudio{}, err
	}

	processedPCM, err := transformWAV(snapshot, recipe, maxDuration, maxDecodedBytes)
	if err != nil {
		return ProcessedAudio{}, err
	}

	var encoded []byte

	var encodedMetadata qoa.Metadata
	if stream {
		encoded, encodedMetadata, err = qoa.EncodeStream(processedPCM.Samples, uint8(processedPCM.Channels), processedPCM.SampleRate)
	} else {
		encoded, encodedMetadata, err = qoa.Encode(processedPCM.Samples, uint8(processedPCM.Channels), processedPCM.SampleRate)
	}

	if err != nil {
		return ProcessedAudio{}, fmt.Errorf("encode QOA: %w", ErrAudioOutput)
	}

	var inspected qoa.Metadata
	if stream {
		inspected, err = qoa.InspectStream(encoded)
	} else {
		inspected, err = qoa.Inspect(encoded)
	}

	if err != nil || inspected != encodedMetadata || inspected.Channels != uint8(processedPCM.Channels) ||
		inspected.SampleRate != processedPCM.SampleRate || inspected.Frames != processedPCM.Frames ||
		inspected.DecodedBytes != uint64(len(processedPCM.Samples))*pcm16BytesPerSample {
		return ProcessedAudio{}, ErrAudioOutput
	}

	return ProcessedAudio{
		Encoding:     AudioEncodingQOA,
		EncodedQOA:   encoded,
		SourceBytes:  uint64(len(snapshot)),
		OutputBytes:  uint64(len(encoded)),
		SourceDigest: sha256.Sum256(snapshot),
		OutputDigest: sha256.Sum256(encoded),
		RecipeDigest: digestAudioRecipe(recipe),
		Metadata: AudioMetadata{
			SourceSampleRate: sourceFormat.sampleRate,
			SourceChannels:   sourceFormat.channels,
			SourceFrames:     uint32(len(sourceData) / int(sourceFormat.blockAlign)),
			SourceBitDepth:   sourceFormat.bitsPerSample,
			SourceFloat:      sourceFormat.encoding == waveFormatIEEEFloat,
			SampleRate:       inspected.SampleRate,
			Channels:         inspected.Channels,
			Frames:           inspected.Frames,
			DecodedBytes:     inspected.DecodedBytes,
		},
	}, nil
}

// DecodeWAV parses a bounded RIFF/WAVE source and normalizes supported samples
// to deterministic signed PCM16. It accepts mono/stereo integer PCM at
// 8/16/24/32 bits and IEEE float32, including their exact extensible GUIDs.
func DecodeWAV(source []byte) (PCM16, error) {
	return decodeWAV(source, asset.MaxSoundDurationSeconds, asset.MaxDecodedSoundBytes)
}

func decodeWAV(source []byte, maxDuration uint64, maxDecodedBytes uint64) (PCM16, error) {
	format, data, err := inspectWAV(source)
	if err != nil {
		return PCM16{}, err
	}

	frames := uint64(len(data)) / uint64(format.blockAlign)
	sampleCount := frames * uint64(format.channels)

	decodedBytes := sampleCount * pcm16BytesPerSample
	if frames == 0 || frames > uint64(format.sampleRate)*maxDuration ||
		decodedBytes > maxDecodedBytes || sampleCount > uint64(^uint(0)>>1) {
		return PCM16{}, ErrAudioBounds
	}

	samples := make([]int16, int(sampleCount))

	bytesPerSample := int(format.bitsPerSample / bitsPerByte)
	for index := range samples {
		offset := index * bytesPerSample

		value, decodeErr := decodeWAVSample(data[offset:offset+bytesPerSample], format)
		if decodeErr != nil {
			return PCM16{}, decodeErr
		}

		samples[index] = value
	}

	return PCM16{
		SampleRate:     format.sampleRate,
		Channels:       format.channels,
		Frames:         uint32(frames),
		Samples:        samples,
		SourceBitDepth: format.bitsPerSample,
		SourceFloat:    format.encoding == waveFormatIEEEFloat,
	}, nil
}

// TransformWAV decodes a WAV and applies the resolved qoa@1 audio recipe.
// Explicit mono conversion is performed before QualityBest resampling.
func TransformWAV(source []byte, recipe asset.AudioRecipe) (PCM16, error) {
	return transformWAV(source, recipe, asset.MaxSoundDurationSeconds, asset.MaxDecodedSoundBytes)
}

func transformWAV(source []byte, recipe asset.AudioRecipe, maxDuration uint64, maxDecodedBytes uint64) (PCM16, error) {
	if err := recipe.Validate(); err != nil {
		return PCM16{}, fmt.Errorf("audio recipe: %w", err)
	}

	pcm, err := decodeWAV(source, maxDuration, maxDecodedBytes)
	if err != nil {
		return PCM16{}, err
	}

	if recipe.ChannelMode == asset.ChannelMono && pcm.Channels == waveStereoChannels {
		mono := make([]int16, pcm.Frames)
		for frame := range pcm.Frames {
			sum := int64(pcm.Samples[int(frame)*2]) + int64(pcm.Samples[int(frame)*2+1])
			mono[frame] = int16(roundRatioAwayFromZero(sum, waveStereoChannels))
		}

		pcm.Samples = mono
		pcm.Channels = waveMonoChannels
	}

	if pcm.SampleRate == recipe.SampleRate {
		return pcm, nil
	}

	resampled, err := resamplePCM16(pcm.Samples, pcm.Channels, pcm.SampleRate, recipe.SampleRate)
	if err != nil {
		return PCM16{}, err
	}

	frames := len(resampled) / int(pcm.Channels)
	if uint64(frames)*uint64(pcm.Channels)*pcm16BytesPerSample > maxDecodedBytes ||
		uint64(frames) > uint64(recipe.SampleRate)*maxDuration {
		return PCM16{}, ErrAudioBounds
	}

	pcm.SampleRate = recipe.SampleRate
	pcm.Frames = uint32(frames)
	pcm.Samples = resampled

	return pcm, nil
}

func inspectWAV(source []byte) (waveFormat, []byte, error) {
	if len(source) > asset.MaxSourceAssetBytes {
		return waveFormat{}, nil, ErrAudioBounds
	}

	if len(source) < waveHeaderSize {
		return waveFormat{}, nil, fmt.Errorf("%w: truncated RIFF header", ErrInvalidWAV)
	}

	if string(source[:4]) != "RIFF" {
		return waveFormat{}, nil, fmt.Errorf("%w: only little-endian RIFF is supported", ErrInvalidWAV)
	}

	if string(source[8:12]) != "WAVE" {
		return waveFormat{}, nil, fmt.Errorf("%w: RIFF form is not WAVE", ErrInvalidWAV)
	}

	riffEnd := uint64(binary.LittleEndian.Uint32(source[4:8])) + waveRIFFSizeFieldBytes
	if riffEnd != uint64(len(source)) || riffEnd < waveHeaderSize {
		return waveFormat{}, nil, fmt.Errorf("%w: inconsistent RIFF size", ErrInvalidWAV)
	}

	var (
		format waveFormat
		data   []byte
	)

	seenFormat, seenData := false, false

	for offset := uint64(waveHeaderSize); offset < riffEnd; {
		if offset+waveChunkHeaderSize > riffEnd {
			return waveFormat{}, nil, fmt.Errorf("%w: truncated chunk header", ErrInvalidWAV)
		}

		chunkSize := uint64(binary.LittleEndian.Uint32(source[offset+4 : offset+8]))
		payloadStart := offset + waveChunkHeaderSize
		payloadEnd := payloadStart + chunkSize

		paddedEnd := payloadEnd + chunkSize%waveChunkAlignment
		if payloadEnd < payloadStart || paddedEnd < payloadEnd || paddedEnd > riffEnd {
			return waveFormat{}, nil, fmt.Errorf("%w: chunk exceeds RIFF bounds", ErrInvalidWAV)
		}

		chunk := source[payloadStart:payloadEnd]
		switch string(source[offset : offset+4]) {
		case "fmt ":
			if seenFormat {
				return waveFormat{}, nil, fmt.Errorf("%w: duplicate fmt chunk", ErrInvalidWAV)
			}

			var err error

			format, err = parseWAVFormat(chunk)
			if err != nil {
				return waveFormat{}, nil, err
			}

			seenFormat = true
		case "data":
			if seenData {
				return waveFormat{}, nil, fmt.Errorf("%w: duplicate data chunk", ErrInvalidWAV)
			}

			seenData = true
			data = chunk
		}

		offset = paddedEnd
	}

	if !seenFormat || !seenData {
		return waveFormat{}, nil, fmt.Errorf("%w: missing fmt or data chunk", ErrInvalidWAV)
	}

	if len(data)%int(format.blockAlign) != 0 {
		return waveFormat{}, nil, fmt.Errorf("%w: data is not frame-aligned", ErrInvalidWAV)
	}

	return format, data, nil
}

func parseWAVFormat(chunk []byte) (waveFormat, error) {
	if len(chunk) < waveFormatBaseSize {
		return waveFormat{}, fmt.Errorf("%w: truncated fmt chunk", ErrInvalidWAV)
	}

	format := waveFormat{
		encoding:      binary.LittleEndian.Uint16(chunk[0:2]),
		channels:      binary.LittleEndian.Uint16(chunk[2:4]),
		sampleRate:    binary.LittleEndian.Uint32(chunk[4:8]),
		byteRate:      binary.LittleEndian.Uint32(chunk[8:12]),
		blockAlign:    binary.LittleEndian.Uint16(chunk[12:14]),
		bitsPerSample: binary.LittleEndian.Uint16(chunk[14:16]),
	}
	if format.encoding != waveFormatExtensible && len(chunk) > waveFormatBaseSize {
		if len(chunk) < waveFormatExtendedSize || int(binary.LittleEndian.Uint16(chunk[16:18]))+waveFormatExtendedSize > len(chunk) {
			return waveFormat{}, fmt.Errorf("%w: malformed extended fmt chunk", ErrInvalidWAV)
		}
	}

	if format.encoding == waveFormatExtensible {
		if len(chunk) < waveFormatExtensibleSize || binary.LittleEndian.Uint16(chunk[16:18]) < waveExtensibleExtraSize ||
			int(binary.LittleEndian.Uint16(chunk[16:18]))+waveFormatExtendedSize > len(chunk) {
			return waveFormat{}, fmt.Errorf("%w: malformed extensible fmt chunk", ErrInvalidWAV)
		}

		if binary.LittleEndian.Uint16(chunk[18:20]) != format.bitsPerSample {
			return waveFormat{}, fmt.Errorf("%w: extensible valid bits differ from container bits", ErrInvalidWAV)
		}

		switch string(chunk[waveSubformatOffset:waveFormatExtensibleSize]) {
		case waveSubformatPCM:
			format.encoding = waveFormatPCM
		case waveSubformatFloat:
			format.encoding = waveFormatIEEEFloat
		default:
			return waveFormat{}, fmt.Errorf("%w: unsupported extensible subformat", ErrInvalidWAV)
		}
	}

	if format.channels != waveMonoChannels && format.channels != waveStereoChannels {
		return waveFormat{}, fmt.Errorf("%w: only mono and stereo are supported", ErrInvalidWAV)
	}

	if format.sampleRate == 0 || format.sampleRate > maxSourceSampleRate {
		return waveFormat{}, fmt.Errorf("%w: unsupported sample rate", ErrInvalidWAV)
	}

	if (format.encoding != waveFormatPCM ||
		(format.bitsPerSample != waveBitDepth8 && format.bitsPerSample != waveBitDepth16 &&
			format.bitsPerSample != waveBitDepth24 && format.bitsPerSample != waveBitDepth32)) &&
		(format.encoding != waveFormatIEEEFloat || format.bitsPerSample != waveBitDepth32) {
		return waveFormat{}, fmt.Errorf("%w: unsupported encoding or bit depth", ErrInvalidWAV)
	}

	wantBlockAlign := uint64(format.channels) * uint64(format.bitsPerSample) / bitsPerByte

	wantByteRate := uint64(format.sampleRate) * wantBlockAlign
	if wantBlockAlign == 0 || uint64(format.blockAlign) != wantBlockAlign || uint64(format.byteRate) != wantByteRate {
		return waveFormat{}, fmt.Errorf("%w: inconsistent block alignment or byte rate", ErrInvalidWAV)
	}

	return format, nil
}

func decodeWAVSample(encoded []byte, format waveFormat) (int16, error) {
	if format.encoding == waveFormatIEEEFloat {
		value := math.Float32frombits(binary.LittleEndian.Uint32(encoded))
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return 0, fmt.Errorf("%w: non-finite floating-point sample", ErrInvalidWAV)
		}

		return normalizedFloatToPCM16(float64(value)), nil
	}

	switch format.bitsPerSample {
	case waveBitDepth8:
		return int16((int32(encoded[0]) - pcm8Zero) << 8), nil
	case waveBitDepth16:
		return int16(binary.LittleEndian.Uint16(encoded)), nil
	case waveBitDepth24:
		value := int64(int32(encoded[0]) | int32(encoded[1])<<8 | int32(encoded[2])<<16)
		if value&0x800000 != 0 {
			value |= ^int64(0xffffff)
		}

		return int16(clampPCM16(roundRatioAwayFromZero(value, 1<<8))), nil
	case waveBitDepth32:
		value := int64(int32(binary.LittleEndian.Uint32(encoded)))

		return int16(clampPCM16(roundRatioAwayFromZero(value, 1<<16))), nil
	default:
		panic("validated WAV bit depth is unreachable")
	}
}

func normalizedFloatToPCM16(value float64) int16 {
	if value <= -1 {
		return math.MinInt16
	}

	if value >= 1 {
		return math.MaxInt16
	}

	return int16(clampPCM16(int64(math.Round(value * pcm16Scale))))
}

func roundRatioAwayFromZero(value, divisor int64) int64 {
	if value < 0 {
		return -((-value + divisor/2) / divisor)
	}

	return (value + divisor/2) / divisor
}

func clampPCM16(value int64) int64 {
	return min(max(value, math.MinInt16), math.MaxInt16)
}

func resamplePCM16(samples []int16, channels uint16, inputRate, outputRate uint32) ([]int16, error) {
	channelOutput := make([][]float64, channels)
	for channel := range int(channels) {
		input := make([]float64, len(samples)/int(channels))
		for frame := range input {
			input[frame] = float64(samples[frame*int(channels)+channel]) / pcm16Scale
		}

		output, err := resample.Resample(input, int(outputRate), int(inputRate), resample.WithQuality(resample.QualityBest))
		if err != nil {
			return nil, fmt.Errorf("resample audio: %w", err)
		}

		channelOutput[channel] = output
	}

	frames := len(channelOutput[0])

	output := make([]int16, frames*int(channels))
	for channel := range int(channels) {
		if len(channelOutput[channel]) != frames {
			return nil, errResampledChannelLengths
		}

		for frame, sample := range channelOutput[channel] {
			output[frame*int(channels)+channel] = normalizedFloatToPCM16(sample)
		}
	}

	return output, nil
}

func digestAudioRecipe(recipe asset.AudioRecipe) [sha256.Size]byte {
	canonical := make([]byte, 0, audioRecipeCapacity)
	appendField := func(value string) {
		var length [4]byte
		binary.BigEndian.PutUint32(length[:], uint32(len(value)))
		canonical = append(canonical, length[:]...)
		canonical = append(canonical, value...)
	}

	appendField("karty-audio-recipe-v1")
	appendField(string(asset.ProcessorQOAv1))

	var sampleRate [4]byte

	binary.BigEndian.PutUint32(sampleRate[:], recipe.SampleRate)
	canonical = append(canonical, sampleRate[:]...)

	appendField(string(recipe.ChannelMode))
	appendField(wavNormalizerRevision)
	appendField(resamplerRevision)

	return sha256.Sum256(canonical)
}
