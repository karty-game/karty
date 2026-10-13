// Package release contains the SDK selected by CLI defaults and release checks.
package release

// CurrentSDK is the released default for new projects and compatibility checks.
const CurrentSDK = "0.0.12"

// SDKVersion returns the unprefixed version used by manifests and project files.
func SDKVersion() string { return CurrentSDK }

// SampleSDKVersion keeps new environment, physics and particle samples on the next candidate until release.
// All other samples and the CLI default continue to use the published SDK.
func SampleSDKVersion(name string) string {
	return CurrentSDK
}
