package toolchain

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

// TestMaterializeRelease is the only network/native execution check. It uses
// public inline release pins, no engine source, and requires explicit opt-in.
//
//nolint:paralleltest // Changes process environment and the default HTTP client.
func TestMaterializeRelease(t *testing.T) {
	if os.Getenv("KARTY_TEST_MATERIALIZE_RELEASE") != "1" {
		t.Skip("set KARTY_TEST_MATERIALIZE_RELEASE=1 for real release download/native smoke")
	}

	manifest := materializeReleaseManifest()
	if _, found := manifest.Artifacts.Materialize[runtime.GOOS+"-"+runtime.GOARCH]; !found {
		t.Skip("no released Materialize for this platform")
	}

	isolatedMaterializeHome(t)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()

	paths, err := Ensure(ctx, manifest, EnsureOptions{NeedMaterialize: true})
	if err != nil {
		t.Fatal(err)
	}

	// A warm install must not consult the network.
	setMaterializeTransport(t, func(_ *http.Request) (*http.Response, error) {
		t.Fatal("warm release installation reached network")

		return nil, os.ErrInvalid
	})

	warm, err := Ensure(ctx, manifest, EnsureOptions{NeedMaterialize: true})
	if err != nil || paths.Materialize == "" || warm.Materialize != paths.Materialize {
		t.Fatalf("warm release install: %+v, %v", warm, err)
	}

	t.Logf("verified native Materialize executable: %s", paths.Materialize)

	for _, argument := range []string{"--version", "--help", "--list-maps"} {
		// #nosec G204 -- Explicit opt-in execution of the checksum-verified managed binary, with fixed non-GPU arguments.
		output, err := exec.CommandContext(ctx, paths.Materialize, argument).CombinedOutput()
		if err != nil || len(output) == 0 {
			t.Fatalf("%s: %v\n%s", argument, err, output)
		}

		t.Logf("%s:\n%s", argument, output)
	}
}
