package build

import (
	"testing"

	uicompiler "github.com/karty-game/karty-ui/compiler"
	ui "github.com/karty-game/karty-ui/schema"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

func TestWidgetSDKCompatibility(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	views := []uicompiler.Component{{Source: "widgets.kui", Template: ui.Template{Version: manifest.Compatibility.UISchema}}}
	if err := validateWidgetSDK(views, manifest); err != nil {
		t.Fatal(err)
	}

	manifest.Compatibility.UISchema--
	if err := validateWidgetSDK(views, manifest); err == nil {
		t.Fatal("SDK without required UI schema accepted widgets")
	}
}

func TestWidgetSchemaUsesBundleMetadata(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	expected := manifest.Compatibility.UISchema
	manifest.API.Version = "future-api"

	views := []uicompiler.Component{{Source: "widgets.kui", Template: ui.Template{Version: expected}}}
	if err := validateWidgetSDK(views, manifest); err != nil {
		t.Fatal(err)
	}

	if sdkUISchema(manifest) != expected {
		t.Fatal("API release number changed the declared widget schema")
	}
}
