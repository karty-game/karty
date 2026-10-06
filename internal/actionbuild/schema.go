package actionbuild

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"regexp"
	"strings"
	"unicode/utf8"
)

// readJSON rejects duplicate keys, invalid UTF-8, trailing values and deep trees.
// Format shapes and field bounds come from the SDK-owned schema, not a CLI copy.
func readJSON(data []byte) (any, error) {
	if len(data) > 64*1024 || !utf8.Valid(data) {
		return nil, invalidActions("JSON exceeds 65536 bytes or contains invalid UTF-8")
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	value, err := readValue(decoder, 0)
	if err != nil {
		return nil, err
	}

	if _, err = decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, invalidActions("trailing JSON value")
	}

	return value, nil
}
func readValue(decoder *json.Decoder, depth int) (any, error) {
	if depth > maxJSONDepth {
		return nil, invalidActions("JSON nesting limit exceeded")
	}

	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}

	switch token {
	case json.Delim('{'):
		value := map[string]any{}

		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return nil, err
			}

			name, validKey := key.(string)
			if !validKey {
				return nil, invalidActions("expected JSON object key")
			}

			if _, exists := value[name]; exists {
				return nil, invalidActions("duplicate JSON key %q", name)
			}

			item, err := readValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}

			value[name] = item
		}

		_, err = decoder.Token()

		return value, err
	case json.Delim('['):
		value := []any{}

		for decoder.More() {
			item, err := readValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}

			value = append(value, item)
		}

		_, err = decoder.Token()

		return value, err
	default:
		return token, nil
	}
}

// validateSchema evaluates the subset used by the public action schema and the
// declaration-derived argument schema. Unknown keywords fail closed at load time.
func checkSchema(schema map[string]any) error {
	for key, value := range schema {
		if err := checkSchemaKeyword(key, value); err != nil {
			return err
		}
	}

	return nil
}
func checkSchemaKeyword(key string, value any) error {
	allowed := map[string]bool{
		"$schema":              true,
		"$id":                  true,
		"title":                true,
		"$defs":                true,
		"$ref":                 true,
		"type":                 true,
		"properties":           true,
		"required":             true,
		"additionalProperties": true,
		"maxProperties":        true,
		"items":                true,
		"maxItems":             true,
		"minLength":            true,
		"maxLength":            true,
		"pattern":              true,
		"minimum":              true,
		"maximum":              true,
		"enum":                 true,
		"const":                true,
		"oneOf":                true,
	}

	if !allowed[key] {
		return invalidActions("unsupported action schema keyword %s", key)
	}

	invalid := func() error { return invalidActions("invalid action schema keyword %s", key) }

	switch key {
	case "$defs", "properties":
		children, ok := value.(map[string]any)
		if !ok {
			return invalid()
		}

		for _, child := range children {
			if err := checkSchemaChild(child); err != nil {
				return err
			}
		}
	case "items":
		return checkSchemaChild(value)
	case "additionalProperties":
		if _, ok := value.(bool); !ok {
			return checkSchemaChild(value)
		}
	case "oneOf":
		children, ok := value.([]any)
		if !ok || len(children) == 0 {
			return invalid()
		}

		for _, child := range children {
			if err := checkSchemaChild(child); err != nil {
				return err
			}
		}
	default:
		return checkSchemaScalar(key, value)
	}

	return nil
}
func numeric(value any) float64 {
	switch number := value.(type) {
	case json.Number:
		f, _ := number.Float64()

		return f
	case float64:
		return number
	case int:
		return float64(number)
	}

	return 0
}
func validateSchema(value any, schema, root map[string]any, path string, depth int) error {
	if depth > maxSchemaDepth {
		return invalidActions("%s: schema nesting limit exceeded", path)
	}

	fail := func(message string) error { return invalidActions("%s: %s", path, message) }

	if ref, ok := schema["$ref"].(string); ok {
		if !strings.HasPrefix(ref, "#/$defs/") {
			return fail("unsupported schema reference")
		}

		defs, _ := root["$defs"].(map[string]any)

		child, ok := defs[strings.TrimPrefix(ref, "#/$defs/")].(map[string]any)
		if !ok {
			return fail("missing schema reference")
		}

		return validateSchema(value, child, root, path, depth+1)
	}

	if choices, ok := schema["oneOf"].([]any); ok {
		if err := validateChoices(value, choices, root, path, depth); err != nil {
			return err
		}
	}

	if expected, ok := schema["const"]; ok && !reflect.DeepEqual(value, expected) {
		return fail("unsupported value or version")
	}

	if choices, ok := schema["enum"].([]any); ok {
		matched := false
		for _, choice := range choices {
			matched = matched || reflect.DeepEqual(value, choice)
		}

		if !matched {
			return fail("unsupported value")
		}
	}

	switch schema["type"] {
	case "object":
		return validateObject(value, schema, root, path, depth)
	case "array":
		return validateArray(value, schema, root, path, depth)
	case "string":
		return validateString(value, schema, path)
	case "boolean":
		return validateBoolean(value, path)
	case "number", "integer":
		return validateNumber(value, schema, path)
	}

	return nil
}

func arrayItemSchema(schema map[string]any) map[string]any {
	child, _ := schema["items"].(map[string]any)

	return child
}

func checkSchemaChild(value any) error {
	child, ok := value.(map[string]any)
	if !ok {
		return invalidActions("schema child must be an object")
	}

	return checkSchema(child)
}

func validateChoices(value any, choices []any, root map[string]any, path string, depth int) error {
	matches := 0

	var argumentError error

	for _, choice := range choices {
		child, validChild := choice.(map[string]any)
		if !validChild {
			return invalidActions("%s: expected schema choice object", path)
		}

		err := validateSchema(value, child, root, path, depth+1)
		if err == nil {
			matches++
		} else if matchesActionChoice(value, child) {
			argumentError = err
		}
	}

	if matches != 1 {
		if matches == 0 && argumentError != nil {
			return argumentError
		}

		return invalidActions("%s: expected exactly one supported action, condition, wait or argument", path)
	}

	return nil
}

func matchesActionChoice(value any, child map[string]any) bool {
	object, ok := value.(map[string]any)
	if !ok {
		return false
	}

	props, _ := child["properties"].(map[string]any)

	for _, key := range []string{"action", "condition", "waitFrames"} {
		supplied, exists := object[key]

		property, _ := props[key].(map[string]any)
		if exists && property != nil && (key == "waitFrames" || reflect.DeepEqual(supplied, property["const"])) {
			return true
		}
	}

	return false
}

func validateObject(value any, schema, root map[string]any, path string, depth int) error {
	fail := func(message string) error { return invalidActions("%s: %s", path, message) }

	object, ok := value.(map[string]any)
	if !ok {
		return fail("expected object")
	}

	if limit, ok := schema["maxProperties"]; ok && len(object) > int(numeric(limit)) {
		return fail("too many arguments")
	}

	if required, ok := schema["required"].([]any); ok {
		for _, key := range required {
			name, validName := key.(string)
			if !validName {
				return fail("expected required field name")
			}

			if _, exists := object[name]; !exists {
				return fail("missing field " + name)
			}
		}
	}

	props, _ := schema["properties"].(map[string]any)
	for key, item := range object {
		child, known := props[key].(map[string]any)
		if !known {
			switch extra := schema["additionalProperties"].(type) {
			case bool:
				if !extra {
					return fail("unknown field " + key)
				}
			case map[string]any:
				child = extra
			default:
				continue
			}
		}

		if child != nil {
			if err := validateSchema(item, child, root, path+"."+key, depth+1); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateArray(value any, schema, root map[string]any, path string, depth int) error {
	fail := func(message string) error { return invalidActions("%s: %s", path, message) }

	items, ok := value.([]any)
	if !ok {
		return fail("expected array")
	}

	if limit, ok := schema["maxItems"]; ok && len(items) > int(numeric(limit)) {
		return fail("too many items")
	}

	for index, item := range items {
		if err := validateSchema(item, arrayItemSchema(schema), root, fmt.Sprintf("%s[%d]", path, index), depth+1); err != nil {
			return err
		}
	}

	return nil
}

func validateString(value any, schema map[string]any, path string) error {
	fail := func(message string) error { return invalidActions("%s: %s", path, message) }

	text, ok := value.(string)
	if !ok {
		return fail("expected string")
	}

	if minimum, ok := schema["minLength"]; ok && len(text) < int(numeric(minimum)) {
		return fail("string too short")
	}

	if maximum, ok := schema["maxLength"]; ok && len(text) > int(numeric(maximum)) {
		return fail("string too long")
	}

	if pattern, ok := schema["pattern"].(string); ok {
		matched, err := regexp.MatchString(pattern, text)
		if err != nil || !matched {
			return fail("invalid identifier")
		}
	}

	return nil
}

func validateBoolean(value any, path string) error {
	fail := func(message string) error { return invalidActions("%s: %s", path, message) }
	if _, ok := value.(bool); !ok {
		return fail("expected boolean")
	}

	return nil
}

func validateNumber(value any, schema map[string]any, path string) error {
	fail := func(message string) error { return invalidActions("%s: %s", path, message) }

	number, ok := value.(json.Number)
	if !ok {
		return fail("expected number")
	}

	numberValue, err := number.Float64()
	if err != nil || math.IsNaN(numberValue) || math.IsInf(numberValue, 0) {
		return fail("non-finite number")
	}

	if schema["type"] == "integer" && math.Trunc(numberValue) != numberValue {
		return fail("expected integer")
	}

	if minimum, ok := schema["minimum"]; ok && numberValue < numeric(minimum) {
		return fail("number below bound")
	}

	if maximum, ok := schema["maximum"]; ok && numberValue > numeric(maximum) {
		return fail("number above bound")
	}

	return nil
}

func checkSchemaScalar(key string, value any) error {
	invalid := func() error { return invalidActions("invalid action schema keyword %s", key) }

	switch key {
	case "required", "enum":
		items, ok := value.([]any)
		if !ok {
			return invalid()
		}

		if key == "required" {
			for _, item := range items {
				if _, ok := item.(string); !ok {
					return invalid()
				}
			}
		}
	case "$schema", "$id", "title", "$ref", "type", "pattern":
		text, ok := value.(string)
		if !ok {
			return invalid()
		}

		if key == "pattern" {
			if _, err := regexp.Compile(text); err != nil {
				return invalid()
			}
		}

		if key == "type" && text != "object" && text != "array" && text != "string" && text != "number" && text != "integer" &&
			text != "boolean" {
			return invalid()
		}
	case "minimum", "maximum", "maxProperties", "maxItems", "minLength", "maxLength":
		number, ok := value.(json.Number)
		if !ok {
			return invalid()
		}

		if f, err := number.Float64(); err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
			return invalid()
		}
	}

	return nil
}
