package schema

import (
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty-sdk/format/worldvegetation"
)

func (g *grammar) vegetationConstraints() {
	set := func(name, field string, s *jsonschema.Schema) { g.definitions[name].Properties[field] = s }
	array := func(items *jsonschema.Schema, minimum, maximum int) *jsonschema.Schema {
		return &jsonschema.Schema{Type: "array", Items: items, MinItems: new(minimum), MaxItems: new(maximum)}
	}
	positiveRange := &jsonschema.Schema{
		Type:                 "object",
		Required:             []string{"min", "max"},
		AdditionalProperties: deny(),
		Properties: map[string]*jsonschema.Schema{
			"min": {Type: "number", Minimum: new(float64(.001)), Maximum: new(float64(worldvegetation.MaxSize))},
			"max": {Type: "number", Minimum: new(float64(.001)), Maximum: new(float64(worldvegetation.MaxSize))},
		},
	}
	g.definitions["Document"].Properties["vegetation"] = g.definitions["Document"].Properties["vegetation"].Then
	g.definitions["Vegetation"].Required = []string{"version", "presets"}
	g.definitions["VegetationPreset"].Required = []string{
		"name",
		"texture",
		"width",
		"height",
		"wind_amplitude",
		"wind_frequency",
		"draw_distance",
	}
	g.definitions["VegetationLayer"].Required = []string{"preset", "density", "seed", "surface"}

	set(
		"Vegetation",
		"version",
		&jsonschema.Schema{
			Enum: []any{
				worldvegetation.Version,
				worldvegetation.GeometryVersion,
				worldvegetation.NormalVersion,
				worldvegetation.TreeVersion,
				worldvegetation.PalmVersion,
			},
		},
	)
	set("Vegetation", "presets", array(&jsonschema.Schema{Ref: "#/$defs/VegetationPreset"}, 1, worldvegetation.MaxPresets))

	for _, field := range []string{"name", "texture"} {
		set("VegetationPreset", field, identifier())
	}

	for _, field := range []string{"width", "height"} {
		set("VegetationPreset", field, positiveRange)
	}

	set("VegetationPreset", "normal_texture", &jsonschema.Schema{AnyOf: []*jsonschema.Schema{identifier(), {Enum: []any{""}}}})
	set("VegetationPreset", "geometry", choice("crossed_cards", "shrub", "tree", "palm"))
	set("VegetationPreset", "wind_amplitude", number(0, worldvegetation.MaxWindAmplitude))
	set("VegetationPreset", "wind_frequency", number(0, worldvegetation.MaxWindFrequency))
	set("VegetationPreset", "draw_distance", number(1, worldvegetation.MaxDrawDistance))
	set("VegetationLayer", "preset", identifier())
	set("VegetationLayer", "density", number(0, worldvegetation.MaxDensity))
	set("VegetationLayer", "surface", choice("floor", "ceiling"))
	set("VegetationLayer", "seed", &jsonschema.Schema{Type: "integer", Minimum: new(float64(0)), Maximum: new(float64(4294967295))})
	set("Room", "vegetation_layers", array(&jsonschema.Schema{Ref: "#/$defs/VegetationLayer"}, 0, worldvegetation.MaxLayers))

	g.definitions["Vegetation"].AllOf = append(g.definitions["Vegetation"].AllOf, &jsonschema.Schema{
		If: &jsonschema.Schema{
			Properties: map[string]*jsonschema.Schema{"version": {ExclusiveMaximum: new(float64(worldvegetation.PalmVersion))}},
		},
		Then: &jsonschema.Schema{Properties: map[string]*jsonschema.Schema{
			"presets": {
				Items: &jsonschema.Schema{Properties: map[string]*jsonschema.Schema{"geometry": choice("crossed_cards", "shrub", "tree")}},
			},
		}},
	})
	g.definitions["Document"].AllOf = append(g.definitions["Document"].AllOf, &jsonschema.Schema{
		If: &jsonschema.Schema{
			Properties: map[string]*jsonschema.Schema{"version": {ExclusiveMaximum: new(float64(worldsource.VegetationPalmVersion))}},
		},
		Then: &jsonschema.Schema{Properties: map[string]*jsonschema.Schema{
			"vegetation": {
				Properties: map[string]*jsonschema.Schema{
					"version": {
						Enum: []any{
							worldvegetation.Version,
							worldvegetation.GeometryVersion,
							worldvegetation.NormalVersion,
							worldvegetation.TreeVersion,
						},
					},
				},
			},
		}},
	})
	g.definitions["Vegetation"].AllOf = append(g.definitions["Vegetation"].AllOf, &jsonschema.Schema{
		If: &jsonschema.Schema{
			Properties: map[string]*jsonschema.Schema{"version": {ExclusiveMaximum: new(float64(worldvegetation.TreeVersion))}},
		},
		Then: &jsonschema.Schema{
			Properties: map[string]*jsonschema.Schema{
				"presets": {
					Items: &jsonschema.Schema{Properties: map[string]*jsonschema.Schema{"geometry": choice("crossed_cards", "shrub")}},
				},
			},
		},
	})
	g.definitions["Document"].AllOf = append(g.definitions["Document"].AllOf, &jsonschema.Schema{
		If: &jsonschema.Schema{
			Properties: map[string]*jsonschema.Schema{"version": {ExclusiveMaximum: new(float64(worldsource.VegetationTreeVersion))}},
		},
		Then: &jsonschema.Schema{
			Properties: map[string]*jsonschema.Schema{
				"vegetation": {
					Properties: map[string]*jsonschema.Schema{
						"version": {Enum: []any{worldvegetation.Version, worldvegetation.GeometryVersion, worldvegetation.NormalVersion}},
					},
				},
			},
		},
	})
	g.definitions["Vegetation"].AllOf = append(g.definitions["Vegetation"].AllOf, &jsonschema.Schema{
		If: &jsonschema.Schema{
			Properties: map[string]*jsonschema.Schema{"version": {ExclusiveMaximum: new(float64(worldvegetation.NormalVersion))}},
		},
		Then: &jsonschema.Schema{
			Properties: map[string]*jsonschema.Schema{
				"presets": {Items: &jsonschema.Schema{Properties: map[string]*jsonschema.Schema{"normal_texture": deny()}}},
			},
		},
	})
	g.definitions["Document"].AllOf = append(g.definitions["Document"].AllOf, &jsonschema.Schema{
		If: &jsonschema.Schema{
			Properties: map[string]*jsonschema.Schema{"version": {ExclusiveMaximum: new(float64(worldsource.VegetationNormalVersion))}},
		},
		Then: &jsonschema.Schema{
			Properties: map[string]*jsonschema.Schema{
				"vegetation": {
					Properties: map[string]*jsonschema.Schema{
						"version": {Enum: []any{worldvegetation.Version, worldvegetation.GeometryVersion}},
					},
				},
			},
		},
	})
	g.definitions["Vegetation"].AllOf = append(g.definitions["Vegetation"].AllOf, &jsonschema.Schema{
		If: &jsonschema.Schema{Properties: map[string]*jsonschema.Schema{"version": {Enum: []any{worldvegetation.Version}}}},
		Then: &jsonschema.Schema{
			Properties: map[string]*jsonschema.Schema{
				"presets": {Items: &jsonschema.Schema{Properties: map[string]*jsonschema.Schema{"geometry": deny()}}},
			},
		},
	})
	g.definitions["Document"].AllOf = append(g.definitions["Document"].AllOf, &jsonschema.Schema{
		If: &jsonschema.Schema{
			Properties: map[string]*jsonschema.Schema{
				"version": {ExclusiveMaximum: new(float64(worldsource.VegetationGeometryVersion))},
			},
		},
		Then: &jsonschema.Schema{
			Properties: map[string]*jsonschema.Schema{
				"vegetation": {Properties: map[string]*jsonschema.Schema{"version": {Enum: []any{worldvegetation.Version}}}},
			},
		},
	})
	g.definitions["Document"].AllOf = append(g.definitions["Document"].AllOf, &jsonschema.Schema{
		If: &jsonschema.Schema{
			Properties: map[string]*jsonschema.Schema{"version": {ExclusiveMaximum: new(float64(worldsource.VegetationVersion))}},
		},
		Then: &jsonschema.Schema{Properties: map[string]*jsonschema.Schema{
			"vegetation": deny(),
			"rooms":      {Items: &jsonschema.Schema{Properties: map[string]*jsonschema.Schema{"vegetation_layers": deny()}}},
			"prefabs": {
				Items: &jsonschema.Schema{
					Properties: map[string]*jsonschema.Schema{
						"rooms": {Items: &jsonschema.Schema{Properties: map[string]*jsonschema.Schema{"vegetation_layers": deny()}}},
					},
				},
			},
		}},
	})
}
