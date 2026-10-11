package project

import (
	"errors"
	"math"
	"slices"
)

var errInvalidPerformanceProfiles = errors.New("project performance profiles must define a descending high, medium, low, potato ladder")

// PerformanceConfig configures the host's adaptive render-quality ladder.
//
//nolint:tagliatelle // JSON keys match the versioned project performance configuration.
type PerformanceConfig struct {
	TargetFPS      float64              `json:"target_fps"      koanf:"target_fps"`
	DefaultProfile string               `json:"default_profile" koanf:"default_profile"`
	Profiles       []PerformanceProfile `json:"profiles"        koanf:"profiles"`
}

// PerformanceProfile names a render scale and optional inclusive window-width
// eligibility range. A nil FloorReflections value inherits the renderer default.
//
//nolint:tagliatelle // JSON keys match the versioned project performance configuration.
type PerformanceProfile struct {
	Name             string  `json:"name"                        koanf:"name"`
	RenderScale      float64 `json:"render_scale"                koanf:"render_scale"`
	MinWindowWidth   int     `json:"min_window_width,omitempty"  koanf:"min_window_width"`
	MaxWindowWidth   int     `json:"max_window_width,omitempty"  koanf:"max_window_width"`
	FloorReflections *bool   `json:"floor_reflections,omitempty" koanf:"floor_reflections"`
}

const (
	defaultTargetFPS     = 60
	highMinWindowWidth   = 1401
	potatoMaxWindowWidth = 799
	highRenderScale      = 1.5
	lowRenderScale       = .75
	potatoRenderScale    = .5
)

func performanceTierNames() []string {
	return []string{"high", "medium", "low", "potato"}
}

func defaultPerformanceConfig() PerformanceConfig {
	reflectionsOff := false

	return PerformanceConfig{
		TargetFPS:      defaultTargetFPS,
		DefaultProfile: "medium",
		Profiles: []PerformanceProfile{
			{Name: "high", RenderScale: highRenderScale, MinWindowWidth: highMinWindowWidth},
			{Name: "medium", RenderScale: 1},
			{Name: "low", RenderScale: lowRenderScale, FloorReflections: &reflectionsOff},
			{Name: "potato", RenderScale: potatoRenderScale, MaxWindowWidth: potatoMaxWindowWidth, FloorReflections: &reflectionsOff},
		},
	}
}

func normalizePerformanceConfig(authored PerformanceConfig) (PerformanceConfig, error) {
	resolved := defaultPerformanceConfig()
	if authored.TargetFPS != 0 {
		resolved.TargetFPS = authored.TargetFPS
	}

	if authored.DefaultProfile != "" {
		resolved.DefaultProfile = authored.DefaultProfile
	}

	if err := applyPerformanceOverrides(&resolved, authored.Profiles); err != nil {
		return PerformanceConfig{}, err
	}

	if math.IsNaN(resolved.TargetFPS) || math.IsInf(resolved.TargetFPS, 0) || resolved.TargetFPS < 1 || resolved.TargetFPS > 1000 ||
		!slices.Contains(performanceTierNames(), resolved.DefaultProfile) {
		return PerformanceConfig{}, errInvalidPerformanceProfiles
	}

	previousScale := math.Inf(1)
	for index, profile := range resolved.Profiles {
		if math.IsNaN(profile.RenderScale) || math.IsInf(profile.RenderScale, 0) ||
			profile.RenderScale < .25 || profile.RenderScale > 1.5 || profile.RenderScale >= previousScale ||
			profile.MinWindowWidth < 0 || profile.MaxWindowWidth < 0 ||
			profile.MinWindowWidth > 0 && profile.MaxWindowWidth > 0 && profile.MinWindowWidth > profile.MaxWindowWidth {
			return PerformanceConfig{}, errInvalidPerformanceProfiles
		}

		if index == 0 && profile.MinWindowWidth < highMinWindowWidth ||
			index == 3 && (profile.MaxWindowWidth == 0 || profile.MaxWindowWidth > potatoMaxWindowWidth) {
			return PerformanceConfig{}, errInvalidPerformanceProfiles
		}

		previousScale = profile.RenderScale
	}

	return resolved, nil
}

func applyPerformanceOverrides(resolved *PerformanceConfig, profiles []PerformanceProfile) error {
	seen := make(map[string]bool, len(profiles))
	for _, profile := range profiles {
		if seen[profile.Name] {
			return errInvalidPerformanceProfiles
		}

		seen[profile.Name] = true

		index := slices.Index(performanceTierNames(), profile.Name)
		if index < 0 {
			return errInvalidPerformanceProfiles
		}

		base := &resolved.Profiles[index]
		if profile.RenderScale != 0 {
			base.RenderScale = profile.RenderScale
		}

		if profile.MinWindowWidth != 0 {
			base.MinWindowWidth = profile.MinWindowWidth
		}

		if profile.MaxWindowWidth != 0 {
			base.MaxWindowWidth = profile.MaxWindowWidth
		}

		if profile.FloorReflections != nil {
			reflections := *profile.FloorReflections
			base.FloorReflections = &reflections
		}
	}

	return nil
}
