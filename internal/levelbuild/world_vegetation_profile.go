package levelbuild

// WebGPU V1 omits procedural plant placement and its bake transport together.
// Keep this on the build-owned definition so screenshot, whole-level and chunk
// paths cannot accidentally bake shadows for geometry that is not packaged.
func vegetationBuildProfile(definition manifest, assets *assetBuild) manifest {
	if assets != nil {
		definition.omitVegetation = true
	}

	return definition
}
