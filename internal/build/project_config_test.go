package build

import (
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/project"
)

func TestProjectConfigConvertsAuthoredFOVDegrees(t *testing.T) {
	t.Parallel()

	var config project.Config

	config.Project.Resolution.Width, config.Project.Resolution.Height = 1280, 720
	config.Project.Camera = project.CameraConfig{Enabled: true, FOVYDegrees: 90, Near: .1, Far: 250, OrthoHeight: 30}

	source, err := projectConfigFile(config)
	if err != nil {
		t.Fatal(err)
	}

	for _, declaration := range []string{"ResolutionWidth   uint32  = 1280", "ResolutionHeight  uint32  = 720",
		"CameraFOVY        float32 = 1.5707964", "CameraNear        float32 = 0.1", "CameraFar         float32 = 250",
		"CameraOrthoHeight float32 = 30"} {
		if !strings.Contains(string(source), declaration) {
			t.Fatalf("missing %q in adapter:\n%s", declaration, source)
		}
	}
}
