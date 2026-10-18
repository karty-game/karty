//go:build ignore

// Generate train Foley. The sampled acoustic score lives in last-train-score.py.
package main

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
)

const sampleRate = 48000

type stereo struct{ left, right float64 }

// Preserve the original Foley stream after separating the retired music generator.
var noiseState uint32 = 1574175314

func noise() float64 {
	noiseState ^= noiseState << 13
	noiseState ^= noiseState >> 17
	noiseState ^= noiseState << 5
	return float64(noiseState)/2147483648 - 1
}
func frequency(note int) float64 { return 440 * math.Exp2(float64(note-69)/12) }
func tone(samples []stereo, start, duration float64, note int, volume, pan float64, kind int) {
	begin, end := int(start*sampleRate), min(len(samples), int((start+duration)*sampleRate))
	f := frequency(note)
	for index := begin; index < end; index++ {
		t := float64(index-begin) / sampleRate
		phase := 2 * math.Pi * f * t
		envelope := math.Min(1, t/.012) * math.Min(1, (duration-t)/.08)
		value := 0.0
		switch kind {
		case 0:
			value = (math.Sin(phase) + .32*math.Sin(2*phase) + .13*math.Sin(3*phase)) * math.Exp(-3.5*t)
		case 1:
			value = (math.Sin(phase) + .3*math.Sin(2*phase)) * math.Exp(-7*t)
		case 2:
			vibrato := .012 * math.Sin(2*math.Pi*5*t) * math.Min(1, t/.3)
			value = math.Sin(phase+vibrato) + .22*math.Sin(3*phase+vibrato) + .08*math.Sin(5*phase)
		}
		value *= volume * envelope
		samples[index].left += value * (1 - pan) * .5
		samples[index].right += value * (1 + pan) * .5
	}
}
func write(name string, samples []stereo) {
	file, err := os.Create(filepath.Join("samples/last-train/assets/audio", name+".wav"))
	if err != nil {
		panic(err)
	}
	defer file.Close()
	data := make([]byte, 44+len(samples)*4)
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(len(data)-8))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], 2)
	binary.LittleEndian.PutUint32(data[24:], sampleRate)
	binary.LittleEndian.PutUint32(data[28:], sampleRate*4)
	binary.LittleEndian.PutUint16(data[32:], 4)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], uint32(len(samples)*4))
	for index, sample := range samples {
		for channel, value := range []float64{sample.left, sample.right} {
			value = math.Tanh(value) * .7
			binary.LittleEndian.PutUint16(data[44+index*4+channel*2:], uint16(int16(value*32767)))
		}
	}
	if _, err := file.Write(data); err != nil {
		panic(err)
	}
}
func main() {
	if err := os.MkdirAll("samples/last-train/assets/audio", 0750); err != nil {
		panic(err)
	}
	train := make([]stereo, 12*sampleRate)
	low := 0.0
	for index := range train {
		t := float64(index) / sampleRate
		low = .994*low + .006*noise()
		phase := math.Mod(t, .58)
		clack := math.Exp(-phase*150) + .65*math.Exp(-math.Mod(t+.11, .58)*160)
		v := low*.24 + noise()*.018 + clack*noise()*.095 + math.Sin(2*math.Pi*39*t)*.009
		edge := math.Min(1, t/.08) * math.Min(1, (12-t)/.08)
		train[index] = stereo{v * edge, (v*.88 + noise()*.005) * edge}
	}
	write("night-train", train)
	punch := make([]stereo, sampleRate/4)
	for index := range punch {
		t := float64(index) / sampleRate
		v := (noise()*.23 + math.Sin(2*math.Pi*1900*t)*.12) * math.Exp(-t*85)
		punch[index] = stereo{v, v}
	}
	write("ticket-punch", punch)
	whistle := make([]stereo, 2*sampleRate)
	tone(whistle, .04, .75, 79, .09, 0, 2)
	tone(whistle, .12, .65, 74, .035, 0, 2)
	tone(whistle, 1, .75, 76, .08, 0, 2)
	tone(whistle, 1.06, .68, 72, .025, 0, 2)
	write("station-whistle", whistle)
}
