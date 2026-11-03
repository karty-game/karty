package levelbuild

import (
	"fmt"
	"slices"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
	"github.com/pelletier/go-toml/v2"
)

type environmentLightmapSettings struct {
	Times           []float64 `toml:"times"`
	DefaultTime     *float64  `toml:"default_time"`
	WeatherVariants bool      `toml:"weather_variants"`
}

func validateEnvironmentLightmapKeys(contents []byte) error {
	var raw struct {
		Lightmap struct {
			Environment map[string]any `toml:"environment"`
		} `toml:"lightmap"`
	}
	if err := toml.Unmarshal(contents, &raw); err != nil {
		return ErrManifest
	}

	for key := range raw.Lightmap.Environment {
		if !slices.Contains([]string{"times", "default_time", "weather_variants"}, key) {
			return fmt.Errorf("unknown lightmap environment option %q: %w", key, ErrManifest)
		}
	}

	return nil
}

func (settings environmentLightmapSettings) validate() error {
	endpoints := 1
	if settings.WeatherVariants {
		endpoints = 2
	}

	if len(settings.Times) == 0 || len(settings.Times)*endpoints > worldlightmap.MaxEnvironmentVariants {
		return ErrManifest
	}

	defaultTime := world.DefaultEnvironmentTimeOfDay
	if settings.DefaultTime != nil {
		defaultTime = *settings.DefaultTime
	}

	found := false

	for i, time := range settings.Times {
		if world.ValidateEnvironmentSettings(world.EnvironmentSettings{TimeOfDay: time}) != nil || i > 0 && time <= settings.Times[i-1] {
			return fmt.Errorf("environment times must be unique ascending hours in [0,24): %w", ErrManifest)
		}

		found = found || time == defaultTime
	}

	if !found {
		return fmt.Errorf("environment default_time must belong to times: %w", ErrManifest)
	}

	return nil
}
