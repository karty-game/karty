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

	for _, mode := range []string{"normal", "http-failure", "wasm-fallback"} {
		cmd := exec.CommandContext(t.Context(), "node", "testdata/launcher.test.mjs", "--mock", "templates/launcher.js.tmpl")

		cmd.Env = append(os.Environ(), "KARTY_TEST_HTTP_FAILURE=0", "KARTY_TEST_WASM_FALLBACK=0")
		if mode == "http-failure" {
			cmd.Env = append(cmd.Env, "KARTY_TEST_HTTP_FAILURE=1")
		}

		if mode == "wasm-fallback" {
			cmd.Env = append(cmd.Env, "KARTY_TEST_WASM_FALLBACK=1")
		}

		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("browser launcher (%s): %v\n%s", mode, err, output)
		}
	}
}
