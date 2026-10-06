package build

import (
	"testing"

	uicompiler "github.com/karty-game/karty-ui/compiler"
	ui "github.com/karty-game/karty-ui/schema"
	"github.com/karty-game/karty/internal/sdk"
)

func TestWidgetSDKCompatibility(t *testing.T) {
	t.Parallel()

	views := []uicompiler.Component{{Source: "widgets.kui", Template: ui.Template{Version: currentUISchema}}}
	manifest := sdk.Manifest{Version: "0.0.8"}

	manifest.API.Version = "0.0.6"
	if err := validateWidgetSDK(views, manifest); err == nil {
		t.Fatal("old SDK accepted typed widget bindings")
	}

	manifest.Version, manifest.API.Version = "0.0.9", "0.0.7"
	if err := validateWidgetSDK(views, manifest); err != nil {
		t.Fatal(err)
	}

	if sdkUISchema(manifest) != currentUISchema {
		t.Fatal("level UI schema did not advance")
	}
}
