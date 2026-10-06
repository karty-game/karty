package schema

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/karty-game/karty-sdk/format/world"
	"go.yaml.in/yaml/v3"
)

var ErrValidation = errors.New("world YAML schema validation failed")

const maxYAMLDepth = 64

// Parse preserves YAML scalar types so schema validation cannot silently coerce
// a quoted number, boolean or identifier through Go's typed YAML decoder.
func Parse(contents []byte) (any, error) {
	if len(contents) == 0 || len(contents) > world.MaxEncodedSize {
		return nil, fmt.Errorf("%w: source must contain 1–%d bytes", ErrValidation, world.MaxEncodedSize)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(contents))

	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrValidation, err)
	}

	var trailing yaml.Node
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: expected exactly one YAML document", ErrValidation)
	}

	if len(document.Content) != 1 {
		return nil, fmt.Errorf("%w: expected a world object", ErrValidation)
	}

	return nodeValue(document.Content[0], 0)
}

func nodeValue(node *yaml.Node, depth int) (any, error) {
	if depth > maxYAMLDepth || node.Anchor != "" || node.Kind == yaml.AliasNode {
		return nil, fmt.Errorf("%w: line %d: aliases, anchors and nesting over %d are unsupported", ErrValidation, node.Line, maxYAMLDepth)
	}

	switch node.Kind {
	case yaml.MappingNode:
		return mappingValue(node, depth)
	case yaml.SequenceNode:
		values := make([]any, len(node.Content))
		for index, child := range node.Content {
			value, err := nodeValue(child, depth+1)
			if err != nil {
				return nil, err
			}

			values[index] = value
		}

		return values, nil
	case yaml.ScalarNode:
		return scalarValue(node)
	case yaml.DocumentNode, yaml.AliasNode:
		// Document wrappers and aliases are never values in the world grammar.
	}

	return nil, fmt.Errorf("%w: line %d: unsupported YAML value or tag %q", ErrValidation, node.Line, node.Tag)
}

func mappingValue(node *yaml.Node, depth int) (map[string]any, error) {
	object := make(map[string]any, len(node.Content)/2)
	for index := 0; index < len(node.Content); index += 2 {
		key := node.Content[index]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || key.Anchor != "" {
			return nil, fmt.Errorf("%w: line %d: object keys must be strings", ErrValidation, key.Line)
		}

		if _, exists := object[key.Value]; exists {
			return nil, fmt.Errorf("%w: line %d: duplicate field %q", ErrValidation, key.Line, key.Value)
		}

		value, err := nodeValue(node.Content[index+1], depth+1)
		if err != nil {
			return nil, err
		}

		object[key.Value] = value
	}

	return object, nil
}

func scalarValue(node *yaml.Node) (any, error) {
	switch node.Tag {
	case "!!str", "!!null", "!!bool", "!!int", "!!float":
		var value any
		if err := node.Decode(&value); err != nil {
			return nil, fmt.Errorf("%w: line %d: %w", ErrValidation, node.Line, err)
		}

		if number, ok := value.(float64); ok && (math.IsNaN(number) || math.IsInf(number, 0)) {
			return nil, fmt.Errorf("%w: line %d: numbers must be finite", ErrValidation, node.Line)
		}

		return value, nil
	}

	return nil, fmt.Errorf("%w: line %d: unsupported YAML value or tag %q", ErrValidation, node.Line, node.Tag)
}
