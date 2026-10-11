// Package sourcevalue parses bounded YAML authoring values. It is build tooling.
package sourcevalue

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

var ErrValue = errors.New("invalid authored YAML value")

func ParseYAML(data []byte, maxBytes, maxDepth, maxNodes int) (any, error) {
	if len(data) == 0 || len(data) > maxBytes || !utf8.Valid(data) {
		return nil, fmt.Errorf("YAML exceeds bounds: %w", ErrValue)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))

	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}

	var trailing yaml.Node
	if !errors.Is(decoder.Decode(&trailing), io.EOF) || len(document.Content) != 1 {
		return nil, fmt.Errorf("expected one YAML document: %w", ErrValue)
	}

	count := 0

	return convert(document.Content[0], 0, maxDepth, maxNodes, &count)
}
func convert(node *yaml.Node, depth, maxDepth, maxNodes int, count *int) (any, error) {
	*count++
	if depth > maxDepth || *count > maxNodes {
		return nil, fmt.Errorf("YAML nesting or value limit exceeded: %w", ErrValue)
	}

	switch node.Kind {
	case yaml.MappingNode:
		return objectValue(node, depth, maxDepth, maxNodes, count)
	case yaml.SequenceNode:
		array := make([]any, len(node.Content))
		for index, child := range node.Content {
			value, err := convert(child, depth+1, maxDepth, maxNodes, count)
			if err != nil {
				return nil, err
			}

			array[index] = value
		}

		return array, nil
	case yaml.ScalarNode:
		return scalarValue(node)
	case yaml.DocumentNode, yaml.AliasNode:
		return nil, fmt.Errorf("unsupported YAML value or alias: %w", ErrValue)
	}

	return nil, ErrValue
}

func objectValue(node *yaml.Node, depth, maxDepth, maxNodes int, count *int) (any, error) {
	object := make(map[string]any, len(node.Content)/2)
	for index := 0; index < len(node.Content); index += 2 {
		key := node.Content[index]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || key.Value == "" {
			return nil, fmt.Errorf("expected string object key: %w", ErrValue)
		}

		if _, duplicate := object[key.Value]; duplicate {
			return nil, fmt.Errorf("duplicate YAML key %q: %w", key.Value, ErrValue)
		}

		value, err := convert(node.Content[index+1], depth+1, maxDepth, maxNodes, count)
		if err != nil {
			return nil, err
		}

		object[key.Value] = value
	}

	return object, nil
}

func scalarValue(node *yaml.Node) (any, error) {
	switch node.Tag {
	case "!!null":
		return nil, nil //nolint:nilnil // YAML null is an explicit, successful authored value.
	case "!!str":
		return node.Value, nil
	case "!!bool":
		return strings.EqualFold(node.Value, "true"), nil
	case "!!int":
		text := strings.ReplaceAll(node.Value, "_", "")
		if signed, err := strconv.ParseInt(text, 0, 64); err == nil {
			return signed, nil
		}

		if unsigned, err := strconv.ParseUint(text, 0, 64); err == nil {
			return unsigned, nil
		}
	case "!!float":
		value, err := strconv.ParseFloat(strings.ReplaceAll(node.Value, "_", ""), 64)
		if err == nil && !math.IsNaN(value) && !math.IsInf(value, 0) {
			return value, nil
		}
	}

	return nil, ErrValue
}
