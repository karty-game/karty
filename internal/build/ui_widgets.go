package build

import (
	"fmt"

	uicompiler "github.com/karty-game/karty-ui/compiler"
	ui "github.com/karty-game/karty-ui/schema"
	"github.com/karty-game/karty/internal/sdk"
)

// currentUISchema mirrors the exported bundle contract independently of compiler symbols.
const currentUISchema uint32 = 11

func sdkUISchema(manifest sdk.Manifest) uint32 {
	if manifest.API.Version == "0.0.7" {
		return currentUISchema
	}

	return ui.SchemaInteractionPolish
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
