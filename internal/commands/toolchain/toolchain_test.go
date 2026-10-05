package toolchain

import (
	"testing"

	"github.com/karty-game/karty/internal/sdk"
)

func TestInstallSelectsMaterializeOnlyWithSDKPin(t *testing.T) {
	t.Parallel()

	for _, version := range []string{"", "2.0.0"} {
		manifest := sdk.Manifest{}
		manifest.Tools.Materialize = version
		options := installOptions(manifest)

		if !options.NeedGo || !options.NeedTinyGo || !options.NeedAir || !options.NeedWasmTools ||
			options.NeedMaterialize != (version != "") {
			t.Fatalf("unexpected selection for %q: %+v", version, options)
		}
	}
}
