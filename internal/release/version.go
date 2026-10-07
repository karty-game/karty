// Package release contains the SDK selected by CLI defaults and release checks.
package release

import "strings"

// CurrentSDK is the released default for new projects and compatibility checks.
const CurrentSDK = "v0.0.8"

// SampleSDK pins the examples to the current candidate. Run mise run
// prepare-release after changing it; the released CLI default stays independent.
const SampleSDK = "0.0.10"

// WorldCameraSDK pins the compiled wall-frame sample independently so future
// world capabilities can be tested before moving the other samples.
const WorldCameraSDK = "0.0.10"

// SDKVersion returns the unprefixed version used by manifests and project files.
func SDKVersion() string { return strings.TrimPrefix(CurrentSDK, "v") }
