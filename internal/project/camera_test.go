package project_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty/internal/project"
	"github.com/karty-game/karty/internal/release"
)

func TestProjectCameraDefaultsAndBounds(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name, fields   string
		valid, enabled bool
	}{
		{"2d larger screen", "[project.resolution]\nwidth=1920\nheight=1080\n", true, false},
		{"defaults", "[project.camera]\n", true, true},
		{"largest supported camera", "[project.resolution]\nwidth=1280\nheight=720\n[project.camera]\nfov_y_degrees=150\nnear=0.1\nfar=2000000\northo_height=2000000\n", true, true},
		{"camera too wide", "[project.resolution]\nwidth=1281\n[project.camera]\n", false, true},
		{"camera too tall", "[project.resolution]\nheight=721\n[project.camera]\n", false, true},
		{"fov too small", "[project.camera]\nfov_y_degrees=4.9\n", false, true},
		{"fov too large", "[project.camera]\nfov_y_degrees=150.1\n", false, true},
		{"fov not finite", "[project.camera]\nfov_y_degrees=nan\n", false, true},
		{"zero near", "[project.camera]\nnear=0\n", false, true},
		{"near underflow", "[project.camera]\nnear=1e-50\n", false, true},
		{"far before near", "[project.camera]\nnear=80\nfar=40\n", false, true},
		{"float32 clip collapse", "[project.camera]\nnear=1000\nfar=1000.000001\n", false, true},
		{"far too large", "[project.camera]\nfar=2000001\n", false, true},
		{"nonfinite far", "[project.camera]\nfar=inf\n", false, true},
		{"zero ortho", "[project.camera]\northo_height=0\n", false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()

			manifest := "[project]\nname='camera'\n[sdk]\nversion='" + release.SDKVersion() + "'\n" + test.fields
			if err := os.WriteFile(filepath.Join(directory, "karty.toml"), []byte(manifest), 0o600); err != nil {
				t.Fatal(err)
			}

			config, err := project.Load(directory)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%t: %v", test.valid, err)
			}

			if !test.valid {
				return
			}

			if config.Project.Camera.Enabled != test.enabled {
				t.Fatalf("camera enabled=%t, expected %t", config.Project.Camera.Enabled, test.enabled)
			}

			if test.name == "defaults" && (config.Project.Camera.FOVYDegrees != 60 || config.Project.Camera.Near != .05 ||
				config.Project.Camera.Far != 80 || config.Project.Camera.OrthoHeight != 22) {
				t.Fatalf("camera defaults: %+v", config.Project.Camera)
			}
		})
	}
}
