// Package processorassets validates optional generated level textures from build tools.
package processorassets

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

var ErrCatalog = errors.New("invalid processor asset catalog")

const CatalogName = "assets.json"
const RegistryName = ".karty/processed/assets.json"
const MaxCatalogBytes = 64 << 10
const Marker = "karty-processed@1\n"

type Texture struct {
	Level     string `json:"level"`
	Name      string `json:"name"`
	Source    string `json:"source"`
	Key       string `json:"key"`
	Profile   string `json:"profile"`
	Processor string `json:"processor,omitempty"`
	ID        uint32 `json:"id,omitempty"`
}
type Catalog struct {
	Version  uint32    `json:"version"`
	Textures []Texture `json:"textures"`
}

// Encode writes the canonical generated registry using the versioned catalog codec.
func Encode(catalog Catalog) ([]byte, error) {
	data, err := json.Marshal(catalog)
	if err != nil {
		return nil, err
	}

	if len(data) > MaxCatalogBytes {
		return nil, fmt.Errorf("%w: generated texture registry exceeds bounds", ErrCatalog)
	}

	return data, nil
}

func Decode(data []byte) (Catalog, error) {
	var result Catalog
	if len(data) > MaxCatalogBytes {
		return result, fmt.Errorf("%w: generated texture catalog exceeds limit", ErrCatalog)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&result); err != nil {
		return result, err
	}

	if decoder.Decode(new(any)) != io.EOF {
		return Catalog{}, fmt.Errorf("%w: trailing generated texture catalog data", ErrCatalog)
	}

	if result.Version != 1 || len(result.Textures) > 256 {
		return result, fmt.Errorf("%w: invalid generated texture catalog", ErrCatalog)
	}

	seen := map[string]bool{}
	for _, texture := range result.Textures {
		if !validPath(texture.Level) || !validPath(texture.Source) ||
			strings.ContainsAny(texture.Name+texture.Key+texture.Profile+texture.Processor, "\x00") ||
			texture.Name == "" ||
			len(texture.Name) > 256 ||
			texture.Key == "" ||
			len(texture.Key) > 1024 ||
			texture.Profile == "" ||
			len(texture.Profile) > 64 ||
			seen[texture.Level+"/"+texture.Name] {
			return Catalog{}, fmt.Errorf("%w: invalid generated texture declaration", ErrCatalog)
		}

		seen[texture.Level+"/"+texture.Name] = true
	}

	return result, nil
}
func validPath(value string) bool {
	return fs.ValidPath(value) && value != "." && !strings.ContainsAny(value, "\\:\x00") && !strings.HasPrefix(value, "../")
}

// AssignIDs appends generated textures after each level's original sorted catalog.
func AssignIDs(root *os.Root, textures []Texture) ([]Texture, error) {
	result := slices.Clone(textures)
	slices.SortFunc(result, func(a, b Texture) int {
		if n := strings.Compare(a.Level, b.Level); n != 0 {
			return n
		}

		return strings.Compare(a.Name, b.Name)
	})

	var (
		current string
		next    uint32
		names   map[string]bool
	)

	keys := map[string]bool{}

	for index := range result {
		texture := &result[index]
		if texture.Level != current {
			data, err := readBounded(root, path.Join("levels", texture.Level, "level.toml"), 1<<20)
			if err != nil {
				return nil, err
			}

			var level struct {
				Textures []struct {
					Name string `toml:"name"`
				} `toml:"textures"`
			}

			if len(data) > 1<<20 {
				return nil, fmt.Errorf("%w: level manifest exceeds limit", ErrCatalog)
			}

			if err := toml.Unmarshal(data, &level); err != nil {
				return nil, err
			}

			names = map[string]bool{}
			for _, entry := range level.Textures {
				if entry.Name == "" || names[entry.Name] {
					return nil, fmt.Errorf("%w: invalid authored texture catalog", ErrCatalog)
				}

				names[entry.Name] = true
			}

			current = texture.Level
			next = uint32(len(level.Textures)) + 1
		}

		key := texture.Processor + "/" + texture.Key
		if names[texture.Name] || keys[key] || next > 4096 {
			return nil, fmt.Errorf("%w: duplicate generated texture or catalog limit", ErrCatalog)
		}

		names[texture.Name] = true
		keys[key] = true
		texture.ID = next
		next++
	}

	return result, nil
}

// Read reads only the transactionally published, owned processor registry.
func Read(root *os.Root) ([]Texture, error) {
	data, err := readBounded(root, RegistryName, MaxCatalogBytes)
	if os.IsNotExist(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	marker, err := readBounded(root, ".karty/processed/.owner", 64)
	if err != nil || string(marker) != Marker {
		return nil, fmt.Errorf("%w: generated asset registry is not owned", ErrCatalog)
	}

	catalog, err := Decode(data)
	if err != nil {
		return nil, err
	}

	for _, texture := range catalog.Textures {
		if texture.ID == 0 || texture.ID > 4096 || !strings.HasPrefix(texture.Source, ".karty/processed/assets/") {
			return nil, fmt.Errorf("%w: invalid generated asset registry entry", ErrCatalog)
		}
	}

	return catalog.Textures, nil
}

func readBounded(root *os.Root, name string, maximum int64) ([]byte, error) {
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > maximum {
		return nil, fmt.Errorf("%w: generated asset metadata exceeds bounds", ErrCatalog)
	}

	data, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		return nil, err
	}

	if int64(len(data)) > maximum {
		return nil, fmt.Errorf("%w: generated asset metadata exceeds bounds", ErrCatalog)
	}

	return data, nil
}
