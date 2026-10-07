// Package release contains the SDK selected by CLI defaults and release checks.
package release

// CurrentSDK is the released default for new projects and compatibility checks.
const CurrentSDK = "0.0.10"

// SDKVersion returns the unprefixed version used by manifests and project files.
func SDKVersion() string { return CurrentSDK }
