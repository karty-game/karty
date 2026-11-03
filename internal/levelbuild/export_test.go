package levelbuild

import (
	"testing"

	"github.com/karty-game/karty/internal/sdk"
)

// BuildAllWithFixtureMaterials exposes only the deterministic test provider to
// external-package world fixtures; it is absent from the production API.
func BuildAllWithFixtureMaterials(t *testing.T, root string, uiSchema uint32, theme string, manifest sdk.Manifest) ([]Artifact, error) {
	t.Helper()

	return buildAllWithFixtureMaterials(t, root, uiSchema, theme, manifest)
}

// BuildNamedWithFixtureMaterials keeps named-world fixtures independent of a GPU.
func BuildNamedWithFixtureMaterials(
	t *testing.T,
	root, name string,
	uiSchema uint32,
	theme string,
	manifest sdk.Manifest,
) (Artifact, error) {
	t.Helper()

	return buildNamed(t.Context(), root, name, uiSchema, theme, fixtureAssetBuild(t, root, manifest))
}
