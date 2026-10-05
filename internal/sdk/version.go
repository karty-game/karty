package sdk

import (
	"strconv"
	"strings"
)

// MinimumVersion is the oldest supported development SDK release.
const MinimumVersion = "0.0.5"

func validateVersion(version string) error {
	if !versionPattern.MatchString(version) {
		return bundleError("invalid SDK version %q", version)
	}

	stable, _, prerelease := strings.Cut(version, "-")
	parts := strings.Split(stable, ".")
	minimum := strings.Split(MinimumVersion, ".")

	var values [3]uint64

	for index, part := range parts {
		value, err := strconv.ParseUint(part, 10, 64)
		if err != nil || len(part) > 1 && part[0] == '0' {
			return bundleError("invalid SDK version %q", version)
		}

		values[index] = value
	}

	for index, value := range values {
		floor, err := strconv.ParseUint(minimum[index], 10, 64)
		if err != nil {
			return err
		}

		if value > floor {
			return nil
		}

		if value < floor {
			return bundleError("SDK %s is unsupported; minimum supported SDK is %s", version, MinimumVersion)
		}
	}

	if prerelease {
		return bundleError("SDK %s is unsupported; minimum supported SDK is %s", version, MinimumVersion)
	}

	return nil
}
