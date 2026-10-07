// Package release contains the SDK selected by CLI defaults and release checks.
package release

// CurrentSDK is the released default for new projects and compatibility checks.
const CurrentSDK = "0.0.10"

// SampleSDK keeps samples on the same SDK as CLI builds and tests.
const SampleSDK = CurrentSDK

// WorldCameraSDK shares the current SDK selection.
const WorldCameraSDK = CurrentSDK

// SDKVersion returns the unprefixed version used by manifests and project files.
func SDKVersion() string { return CurrentSDK }
