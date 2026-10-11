package schema

import "github.com/google/jsonschema-go/jsonschema"

func (g *grammar) environmentConstraints() {
	environment := g.definitions["Environment"]
	environment.Properties["preset"] = choice("daylight", "space", "cartoon", "fantasy")
	environment.Properties["palette"] = choice("", "default", "violet", "twilight")
	environment.Properties["time_of_day"] = &jsonschema.Schema{Type: "number", Minimum: new(float64(0)), ExclusiveMaximum: new(float64(24))}
	environment.Properties["cloud_amount"] = number(0, 1)
	environment.AllOf = append(environment.AllOf, &jsonschema.Schema{
		Not: &jsonschema.Schema{Required: []string{"time_of_day", "sun"}},
	})
	sun := g.definitions["EnvironmentSun"]
	sun.Properties["azimuth"] = &jsonschema.Schema{Type: "number", Minimum: new(float64(0)), ExclusiveMaximum: new(float64(360))}
	sun.Properties["elevation"] = number(0, 90)
	// Optional fields must be omitted, not explicitly null.
	g.definitions["Document"].Properties["environment"] = g.definitions["Document"].Properties["environment"].Then
	environment.Properties["sun"] = environment.Properties["sun"].Then
	sky := &jsonschema.Schema{Required: []string{"sky"}, Properties: map[string]*jsonschema.Schema{"sky": {Enum: []any{true}}}}
	rooms := &jsonschema.Schema{
		Required:   []string{"rooms"},
		Properties: map[string]*jsonschema.Schema{"rooms": {Type: "array", Contains: sky}},
	}
	g.definitions["Document"].AllOf = append(g.definitions["Document"].AllOf, &jsonschema.Schema{
		If: &jsonschema.Schema{AnyOf: []*jsonschema.Schema{
			rooms,
			{Required: []string{"prefabs"}, Properties: map[string]*jsonschema.Schema{"prefabs": {Type: "array", Contains: rooms}}},
		}},
		Then: &jsonschema.Schema{Required: []string{"environment"}},
	})
	g.definitions["Room"].AllOf = append(g.definitions["Room"].AllOf, &jsonschema.Schema{
		If: &jsonschema.Schema{
			Required:   []string{"boundary"},
			Properties: map[string]*jsonschema.Schema{"boundary": {Type: "array", Contains: sky}},
		},
		Then: sky,
	})
}
