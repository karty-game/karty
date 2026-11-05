package levelbuild

import (
	"fmt"
	"slices"

	"github.com/karty-game/karty-sdk/bake/reflectionplan"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldreflection"
)

// A valid source digest and image set do not establish complete coverage. Match
// every planned environment and owner offline before accepting a required bake.
func validateRequiredReflectionCoverage(capture worldreflection.Bundle, document *world.Document) error {
	if len(capture.Probes) == 0 {
		return fmt.Errorf("required reflection capture has no environments: %w", ErrManifest)
	}

	planned, err := reflectionplan.Plan(document, capture.InputSHA256)
	if err != nil {
		return err
	}

	if len(capture.Probes) != len(planned.Probes) || !slices.Equal(capture.Sectors, planned.Sectors) ||
		!slices.Equal(capture.Solids, planned.Solids) {
		return fmt.Errorf(
			"required reflection capture does not cover the complete environment plan; recapture reflections: %w",
			ErrManifest,
		)
	}

	for index, probe := range capture.Probes {
		want := planned.Probes[index]
		if probe.ID != want.ID || probe.Origin != want.Origin || probe.Low != want.Low || probe.High != want.High {
			return fmt.Errorf(
				"required reflection capture environment %d does not match the planned placement; recapture reflections: %w",
				probe.ID,
				ErrManifest,
			)
		}
	}

	return nil
}
