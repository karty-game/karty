package levelbuild

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty/internal/sourcevalue"
	"github.com/pelletier/go-toml/v2"
)

// YAML and TOML are authoring inputs only. Levels carry typed binary metadata.
func parseAuthoredMetadata(contents []byte, path string) (level.Metadata, error) {
	var (
		source any
		err    error
	)

	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		source, err = sourcevalue.ParseYAML(contents, level.MaxMetadataSize, level.MaxMetadataDepth, level.MaxMetadataNodes)
	case ".toml":
		var object map[string]any

		err = toml.Unmarshal(contents, &object)
		source = object
	default:
		return nil, fmt.Errorf("authored metadata must use YAML or TOML: %w", level.ErrMetadata)
	}

	if err != nil {
		return nil, err
	}

	value, err := authoredMetadataValue(source)
	if err != nil {
		return nil, err
	}

	metadata, ok := value.AsObject()
	if !ok {
		return nil, level.ErrMetadata
	}

	return metadata, nil
}

func authoredMetadataValue(source any) (level.Value, error) {
	switch value := source.(type) {
	case nil:
		return level.NullValue(), nil
	case bool:
		return level.BoolValue(value), nil
	case string:
		return level.StringValue(value), nil
	case int64:
		return level.IntValue(value), nil
	case uint64:
		return level.UintValue(value), nil
	case float64:
		return level.FloatValue(value), nil
	case []any:
		array := make([]level.Value, len(value))
		for index, child := range value {
			converted, err := authoredMetadataValue(child)
			if err != nil {
				return level.Value{}, err
			}

			array[index] = converted
		}

		return level.ArrayValue(array), nil
	case map[string]any:
		object := make(level.Metadata, len(value))
		for key, child := range value {
			converted, err := authoredMetadataValue(child)
			if err != nil {
				return level.Value{}, err
			}

			object[key] = converted
		}

		return level.ObjectValue(object), nil
	}

	return level.Value{}, fmt.Errorf("unsupported authored metadata value: %w", level.ErrMetadata)
}
