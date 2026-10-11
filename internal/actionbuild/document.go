package actionbuild

import (
	"encoding/json"
	"os"
	"strconv"

	editorschema "github.com/karty-game/karty-sdk/authoring/actions"
	"github.com/karty-game/karty-sdk/format/actions"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty/internal/sourcevalue"
)

func readAuthoredActionSource(directory string) ([]byte, bool, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, false, err
	}
	defer root.Close()

	if _, legacyErr := root.Lstat("actions.json"); legacyErr == nil {
		return nil, false, invalidActions("actions.json is unsupported; migrate to actions.yaml")
	} else if !os.IsNotExist(legacyErr) {
		return nil, false, legacyErr
	}

	contents, err := root.ReadFile("actions.yaml")
	if os.IsNotExist(err) {
		return nil, true, nil
	}

	return contents, false, err
}

func readAuthoredYAML(data []byte) (any, error) {
	value, err := sourcevalue.ParseYAML(data, actions.MaxBytes, 64, 16384)
	if err != nil {
		return nil, err
	}

	return schemaNumbers(value), nil
}

// The editor JSON Schema validator uses json.Number internally. Source input
// remains YAML; numbers are converted without a JSON serialization round trip.
func schemaNumbers(value any) any {
	switch typed := value.(type) {
	case int64:
		return json.Number(strconv.FormatInt(typed, 10))
	case uint64:
		return json.Number(strconv.FormatUint(typed, 10))
	case float64:
		return json.Number(strconv.FormatFloat(typed, 'g', -1, 64))
	case []any:
		for index, child := range typed {
			typed[index] = schemaNumbers(child)
		}
	case map[string]any:
		for key, child := range typed {
			typed[key] = schemaNumbers(child)
		}
	}

	return value
}

// CompileDocument validates YAML against the public editor schema and emits
// the bounded binary runtime document stored in the KLD.
func CompileDocument(source []byte) ([]byte, error) {
	value, err := readAuthoredYAML(source)
	if err != nil {
		return nil, err
	}

	schemaValue, err := readJSON(editorschema.Schema)
	if err != nil {
		return nil, err
	}

	schema, valid := schemaValue.(map[string]any)
	if !valid {
		return nil, invalidActions("invalid public editor schema")
	}

	if err := validateSchema(value, schema, schema, "actions.yaml", 0); err != nil {
		return nil, err
	}

	object, valid := value.(map[string]any)
	if !valid {
		return nil, invalidActions("expected actions object")
	}

	sequences, valid := object["sequences"].([]any)
	if !valid {
		return nil, invalidActions("expected sequences array")
	}

	document := actions.Document{Version: actions.Version, Sequences: make([]actions.Sequence, len(sequences))}
	if schema, ok := object["$schema"].(string); ok {
		document.Schema = schema
	}

	for index, sequenceValue := range sequences {
		sequence, err := runtimeSequence(sequenceValue)
		if err != nil {
			return nil, err
		}

		document.Sequences[index] = sequence
	}

	return actions.Encode(document)
}

func runtimeSequence(value any) (actions.Sequence, error) {
	object, valid := value.(map[string]any)
	if !valid {
		return actions.Sequence{}, invalidActions("expected sequence object")
	}

	values, valid := object["steps"].([]any)
	if !valid {
		return actions.Sequence{}, invalidActions("expected steps array")
	}

	steps, err := runtimeSteps(values)
	if err != nil {
		return actions.Sequence{}, err
	}

	name, _ := object["name"].(string)
	repeat, _ := object["onRepeat"].(string)

	return actions.Sequence{Name: name, OnRepeat: repeat, Steps: steps}, nil
}

func runtimeSteps(values []any) ([]actions.Step, error) {
	if values == nil {
		return nil, nil
	}

	steps := make([]actions.Step, len(values))
	for index, value := range values {
		object, ok := value.(map[string]any)
		if !ok {
			return nil, invalidActions("expected step object")
		}

		step := actions.Step{}
		step.Action, _ = object["action"].(string)
		step.Condition, _ = object["condition"].(string)

		if frames, ok := object["waitFrames"].(json.Number); ok {
			parsed, err := strconv.ParseUint(string(frames), 10, 64)
			if err != nil {
				return nil, err
			}

			step.WaitFrames = parsed
		}

		args, err := runtimeArgs(object["args"])
		if err != nil {
			return nil, err
		}

		step.Args = args
		for _, branch := range []struct {
			key    string
			target *[]actions.Step
		}{{"then", &step.Then}, {"else", &step.Else}, {"onFailure", &step.OnFailure}} {
			if nested, ok := object[branch.key].([]any); ok {
				compiled, err := runtimeSteps(nested)
				if err != nil {
					return nil, err
				}

				*branch.target = compiled
			}
		}

		steps[index] = step
	}

	return steps, nil
}

func runtimeArgs(value any) (level.Metadata, error) {
	if value == nil {
		return nil, nil //nolint:nilnil // An omitted args object remains absent on the wire.
	}

	args, ok := value.(map[string]any)
	if !ok {
		return nil, invalidActions("expected argument object")
	}

	metadata := make(level.Metadata, len(args))
	for key, value := range args {
		compiled, err := runtimeArgument(value)
		if err != nil {
			return nil, err
		}

		metadata[key] = compiled
	}

	return metadata, nil
}

func runtimeArgument(value any) (level.Value, error) {
	switch typed := value.(type) {
	case string:
		return level.StringValue(typed), nil
	case bool:
		return level.BoolValue(typed), nil
	case json.Number:
		if signed, err := strconv.ParseInt(string(typed), 10, 64); err == nil {
			return level.IntValue(signed), nil
		}

		if unsigned, err := strconv.ParseUint(string(typed), 10, 64); err == nil {
			return level.UintValue(unsigned), nil
		}

		number, err := strconv.ParseFloat(string(typed), 64)
		if err != nil {
			return level.Value{}, err
		}

		return level.FloatValue(number), nil
	case map[string]any:
		actor, ok := typed["actor"].(string)
		if !ok || len(typed) != 1 {
			return level.Value{}, invalidActions("expected actor reference")
		}

		return level.ObjectValue(level.Metadata{"actor": level.StringValue(actor)}), nil
	}

	return level.Value{}, invalidActions("unsupported action argument")
}
