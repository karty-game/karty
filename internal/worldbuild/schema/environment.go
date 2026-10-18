package schema

import (
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/karty-game/karty-sdk/format/worldsource"
)

const maxEnvironmentAmbient = 4.0

func (g *grammar) environmentConstraints() {
	environment := g.definitions["Environment"]
	environment.Properties["lighting"] = environment.Properties["lighting"].Then
	environment.Properties["colors"] = environment.Properties["colors"].Then
	lighting := g.definitions["EnvironmentLighting"]
	lighting.Required = []string{"sun_color", "sun_intensity", "ambient"}
	lighting.Properties["sun_intensity"] = number(0, 16)
	lighting.Properties["ambient_cube"] = lighting.Properties["ambient_cube"].Then

	boundedColor := func(upperBound float64) *jsonschema.Schema {
		return &jsonschema.Schema{
			Required:   []string{"x", "y", "z"},
			Properties: map[string]*jsonschema.Schema{"x": number(0, upperBound), "y": number(0, upperBound), "z": number(0, upperBound)},
		}
	}
	for name, limit := range map[string]float64{"sun_color": 1, "ambient": maxEnvironmentAmbient} {
		lighting.Properties[name] = &jsonschema.Schema{AllOf: []*jsonschema.Schema{lighting.Properties[name], boundedColor(limit)}}
	}

	for name, property := range g.definitions["EnvironmentColors"].Properties {
		g.definitions["EnvironmentColors"].Properties[name] = &jsonschema.Schema{AllOf: []*jsonschema.Schema{property, boundedColor(1)}}
	}

	cubeProperties := map[string]*jsonschema.Schema{}

	cubeRequired := []string{"positive_x", "negative_x", "positive_y", "negative_y", "positive_z", "negative_z"}
	for _, name := range cubeRequired {
		cubeProperties[name] = boundedColor(8)
	}

	lighting.Properties["ambient_cube"] = &jsonschema.Schema{
		AllOf: []*jsonschema.Schema{lighting.Properties["ambient_cube"], {Required: cubeRequired, Properties: cubeProperties}},
	}
	g.definitions["EnvironmentColors"].Required = []string{"zenith", "horizon", "nadir", "cloud", "sun"}
	g.definitions["Document"].AllOf = append(g.definitions["Document"].AllOf, &jsonschema.Schema{
		If: &jsonschema.Schema{
			Properties: map[string]*jsonschema.Schema{"version": {Minimum: new(float64(worldsource.ExplicitEnvironmentVersion))}},
		},
		Then: &jsonschema.Schema{
			Properties: map[string]*jsonschema.Schema{
				"environment": {
					Required:   []string{"lighting"},
					Properties: map[string]*jsonschema.Schema{"preset": deny(), "palette": deny(), "backdrop": deny()},
				},
			},
		},
		Else: &jsonschema.Schema{
			Properties: map[string]*jsonschema.Schema{
				"environment": {
					Required:   []string{"preset"},
					Properties: map[string]*jsonschema.Schema{"lighting": deny(), "colors": deny()},
				},
			},
		},
	})
	environment.Properties["preset"] = choice("daylight", "daylight-bright", "space", "cartoon", "fantasy")
	environment.Properties["palette"] = choice("", "default", "violet", "twilight")
	environment.Properties["backdrop"] = choice("", "none", "mountains", "city")
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
	environment.Properties["sky"] = environment.Properties["sky"].Then
	skySettings := g.definitions["SkySettings"]
	g.definitions["Sky"].Properties["settings"] = g.definitions["Sky"].Properties["settings"].Then

	for name, setting := range skySettings.Properties {
		skySettings.Properties[name] = setting.Then
	}

	skySettings.Properties["cloud_uv_scale"].Items = &jsonschema.Schema{
		Type:             "number",
		ExclusiveMinimum: new(float64(0)),
		Maximum:          new(float64(4)),
	}
	skySettings.Properties["cloud_dense_scroll_scale"] = number(0, 4)
	skySettings.Properties["cloud_dense_offset"].Items = number(-1, 1)
	skySettings.Properties["cloud_parallax"].Items = number(-.01, .01)
	skySettings.Properties["horizon_parallax"] = number(0, .01)
	skySettings.Properties["horizon_uv_scale"] = &jsonschema.Schema{
		Type:             "number",
		ExclusiveMinimum: new(float64(0)),
		Maximum:          new(float64(4)),
	}
	skySettings.Properties["horizon_haze"] = number(0, 1)
	skySettings.Properties["horizon_emission"].Items = number(0, 1)
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
