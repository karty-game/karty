package build

import (
	"os"
	"os/exec"
	"testing"
)

func TestBrowserLauncher(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node unavailable; required by mise run check-web")
	}

	for _, mode := range []string{"sound-present", "sound-absent", "sound-duplicate", "http-failure", "wasm-fallback", "delayed-presentation", "inactive-camera-presentation"} {
		cmd := exec.CommandContext(t.Context(), "node", "testdata/launcher.test.mjs", "--mock", "templates/launcher.js.tmpl")

		cmd.Env = append(os.Environ(), "KARTY_TEST_HTTP_FAILURE=0", "KARTY_TEST_WASM_FALLBACK=0", "KARTY_TEST_SOUND_SECTIONS=present",
			"KARTY_TEST_PRESENTATION_DELAY=0", "KARTY_TEST_INACTIVE_CAMERA=0")
		if mode == "sound-absent" || mode == "sound-duplicate" {
			cmd.Env = append(cmd.Env, "KARTY_TEST_SOUND_SECTIONS="+mode[len("sound-"):])
		}

		if mode == "http-failure" {
			cmd.Env = append(cmd.Env, "KARTY_TEST_HTTP_FAILURE=1")
		}

		if mode == "wasm-fallback" {
			cmd.Env = append(cmd.Env, "KARTY_TEST_WASM_FALLBACK=1")
		}

		if mode == "delayed-presentation" || mode == "inactive-camera-presentation" {
			cmd.Env = append(cmd.Env, "KARTY_TEST_PRESENTATION_DELAY=1")
		}

		if mode == "inactive-camera-presentation" {
			cmd.Env = append(cmd.Env, "KARTY_TEST_INACTIVE_CAMERA=1")
		}

		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("browser launcher (%s): %v\n%s", mode, err, output)
		}
	}
}
