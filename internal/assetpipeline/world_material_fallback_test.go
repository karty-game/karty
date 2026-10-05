package assetpipeline

import (
	"context"
	"os"
	"slices"
	"testing"
)

func TestMaterializeAutomaticSoftwareFallback(t *testing.T) {
	t.Parallel()

	environment := []string{"HOME=/fixture", "LIBGL_ALWAYS_SOFTWARE=0"}
	calls := 0

	err := materializeWithFallback(t.Context(), "linux", environment, func(env []string) (string, error) {
		calls++
		if calls == 1 {
			return "No GPU adapter available", os.ErrInvalid
		}

		for _, want := range []string{"MATERIALIZE_GPU_BACKEND=gl", "LIBGL_ALWAYS_SOFTWARE=1", "EGL_PLATFORM=surfaceless", "HOME=/fixture"} {
			if !slices.Contains(env, want) {
				t.Fatalf("fallback missing %s: %v", want, env)
			}
		}

		return "generated", nil
	})
	if err != nil || calls != 2 || environment[1] != "LIBGL_ALWAYS_SOFTWARE=0" {
		t.Fatalf("fallback err=%v calls=%d environment=%v", err, calls, environment)
	}
}

func TestMaterializeFallbackDoesNotHideFailures(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"explicit", "shader", "nonlinux", "canceled"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			var env []string

			system, output := "linux", "No GPU adapter available"

			switch kind {
			case "explicit":
				env = []string{"MATERIALIZE_GPU_BACKEND=vulkan"}
			case "shader":
				output = "shader compilation failed"
			case "nonlinux":
				system = "windows"
			case "canceled":
				cancel()
			}

			calls := 0

			err := materializeWithFallback(ctx, system, env, func([]string) (string, error) {
				calls++

				return output, os.ErrInvalid
			})
			if err == nil || calls != 1 {
				t.Fatalf("hidden failure: err=%v calls=%d", err, calls)
			}
		})
	}
}
