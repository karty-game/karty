package build

import (
	sdkperformance "github.com/karty-game/karty-sdk/format/performance"
	"github.com/karty-game/karty/internal/project"
)

func runtimePerformance(authored project.PerformanceConfig) sdkperformance.Config {
	config := sdkperformance.Config{
		TargetFPS:      authored.TargetFPS,
		DefaultProfile: authored.DefaultProfile,
		Profiles:       make([]sdkperformance.Profile, len(authored.Profiles)),
	}
	for index, profile := range authored.Profiles {
		config.Profiles[index] = sdkperformance.Profile{
			Name:             profile.Name,
			RenderScale:      profile.RenderScale,
			MinWindowWidth:   profile.MinWindowWidth,
			MaxWindowWidth:   profile.MaxWindowWidth,
			FloorReflections: profile.FloorReflections,
		}
	}

	return config
}
