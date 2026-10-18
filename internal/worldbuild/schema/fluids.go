package schema

import "github.com/google/jsonschema-go/jsonschema"

func (g *grammar) fluidRules() {
	f := g.definitions["Fluids"]
	f.Required = []string{"version", "items"}
	f.Properties["version"] = &jsonschema.Schema{Type: "integer", Enum: []any{1}}
	f.Properties["items"].MinItems = new(1)
	f.Properties["items"].MaxItems = new(8)
	fluid := g.definitions["Fluid"]
	fluid.Required = []string{"id", "solids", "density"}
	fluid.Properties["solids"].MinItems = new(1)
	fluid.Properties["solids"].MaxItems = new(64)

	for _, v := range []struct {
		name     string
		min, max float64
	}{{"density", .001, 20000}, {"linear_drag", 0, 30}, {"angular_drag", 0, 30}, {"foam_width", 0, 1}, {"foam_strength", 0, 1}, {"fog_density", 0, 10}, {"distortion", 0, .02}} {
		fluid.Properties[v.name] = &jsonschema.Schema{Type: "number", Minimum: new(v.min), Maximum: new(v.max)}
	}

	for _, name := range []string{"foam_color", "fog_color"} {
		fluid.Properties[name] = &jsonschema.Schema{
			Type:                 "object",
			AdditionalProperties: deny(),
			Properties: map[string]*jsonschema.Schema{
				"x": {Type: "number", Minimum: new(0.0), Maximum: new(1.0)},
				"y": {Type: "number", Minimum: new(0.0), Maximum: new(1.0)},
				"z": {Type: "number", Minimum: new(0.0), Maximum: new(1.0)},
			},
		}
	}
}
