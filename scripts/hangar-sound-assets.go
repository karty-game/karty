//go:build ignore

// Prepare mono Hangar loops from retained CC0 field recordings. This offline
// processor uses only the standard library and adds no synthesized sound.
package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

const sampleRate = 48000

var errInvalidWAV = errors.New("expected a nonempty PCM16 mono/stereo RIFF/WAVE recording")

type loopSpec struct {
	name, source                   string
	start, seconds, crossfade, rms float64
}

func main() {
	check := flag.Bool("check", false, "verify authored sound assets without writing")
	flag.Parse()
	if err := generate(*check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(check bool) error {
	for _, clip := range []loopSpec{
		{"reactor-hum", "reactor", -1, 30, .25, .12},
		{"computer-idle", "computer", -1, 30, .25, .035},
		// The original cinderblock scrape is 1.285 seconds; its sustained friction
		// lies between .2 and 1 second. Overlap leaves a .72-second playback loop.
		{"crate-scrape", "crate", .2, .8, .08, .10},
		{"fire-ambience", "fire", -1, 30, .25, .08},
		{"acid-ambience", "acid", -1, 30, .25, .07},
	} {
		source := filepath.Join("samples/hangar/sound-sources", clip.source+".wav")
		original, err := os.ReadFile(source)
		if err != nil {
			return fmt.Errorf("read retained recording %s: %w", source, err)
		}
		samples, rate, err := decodeWAV(original)
		if err != nil {
			return fmt.Errorf("decode retained recording %s: %w", source, err)
		}
		samples, err = prepareLoop(samples, rate, clip)
		if err != nil {
			return fmt.Errorf("prepare %s: %w", clip.name, err)
		}
		data := wav(samples)
		path := filepath.Join("samples/hangar/assets/sounds", clip.name+".wav")
		if check {
			current, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(current, data) {
				return fmt.Errorf("%s is stale; run mise run generate-hangar-sounds", path)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return err
		}
		fmt.Printf("%s: %.3fs mono @ %d Hz, RMS %.4f, peak %.4f\n", path,
			float64(len(samples))/sampleRate, sampleRate, signalRMS(samples), peak(samples))
	}
	return nil
}

// decodeWAV walks RIFF chunks, including odd-sized metadata and data before
// fmt. It rejects compressed/extensible encodings, truncation and partial frames.
func decodeWAV(data []byte) ([]float64, int, error) {
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, 0, errInvalidWAV
	}
	end := uint64(binary.LittleEndian.Uint32(data[4:8])) + 8
	if end != uint64(len(data)) {
		return nil, 0, errInvalidWAV
	}
	var format, pcm []byte
	for offset := uint64(12); offset < end; {
		if end-offset < 8 {
			return nil, 0, errInvalidWAV
		}
		size := uint64(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		next := offset + 8 + size
		if next > end {
			return nil, 0, errInvalidWAV
		}
		chunk := data[offset+8 : next]
		switch string(data[offset : offset+4]) {
		case "fmt ":
			if format != nil {
				return nil, 0, errInvalidWAV
			}
			format = chunk
		case "data":
			if pcm != nil {
				return nil, 0, errInvalidWAV
			}
			pcm = chunk
		}
		offset = next + (size & 1)
		if offset > end {
			return nil, 0, errInvalidWAV
		}
	}
	if len(format) < 16 || binary.LittleEndian.Uint16(format[:2]) != 1 || binary.LittleEndian.Uint16(format[14:16]) != 16 {
		return nil, 0, errInvalidWAV
	}
	channels := int(binary.LittleEndian.Uint16(format[2:4]))
	rate := int(binary.LittleEndian.Uint32(format[4:8]))
	if (channels != 1 && channels != 2) || rate < 8000 || rate > 192000 ||
		int(binary.LittleEndian.Uint16(format[12:14])) != channels*2 ||
		uint64(binary.LittleEndian.Uint32(format[8:12])) != uint64(rate)*uint64(channels)*2 ||
		len(pcm) == 0 || len(pcm)%(channels*2) != 0 {
		return nil, 0, errInvalidWAV
	}
	samples := make([]float64, len(pcm)/(channels*2))
	for frame := range samples {
		for channel := range channels {
			offset := (frame*channels + channel) * 2
			samples[frame] += float64(int16(binary.LittleEndian.Uint16(pcm[offset:offset+2]))) / 32768 / float64(channels)
		}
	}
	return samples, rate, nil
}

func prepareLoop(samples []float64, rate int, clip loopSpec) ([]float64, error) {
	wanted := int(clip.seconds * float64(rate))
	start := int(clip.start * float64(rate))
	if clip.start < 0 {
		start = max(0, (len(samples)-wanted)/2)
	}
	if start >= len(samples)-4 {
		start = 0
	}
	count := min(wanted, len(samples)-start)
	if count < 4 {
		return nil, errors.New("recording is too short for a loop")
	}
	segment := resample(samples[start:start+count], rate)
	removeDC(segment)
	fade := min(int(clip.crossfade*sampleRate), len(segment)/4)
	if fade < 2 {
		return nil, errors.New("recording is too short for a crossfade")
	}
	// Rotate past the head and blend the original tail into that head. The
	// final blended frame joins head[fade] naturally across the wrap boundary;
	// this overlaps real recording material instead of adding silent fades.
	loop := make([]float64, len(segment)-fade)
	copied := copy(loop, segment[fade:len(segment)-fade])
	for frame := range fade {
		blend := float64(frame) / float64(fade-1)
		blend = blend * blend * (3 - 2*blend)
		loop[copied+frame] = segment[len(segment)-fade+frame]*(1-blend) + segment[frame]*blend
	}
	removeDC(loop)
	rms, maximum := signalRMS(loop), peak(loop)
	if rms < 1e-9 || maximum < 1e-9 {
		return nil, errors.New("selected recording region is silent")
	}
	gain := min(clip.rms/rms, .6/maximum)
	for frame := range loop {
		loop[frame] *= gain
	}
	return loop, nil
}

// Linear interpolation preserves the recording's character. A low-pass before
// downsampling reduces aliasing for recordings above the 48 kHz output rate.
func resample(samples []float64, rate int) []float64 {
	if rate > sampleRate {
		samples = append([]float64(nil), samples...)
		coefficient := 1 - math.Exp(-2*math.Pi*(sampleRate*.4)/float64(rate))
		for range 3 {
			filtered := samples[0]
			for frame, value := range samples {
				filtered += coefficient * (value - filtered)
				samples[frame] = filtered
			}
		}
	}
	frames := int(int64(len(samples)) * sampleRate / int64(rate))
	output := make([]float64, frames)
	for frame := range output {
		position := float64(frame) * float64(rate) / sampleRate
		index := int(position)
		fraction := position - float64(index)
		output[frame] = samples[index]*(1-fraction) + samples[min(index+1, len(samples)-1)]*fraction
	}
	return output
}

func removeDC(samples []float64) {
	var sum float64
	for _, sample := range samples {
		sum += sample
	}
	mean := sum / float64(len(samples))
	for frame := range samples {
		samples[frame] -= mean
	}
}

func signalRMS(samples []float64) float64 {
	var sum float64
	for _, sample := range samples {
		sum += sample * sample
	}
	return math.Sqrt(sum / float64(len(samples)))
}

func peak(samples []float64) float64 {
	var maximum float64
	for _, sample := range samples {
		maximum = max(maximum, math.Abs(sample))
	}
	return maximum
}

func wav(samples []float64) []byte {
	data := make([]byte, 44+len(samples)*2)
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(data)-8))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:20], 16)
	binary.LittleEndian.PutUint16(data[20:22], 1)
	binary.LittleEndian.PutUint16(data[22:24], 1)
	binary.LittleEndian.PutUint32(data[24:28], sampleRate)
	binary.LittleEndian.PutUint32(data[28:32], sampleRate*2)
	binary.LittleEndian.PutUint16(data[32:34], 2)
	binary.LittleEndian.PutUint16(data[34:36], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:44], uint32(len(samples)*2))
	for frame, sample := range samples {
		binary.LittleEndian.PutUint16(data[44+frame*2:], uint16(int16(math.Round(sample*32767))))
	}
	return data
}
