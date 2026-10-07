package build

import (
	"fmt"

	uicompiler "github.com/karty-game/karty-ui/compiler"
	ui "github.com/karty-game/karty-ui/schema"
	"github.com/karty-game/karty/internal/sdk"
)

func sdkUISchema(manifest sdk.Manifest) uint32 {
	return manifest.Compatibility.UISchema
}

func validateWidgetSDK(views []uicompiler.Component, manifest sdk.Manifest) error {
	for _, view := range views {
		if view.Template.Version > sdkUISchema(manifest) {
			return fmt.Errorf(
				"%s requires UI schema %d; SDK %s supports schema %d: %w",
				view.Source,
				view.Template.Version,
				manifest.Version,
				sdkUISchema(manifest),
				ui.ErrTemplate,
			)
		}
	}

	return nil
}
