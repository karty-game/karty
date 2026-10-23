//go:build ignore

// Original cartoon effects, synthesized with the Go standard library.
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

func main() {
	check := flag.Bool("check", false, "compare generated sounds without writing")
	flag.Parse()
	for _, clip := range []struct {
		name    string
		seconds float64
	}{{"shoot", .14}, {"blast", .65}, {"crate-hit", .24}} {
		data := synth(clip.name, clip.seconds)
		path := filepath.Join("samples/crate-room/assets/sounds", clip.name+".wav")
		if *check {
			actual, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(actual, data) {
				panic("stale sound: " + path)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				panic(err)
			}
			if err := os.WriteFile(path, data, 0644); err != nil {
				panic(err)
			}
		}
		fmt.Println(path)
	}
}

func synth(name string, seconds float64) []byte {
	const rate = 24000
	count := int(seconds * rate)
	data := make([]byte, 44+count*2)
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(len(data)-8))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], 1)
	binary.LittleEndian.PutUint32(data[24:], rate)
	binary.LittleEndian.PutUint32(data[28:], rate*2)
	binary.LittleEndian.PutUint16(data[32:], 2)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], uint32(count*2))
	state := uint32(1741)
	low, phase := 0.0, 0.0
	for i := range count {
		t := float64(i) / rate
		state ^= state << 13
		state ^= state >> 17
		state ^= state << 5
		noise := float64(state)/float64(math.MaxUint32)*2 - 1
		low += .12 * (noise - low)
		var value float64
		switch name {
		case "shoot":
			phase += 2 * math.Pi * (180 + 1250*math.Exp(-t*40)) / rate
			value = (.17*math.Sin(phase) + .15*noise) * math.Exp(-t*38)
		case "blast":
			phase += 2 * math.Pi * (42 + 130*math.Exp(-t*18)) / rate
			value = (.38*math.Sin(phase) + .8*low + .14*noise*math.Exp(-t*28)) * math.Exp(-t*8)
		case "crate-hit":
			value = (.3*math.Sin(2*math.Pi*173*t)+.15*math.Sin(2*math.Pi*287*t)+.1*math.Sin(2*math.Pi*431*t))*math.Exp(
				-t*32,
			) + .17*noise*math.Exp(
				-t*100,
			)
		}
		// Quiet PCM headroom, short attack and zero endpoints prevent clicks.
		value *= math.Min(1, t/.0015) * math.Min(1, (seconds-t-1.0/rate)/.015)
		value = math.Max(-.8, math.Min(.8, value))
		binary.LittleEndian.PutUint16(data[44+i*2:], uint16(int16(value*32767)))
	}
	return data
}
