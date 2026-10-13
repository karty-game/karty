//go:build ignore

// Prepare single-image Particle Lab assets. Three original CC0 masks are copied
// byte-for-byte; the auxiliary sprites are static procedural shapes. No time,
// sequence, video or flipbook is baked into an image. Run from the CLI root with
// mise run generate-particle-lab; use -check for offline drift validation.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
)

const assetDirectory = "samples/particle-lab/effects/textures"

var check bool

func main() {
	flag.BoolVar(&check, "check", false, "check static assets without writing")
	flag.Parse()
	checkStaticSources()
	if !check {
		if err := os.MkdirAll(assetDirectory, 0755); err != nil {
			panic(err)
		}
	}
	for _, source := range []struct{ file, name, sha string }{
		{"brackeys-smoke-07-a.png", "lab-smoke-mask", "7a2448e7c2fefc0888fd068c37ec991d44309ecdae8ca338cec5e63d7065b700"},
		{"brackeys-smoke-07-strong-a.png", "lab-smoke-strong-mask", "94006c02d2d163b0ae1049e937b260f166c4c4b3029e49183731412d0a922fe2"},
		{"brackeys-spotlight-01-a.png", "lab-glow", "95e02b339012aedc584815b14e5456fa1af9fb596dd9c8368f5a44a53bda9343"},
	} {
		path := filepath.Join(assetDirectory, source.file)
		data := readStaticSource("textures/"+source.file, 16<<20)
		if fmt.Sprintf("%x", sha256.Sum256(data)) != source.sha {
			panic("retained static CC0 source checksum mismatch: " + path)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			panic(err)
		}
		if cfg.Width != 512 || cfg.Height != 512 {
			panic("static mask must be original 512x512 image: " + path)
		}
		publish(source.name, data)
	}
	preparePixiFire()
	writeTexture("lab-fleck", 32, func(x, y float64) color.NRGBA {
		// Angular, slightly chipped incandescent fragments, rather than circles.
		r := math.Abs(x+0.22*y)/0.28 + math.Abs(y)/0.58
		return white((1 - smooth(0.62, 1, r)) * edge(x, y))
	})
	writeTexture("lab-streak", 64, func(x, y float64) color.NRGBA {
		// A sprite impression of a short spark; this is not a motion trail.
		return white(math.Exp(-180*x*x) * (1 - smooth(0.25, 0.92, math.Abs(y))) * (0.65 + 0.35*(1-y)/2))
	})
	writeTexture("lab-ring", 128, func(x, y float64) color.NRGBA {
		r, angle := math.Hypot(x, y), math.Atan2(y, x)
		broken := 0.2 + 0.8*smooth(-0.35, 0.2, math.Sin(3*angle+0.7)+0.38*math.Sin(7*angle))
		rim := math.Exp(-math.Pow((r-0.74)/0.021, 2))
		return white(rim*broken + 0.06*math.Exp(-math.Pow((r-0.74)/0.075, 2)))
	})
	writeTexture("lab-shard", 64, func(x, y float64) color.NRGBA {
		// A tapered angular energy splinter with one brighter edge.
		x += 0.14 * y
		width := 0.09 + 0.12*(1-y)/2
		body := (1 - smooth(width*0.55, width, math.Abs(x))) * (1 - smooth(0.45, 0.9, math.Abs(y)))
		return white(body * (0.4 + 0.6*smooth(-width, width, x)))
	})
	writeTexture("lab-flash", 128, func(x, y float64) color.NRGBA {
		r := math.Hypot(x, y)
		core := math.Exp(-85 * r * r)
		cross := math.Exp(-900*x*x-4.5*y*y) + math.Exp(-900*y*y-4.5*x*x)
		diagonal := math.Exp(-600*math.Pow(x-y, 2)-12*r*r) + math.Exp(-600*math.Pow(x+y, 2)-12*r*r)
		return white(math.Min(1, core+0.34*cross+0.14*diagonal) * edge(x, y))
	})

	for _, entry := range []struct {
		name string
		rgb  color.NRGBA
	}{
		{"room-floor", color.NRGBA{R: 64, G: 67, B: 72, A: 255}},
		{"room-wall", color.NRGBA{R: 51, G: 54, B: 59, A: 255}},
		{"room-ceiling", color.NRGBA{R: 39, G: 42, B: 47, A: 255}},
		{"smoke-room-wall", color.NRGBA{R: 148, G: 148, B: 148, A: 255}},
		{"smoke-room-floor", color.NRGBA{R: 128, G: 128, B: 128, A: 255}},
	} {
		writeTexture(entry.name, 8, func(_, _ float64) color.NRGBA { return entry.rgb })
	}
	fmt.Printf(
		"Particle Lab static assets: 3 pinned CC0 masks, 1 retained MIT tongue mask, 5 auxiliary sprites and 5 room textures (%s).\n",
		map[bool]string{true: "checked", false: "prepared"}[check],
	)
}

func preparePixiFire() {
	data := readStaticSource("sources/pixi/Fire.png", 16<<20)
	if fmt.Sprintf("%x", sha256.Sum256(data)) != "b7a330148b31ec5c52ba5dbeedc014f1e030b4462935f048f55d4aee46e96e25" {
		panic("retained Pixi fire source checksum mismatch")
	}
	source, err := png.Decode(bytes.NewReader(data))
	if err != nil || source.Bounds().Dx() != 258 || source.Bounds().Dy() != 51 {
		panic("retained Pixi fire source must be the original 258x51 PNG")
	}
	visible := image.Rectangle{}
	for y := range source.Bounds().Dy() {
		for x := range source.Bounds().Dx() {
			_, _, _, alpha := source.At(x, y).RGBA()
			if alpha != 0 {
				visible = visible.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if visible.Empty() || visible.Dx() > 128 || visible.Dy() > 128 {
		panic("retained Pixi mask exceeds its normalized static sprite canvas")
	}
	// Keep all visible source pixels and their original alpha. Only transparent
	// margins change so the GPU's required square billboard has a useful pivot.
	result := image.NewNRGBA(image.Rect(0, 0, 128, 128))
	offset := image.Pt((128-visible.Dx())/2, (128-visible.Dy())/2)
	draw.Draw(result, image.Rectangle{Min: offset, Max: offset.Add(visible.Size())}, source, visible.Min, draw.Src)
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, result); err != nil {
		panic(err)
	}
	publish("lab-fire-tongue", encoded.Bytes())
}

// Only the used flat sources/templates and their compact provenance are kept.
func checkStaticSources() {
	var manifest struct {
		Version    int
		Provenance struct {
			Repository, Commit, License, SourceSHA256 string
		}
		Assets []struct {
			File, SHA256, Texture string
			Bytes                 int64
		}
	}
	if err := json.Unmarshal(readStaticSource("sources/static-assets.json", 1<<20), &manifest); err != nil {
		panic(err)
	}
	if manifest.Version != 1 || len(manifest.Assets) != 15 || manifest.Provenance.License != "CC0-1.0" ||
		manifest.Provenance.Repository != "https://github.com/Brackeys/vfx-in-godot" ||
		manifest.Provenance.Commit != "23b58f272b828b0a894ed7ef72fbf9033ee9d177" ||
		manifest.Provenance.SourceSHA256 != "19c4994f065dc186fa034e99b865a2b19a377dddfd3228511cfae5ffb58172a0" {
		panic("static source provenance or file count changed")
	}
	seen := map[string]bool{}
	total, masks, templates := int64(0), 0, 0
	for _, asset := range manifest.Assets {
		if seen[asset.File] || asset.Bytes < 0 || asset.Bytes > 16<<20 || total > (128<<20)-asset.Bytes {
			panic("static source duplicate or byte bounds rejected")
		}
		seen[asset.File] = true
		total += asset.Bytes
		data := readStaticSource(asset.File, 16<<20)
		if int64(len(data)) != asset.Bytes || fmt.Sprintf("%x", sha256.Sum256(data)) != asset.SHA256 {
			panic("static source bytes changed: " + asset.File)
		}
		if asset.Texture != "" {
			cfg, err := png.DecodeConfig(bytes.NewReader(data))
			if err != nil || cfg.Width != 512 || cfg.Height != 512 {
				panic("static mask must remain an original 512x512 PNG: " + asset.File)
			}
			masks++
		}
		if strings.HasSuffix(asset.File, ".yaml") {
			templates++
		}
	}
	if masks != 11 || templates != 2 || !seen["sources/pixi/Fire.png"] || !seen["sources/LICENSE-PARTICLES.txt"] {
		panic("used static masks, fire templates or license notice missing")
	}
}

// Bound reads and reject symlinks before using os.Root to confine the open.
func readStaticSource(name string, limit int64) []byte {
	if len(name) > 512 || !fs.ValidPath(name) || strings.ContainsAny(name, "\\:\x00") ||
		strings.ContainsFunc(name, func(char rune) bool { return char < 32 || char == 127 }) {
		panic("invalid static source path")
	}
	directory := filepath.Dir(assetDirectory)
	for current := filepath.Join(directory, filepath.FromSlash(name)); ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			panic("static source path missing or symlinked: " + current)
		}
		parent := filepath.Dir(current)
		if parent == current || current == "." {
			break
		}
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		panic(err)
	}
	defer root.Close()
	file, err := root.Open(name)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > limit {
		panic("static source nonregular or oversized: " + name)
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(data)) != info.Size() || int64(len(data)) > limit {
		panic("static source changed or exceeded read bounds: " + name)
	}
	return data
}
func smooth(lo, hi, value float64) float64 {
	t := clamp((value - lo) / (hi - lo))
	return t * t * (3 - 2*t)
}

func edge(x, y float64) float64 {
	return (1 - smooth(0.92, 1, math.Abs(x))) * (1 - smooth(0.92, 1, math.Abs(y)))
}

func clamp(value float64) float64 { return math.Max(0, math.Min(1, value)) }
func channel(value float64) byte  { return byte(math.Round(255 * clamp(value))) }
func white(alpha float64) color.NRGBA {
	return color.NRGBA{R: 255, G: 255, B: 255, A: channel(alpha)}
}

func writeTexture(name string, size int, pixel func(float64, float64) color.NRGBA) {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for py := range size {
		for px := range size {
			x := 2*(float64(px)+.5)/float64(size) - 1
			y := 2*(float64(py)+.5)/float64(size) - 1
			img.SetNRGBA(px, py, pixel(x, y))
		}
	}
	var data bytes.Buffer
	if err := png.Encode(&data, img); err != nil {
		panic(err)
	}
	publish(name, data.Bytes())
}
func publish(name string, data []byte) {
	path := filepath.Join(assetDirectory, name+".png")
	if check {
		previous, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}
		if !bytes.Equal(previous, data) {
			panic("static asset drift: " + path)
		}
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		panic(err)
	}
}
