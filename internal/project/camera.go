package project

import (
	"math"

	"github.com/knadh/koanf/v2"
)

const errInvalidCamera staticError = "project.camera requires finite fov_y_degrees in [5,150], 0 < near < far <= 2000000, 0 < ortho_height <= 2000000, and project.resolution within 1280x720"

// CameraConfig contains optional build-time defaults for project Camera3D settings.
// Enabled distinguishes a declared camera table from a 2D-only project.
type CameraConfig struct {
	Enabled     bool    `koanf:"-"`
	FOVYDegrees float64 `koanf:"fov_y_degrees"`
	Near        float64 `koanf:"near"`
	Far         float64 `koanf:"far"`
	OrthoHeight float64 `koanf:"ortho_height"`
}

func loadCameraConfig(config *Config, manifest *koanf.Koanf) error {
	camera := &config.Project.Camera
	camera.Enabled = manifest.Exists("project.camera")

	for _, field := range []struct {
		name     string
		value    *float64
		fallback float64
	}{
		{"fov_y_degrees", &camera.FOVYDegrees, 60},
		{"near", &camera.Near, 0.05},
		{"far", &camera.Far, 80},
		{"ortho_height", &camera.OrthoHeight, 22},
	} {
		if !manifest.Exists("project.camera." + field.name) {
			*field.value = field.fallback
		}

		if math.IsNaN(*field.value) || math.IsInf(*field.value, 0) {
			return errInvalidCamera
		}
	}

	if camera.FOVYDegrees < 5 || camera.FOVYDegrees > 150 || camera.Near <= 0 ||
		camera.Far <= camera.Near || camera.Far > 2_000_000 || camera.OrthoHeight <= 0 || camera.OrthoHeight > 2_000_000 {
		return errInvalidCamera
	}

	if float32(camera.Near) <= 0 || float32(camera.Far) <= float32(camera.Near) || float32(camera.OrthoHeight) <= 0 {
		return errInvalidCamera
	}

	if camera.Enabled && (config.Project.Resolution.Width > 1280 || config.Project.Resolution.Height > 720) {
		return errInvalidCamera
	}

	return nil
}
