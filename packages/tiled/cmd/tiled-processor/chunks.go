package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/karty-game/tiled"
	"github.com/karty-game/tiled/assets"
)

type buildOptions struct {
	Version      uint32   `json:"version"`
	Map          string   `json:"map"`
	Level        string   `json:"level"`
	StaticLayers []string `json:"staticLayers"`
	ChunkSize    int      `json:"chunkSize"`
	PixelScale   int      `json:"pixelScale"`
}
type textureOutput struct {
	Level   string `json:"level"`
	Name    string `json:"name"`
	Source  string `json:"source"`
	Key     string `json:"key"`
	Profile string `json:"profile"`
}
type assetCatalog struct {
	Version  uint32          `json:"version"`
	Textures []textureOutput `json:"textures"`
}

func readBuildOptions(files memoryFS) (map[string]buildOptions, error) {
	result := map[string]buildOptions{}
	for name, data := range files {
		if path.Base(name) != "tiled-build.json" {
			continue
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		var option buildOptions
		if len(data) > 64<<10 {
			return nil, fmt.Errorf("Tiled build options exceed bounds")
		}
		if err := decoder.Decode(&option); err != nil {
			return nil, err
		}
		if decoder.Decode(new(any)) != io.EOF {
			return nil, fmt.Errorf("trailing Tiled build options")
		}
		if option.Version != 1 || !validOutputPath(option.Map) || !validOutputPath(option.Level) ||
			!strings.HasSuffix(option.Map, ".tmj") ||
			len(option.StaticLayers) == 0 ||
			len(option.StaticLayers) > tiled.MaxLayers {
			return nil, fmt.Errorf("invalid Tiled build options")
		}
		if _, exists := result[option.Map]; exists {
			return nil, fmt.Errorf("duplicate Tiled build options")
		}
		if _, exists := files[option.Map]; !exists {
			return nil, fmt.Errorf("Tiled build map is not an input")
		}
		result[option.Map] = option
	}
	return result, nil
}
func validOutputPath(name string) bool {
	return fs.ValidPath(name) && name != "." && !strings.ContainsAny(name, "\\:\x00")
}

func compileStaticMap(files, output memoryFS, name string, level *tiled.Map, options buildOptions) error {
	catalog := assetCatalog{Version: 1}
	if previous, ok := output["assets.json"]; ok {
		if err := json.Unmarshal(previous, &catalog); err != nil {
			return err
		}
	}
	prefix := fmt.Sprintf("generated/%x", sha256.Sum256([]byte(name)))
	chunks, err := assets.BakeStaticLayers(
		files,
		level,
		options.StaticLayers,
		assets.ChunkOptions{Size: options.ChunkSize, PixelScale: options.PixelScale},
		func(chunk assets.StaticChunk) (string, error) {
			buffer := boundedOutput{limit: maxBytes - outputBytes(output)}
			encoder := png.Encoder{CompressionLevel: png.BestSpeed}
			if err := encoder.Encode(&buffer, chunk.Pixels); err != nil {
				return "", err
			}
			sum := sha256.Sum256(buffer.Bytes())
			imageName := fmt.Sprintf("%s/%x.png", prefix, sum)
			if _, exists := output[imageName]; !exists {
				if err := setOutput(output, imageName, buffer.Bytes()); err != nil {
					return "", err
				}
				catalog.Textures = append(
					catalog.Textures,
					textureOutput{
						Level:   options.Level,
						Name:    fmt.Sprintf("_tiled_%x", sum),
						Source:  imageName,
						Key:     imageName + fmt.Sprintf("#0,0,%d,%d", chunk.Pixels.Rect.Dx(), chunk.Pixels.Rect.Dy()),
						Profile: "sprite",
					},
				)
			}
			return imageName, nil
		},
	)
	if err != nil {
		return err
	}
	selected := map[int]bool{}
	for index, layer := range level.LayerNames {
		for _, candidate := range options.StaticLayers {
			if layer == candidate {
				selected[index] = true
			}
		}
	}
	render := *level
	render.Sprites = nil
	for _, sprite := range level.Sprites {
		if !selected[sprite.Layer] {
			render.Sprites = append(render.Sprites, sprite)
		}
	}
	render.Sprites = append(render.Sprites, chunks...)
	encoded, err := tiled.Encode(&render)
	if err != nil {
		return err
	}
	if err := setOutput(output, strings.TrimSuffix(name, ".tmj")+".render.bin", encoded); err != nil {
		return err
	}
	catalogBytes, err := json.Marshal(catalog)
	if err != nil {
		return err
	}
	return setOutput(output, "assets.json", catalogBytes)
}

func encodeResponse(files memoryFS) ([]byte, error) {
	if len(files) > maxFiles {
		return nil, fmt.Errorf("too many compiled Tiled outputs")
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	response := []byte{'K', 'P', 'R', '1', 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(response[4:], uint32(len(names)))
	total := 0
	for _, name := range names {
		data := files[name]
		if !validOutputPath(name) || len(name) > 256 || len(data) > maxBytes-total {
			return nil, fmt.Errorf("compiled Tiled output exceeds bounds")
		}
		total += len(data)
		response = binary.LittleEndian.AppendUint16(response, uint16(len(name)))
		response = binary.LittleEndian.AppendUint32(response, uint32(len(data)))
		response = append(response, name...)
		response = append(response, data...)
	}
	return response, nil
}
