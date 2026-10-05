package build

import (
	"math"

	"github.com/karty-game/karty-ui/codegen"
	"github.com/karty-game/karty/internal/project"
)

func projectConfigFile(config project.Config) ([]byte, error) {
	camera := config.Project.Camera

	return codegen.ProjectConfigFile(codegen.ProjectConfiguration{
		ResolutionWidth:   uint32(config.Project.Resolution.Width),
		ResolutionHeight:  uint32(config.Project.Resolution.Height),
		CameraFOVY:        float32(camera.FOVYDegrees * math.Pi / 180),
		CameraNear:        float32(camera.Near),
		CameraFar:         float32(camera.Far),
		CameraOrthoHeight: float32(camera.OrthoHeight),
	})
}
