// Package release defines the CLI build identity and selected SDK.
package release

// Version identifies the CLI build, independently of its selected SDK.
// Release builds set it from their tag through Go linker flags.
var Version = "dev" //nolint:gochecknoglobals // Go linker flags require a variable for the release build identity.

// CurrentSDK is the SDK required by current projects, templates and samples.
const CurrentSDK = "0.0.19"

// SDKVersion returns the unprefixed version used by manifests and project files.
func SDKVersion() string { return CurrentSDK }
