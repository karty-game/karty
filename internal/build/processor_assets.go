package build

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/karty-game/karty/internal/processorassets"
)

const generatedTexturePixelBytes = 4
const generatedTextureMaxDimension = 4096
const generatedTextureMaxDecodedBytes = 256 << 20

func stageProcessorAssets(
	root *os.Root,
	output, processor string,
	files []processorFile,
) (map[string]bool, []processorassets.Texture, error) {
	byName := map[string][]byte{}
	for _, file := range files {
		byName[file.Path] = file.Data
	}

	used := map[string]bool{}

	data, exists := byName[processorassets.CatalogName]
	if !exists {
		return used, nil, nil
	}

	catalog, err := processorassets.Decode(data)
	if err != nil {
		return nil, nil, err
	}

	if err := os.MkdirAll(filepath.Join(output, "assets"), 0700); err != nil {
		return nil, nil, err
	}

	var decodedBytes int64

	for index := range catalog.Textures {
		texture := &catalog.Textures[index]
		if texture.Processor != "" || texture.ID != 0 || !strings.HasSuffix(texture.Source, ".png") {
			return nil, nil, fmt.Errorf("%w: processor cannot assign generated texture identity", errProcessorResponse)
		}

		pixels, ok := byName[texture.Source]
		if !ok {
			return nil, nil, fmt.Errorf("%w: generated texture source is absent: %s", errProcessorResponse, texture.Source)
		}

		if !used[texture.Source] {
			configuration, err := png.DecodeConfig(bytes.NewReader(pixels))
			if err != nil {
				return nil, nil, err
			}

			charge := int64(configuration.Width) * int64(configuration.Height) * generatedTexturePixelBytes
			if configuration.Width < 1 || configuration.Height < 1 || configuration.Width > generatedTextureMaxDimension ||
				configuration.Height > generatedTextureMaxDimension ||
				charge > generatedTextureMaxDecodedBytes-decodedBytes {
				return nil, nil, fmt.Errorf("%w: generated texture decoded pixels exceed bounds", errProcessorResponse)
			}

			decodedBytes += charge

			if _, err := png.Decode(bytes.NewReader(pixels)); err != nil {
				return nil, nil, err
			}
		}

		used[texture.Source] = true
		digest := sha256.Sum256([]byte(processor + "\x00" + texture.Source))

		name := hex.EncodeToString(digest[:]) + ".png"
		if err := os.WriteFile(filepath.Join(output, "assets", name), pixels, 0600); err != nil {
			return nil, nil, err
		}

		texture.Source = ".karty/processed/assets/" + name
		texture.Processor = processor
	}
	// Resolve existing authored catalogs before publishing any processor output.
	if _, err := processorassets.AssignIDs(root, catalog.Textures); err != nil {
		return nil, nil, err
	}

	return used, catalog.Textures, nil
}

func publishProcessorAssets(root *os.Root, output string, textures []processorassets.Texture) (string, error) {
	assigned, err := processorassets.AssignIDs(root, textures)
	if err != nil {
		return "", err
	}

	data, err := processorassets.Encode(processorassets.Catalog{Version: 1, Textures: assigned})
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(filepath.Join(output, "assets.json"), data, 0600); err != nil {
		return "", err
	}

	var lookup strings.Builder
	lookup.WriteString(
		"\n// TextureID resolves a generated level texture by processor image key.\nfunc TextureID(processor, key string) (uint32,bool) { switch processor+\"/\"+key {\n",
	)

	for _, texture := range assigned {
		fmt.Fprintf(&lookup, "case %s: return %d,true\n", strconv.Quote(texture.Processor+"/"+texture.Key), texture.ID)
	}

	lookup.WriteString("};return 0,false}\n")

	return lookup.String(), nil
}
