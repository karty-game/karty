// Package release contains the SDK selected by CLI defaults and release checks.
package release

import "strings"

// CurrentSDK is the single SDK release pin. Run mise run prepare-release after changing it.
const CurrentSDK = "v0.0.8"

// SDKVersion returns the unprefixed version used by manifests and project files.
func SDKVersion() string { return strings.TrimPrefix(CurrentSDK, "v") }
