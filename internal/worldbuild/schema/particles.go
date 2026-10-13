package schema

import (
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/karty-game/karty-sdk/format/worldparticles"
	"github.com/karty-game/karty-sdk/format/worldsource"
)

func (g *grammar) particleConstraints() {
	set := func(name, field string, s *jsonschema.Schema) { g.definitions[name].Properties[field] = s }
	array := func(items *jsonschema.Schema, minimum, maximum int) *jsonschema.Schema {
		return &jsonschema.Schema{Type: "array", Items: items, MinItems: new(minimum), MaxItems: new(maximum)}
	}
	rangeSchema := func(minimum, maximum float64) *jsonschema.Schema {
		return &jsonschema.Schema{
			Type: "object", Required: []string{"min", "max"}, AdditionalProperties: deny(),
			Properties: map[string]*jsonschema.Schema{"min": number(minimum, maximum), "max": number(minimum, maximum)},
		}
	}

	g.definitions["Document"].Properties["particles"] = g.definitions["Document"].Properties["particles"].Then

	for _, scope := range []string{"Room", "Prefab"} {
		set(scope, "particle_emitters", array(&jsonschema.Schema{Ref: "#/$defs/ParticleEmitter"}, 0, worldparticles.MaxEmitters))
	}

	for name, fields := range map[string][]string{
		"Particles": {"version", "presets"}, "ParticlePreset": {"name", "capacity", "emission", "lifetime_seconds", "spawn", "sprite", "size_over_life", "color_over_life"}, "ParticleSpawn": {"shape"}, "ParticleSprite": {"frames", "playback", "facing", "blend"}, "ParticleEmitter": {"id", "preset", "position", "seed", "playback"}, "Range": {"min", "max"}, "ScalarKey": {"age", "value"}, "ColorKey": {"age", "rgba"}, "Playback": {"mode"}, "Burst": {"at_seconds", "count"},
	} {
		g.definitions[name].Required = fields
	}

	set("Particles", "version", &jsonschema.Schema{Enum: []any{worldparticles.Version, worldparticles.AlphaVersion}})
	set("Particles", "presets", array(&jsonschema.Schema{Ref: "#/$defs/ParticlePreset"}, 1, worldparticles.MaxPresets))
	set("Particles", "emitters", array(&jsonschema.Schema{Ref: "#/$defs/ParticleEmitter"}, 0, worldparticles.MaxEmitters))
	set(
		"ParticlePreset",
		"capacity",
		&jsonschema.Schema{Type: "integer", Minimum: new(float64(1)), Maximum: new(float64(worldparticles.MaxCapacity))},
	)
	set("Emission", "rate", number(0, worldparticles.MaxRate))
	set("Emission", "duration_seconds", number(0, worldparticles.MaxSeconds))
	set("ParticlePreset", "lifetime_seconds", rangeSchema(.001, worldparticles.MaxSeconds))
	set("ParticlePreset", "rotation_degrees", rangeSchema(-360000, 360000))
	set("ParticlePreset", "rotation_speed_degrees", rangeSchema(-360000, 360000))
	set("Emission", "bursts", array(&jsonschema.Schema{Ref: "#/$defs/Burst"}, 0, worldparticles.MaxBursts))
	set("Burst", "at_seconds", number(0, worldparticles.MaxSeconds))
	set("Burst", "count", &jsonschema.Schema{Type: "integer", Minimum: new(float64(1)), Maximum: new(float64(worldparticles.MaxCapacity))})
	set("ParticleSpawn", "shape", choice("point", "box"))
	set("ParticleSpawn", "half_extent", vector("xyz", 0, 1000000))
	set("ParticlePreset", "velocity_min", vector("xyz", -worldparticles.MaxVelocity, worldparticles.MaxVelocity))
	set("ParticlePreset", "velocity_max", vector("xyz", -worldparticles.MaxVelocity, worldparticles.MaxVelocity))
	set("ParticlePreset", "acceleration", vector("xyz", -worldparticles.MaxAcceleration, worldparticles.MaxAcceleration))
	set("ParticlePreset", "drag", number(0, 1000))
	set("ParticleSprite", "frames", array(identifier(), 1, worldparticles.MaxFrames))
	set("ParticleSprite", "fps", number(0, 1000))
	set("ParticleSprite", "playback", choice("once", "cycle"))
	set("ParticleSprite", "facing", choice("camera-facing", "upright"))
	set("ParticleSprite", "blend", choice("additive", "cutout", "alpha", "premultiplied"))
	g.definitions["Particles"].AllOf = append(g.definitions["Particles"].AllOf, &jsonschema.Schema{
		If: &jsonschema.Schema{Properties: map[string]*jsonschema.Schema{"version": {Enum: []any{worldparticles.Version}}}},
		Then: &jsonschema.Schema{Properties: map[string]*jsonschema.Schema{
			"presets": {Items: &jsonschema.Schema{Properties: map[string]*jsonschema.Schema{
				"sprite": {Properties: map[string]*jsonschema.Schema{"blend": choice("additive", "cutout")}},
			}}},
		}},
	})
	g.definitions["Document"].AllOf = append(g.definitions["Document"].AllOf, &jsonschema.Schema{
		If: &jsonschema.Schema{
			Properties: map[string]*jsonschema.Schema{"version": {ExclusiveMaximum: new(float64(worldsource.AlphaParticlesVersion))}},
		},
		Then: &jsonschema.Schema{Properties: map[string]*jsonschema.Schema{
			"particles": {Properties: map[string]*jsonschema.Schema{"version": {Enum: []any{worldparticles.Version}}}},
		}},
	})

	g.definitions["ParticleSprite"].AllOf = append(g.definitions["ParticleSprite"].AllOf, &jsonschema.Schema{
		If: &jsonschema.Schema{Properties: map[string]*jsonschema.Schema{"frames": {MinItems: new(2)}}},
		Then: &jsonschema.Schema{
			Required: []string{"fps"},
			Properties: map[string]*jsonschema.Schema{
				"fps": {Type: "number", ExclusiveMinimum: new(float64(0)), Maximum: new(float64(1000))},
			},
		},
	})

	set("ParticlePreset", "size_over_life", array(&jsonschema.Schema{Ref: "#/$defs/ScalarKey"}, 2, worldparticles.MaxCurveKeys))
	set("ParticlePreset", "color_over_life", array(&jsonschema.Schema{Ref: "#/$defs/ColorKey"}, 2, worldparticles.MaxCurveKeys))
	set("ScalarKey", "age", number(0, 1))
	set("ScalarKey", "value", number(0, worldparticles.MaxSize))
	set("ColorKey", "age", number(0, 1))
	set("ColorKey", "rgba", array(number(0, 1), 4, 4))
	set("Playback", "mode", choice("once", "cycle"))
	set("Playback", "period_seconds", number(0, worldparticles.MaxSeconds))
	set("Playback", "delay_seconds", number(0, worldparticles.MaxSeconds))
	set("ParticleEmitter", "scale", &jsonschema.Schema{AnyOf: []*jsonschema.Schema{
		{Enum: []any{float64(0)}}, number(.000001, 1000),
	}})
	set("ParticleEmitter", "yaw_degrees", number(-360000, 360000))
	set("ParticleEmitter", "seed", &jsonschema.Schema{Type: "integer", Minimum: new(float64(0)), Maximum: new(float64(4294967295))})

	g.definitions["Playback"].AllOf = append(g.definitions["Playback"].AllOf, &jsonschema.Schema{
		If: &jsonschema.Schema{Properties: map[string]*jsonschema.Schema{"mode": choice("cycle")}},
		Then: &jsonschema.Schema{
			Required:   []string{"period_seconds"},
			Properties: map[string]*jsonschema.Schema{"period_seconds": number(.001, worldparticles.MaxSeconds)},
		},
		Else: &jsonschema.Schema{Properties: map[string]*jsonschema.Schema{"period_seconds": {Enum: []any{float64(0)}}}},
	})
}
