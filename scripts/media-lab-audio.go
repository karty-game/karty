//go:build ignore

// Master the retained demonstration sources without changing their sound design.
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty/internal/assetpipeline"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	check := flag.Bool("check", false, "verify mastered WAVs without writing")
	flag.Parse()
	const root = "samples/media-lab"
	source, err := os.ReadFile(filepath.Join(root, "sound-sources/loop.wav"))
	must(err)
	pcm, err := assetpipeline.DecodeWAV(source)
	must(err)
	// Periodic padding provides the filter's real history/lookahead. Retain
	// only the middle cycle, with no repeat-time fade or loss of music duration.
	tiled := make([]int16, 0, len(pcm.Samples)*3)
	for range 3 {
		tiled = append(tiled, pcm.Samples...)
	}
	converted, err := assetpipeline.TransformWAV(
		wav(tiled, pcm.SampleRate, pcm.Channels),
		asset.AudioRecipe{SampleRate: 48_000, ChannelMode: asset.ChannelPreserve},
	)
	must(err)
	cycle := len(converted.Samples) / 3
	master := converted.Samples[cycle : cycle*2]
	for index := range master {
		master[index] /= 2
	}
	music := wav(master, 48_000, pcm.Channels)
	source, err = os.ReadFile(filepath.Join(root, "sound-sources/machine-gun.wav"))
	must(err)
	metadata, err := assetpipeline.DecodeWAV(source)
	must(err)
	if metadata.SourceBitDepth != 24 || metadata.Channels != 1 {
		panic("expected the retained PCM24 mono effect")
	}
	effect := bytes.Clone(source)
	for offset := 12; offset < len(effect); {
		size := int(binary.LittleEndian.Uint32(effect[offset+4:]))
		start := offset + 8
		if string(effect[offset:offset+4]) == "data" {
			for pos := start; pos < start+size; pos += 3 {
				value := int32(effect[pos]) | int32(effect[pos+1])<<8 | int32(effect[pos+2])<<16
				value = (value << 8 >> 8) / 40
				for shift := range 3 {
					effect[pos+shift] = byte(uint32(value) >> uint(8*shift))
				}
			}
		}
		offset = start + size + size%2
	}
	for path, data := range map[string][]byte{"assets/sounds/music/loop.wav": music, "assets/sounds/effects/machine-gun.wav": effect} {
		path = filepath.Join(root, path)
		if *check {
			current, err := os.ReadFile(path)
			must(err)
			if !bytes.Equal(current, data) {
				panic(fmt.Sprintf("%s is stale; run mise run generate-media-lab-audio", path))
			}
		} else {
			must(os.WriteFile(path, data, 0644))
		}
	}
}

func wav(samples []int16, rate uint32, channels uint16) []byte {
	data := make([]byte, 44+len(samples)*2)
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(len(data)-8))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], channels)
	binary.LittleEndian.PutUint32(data[24:], rate)
	binary.LittleEndian.PutUint32(data[28:], rate*uint32(channels)*2)
	binary.LittleEndian.PutUint16(data[32:], channels*2)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], uint32(len(data)-44))
	for i, sample := range samples {
		binary.LittleEndian.PutUint16(data[44+i*2:], uint16(sample))
	}
	return data
}
