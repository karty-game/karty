package ui

import "fmt"

const binaryUnit = 1024

// Size formats an on-disk byte count using binary units.
func Size(bytes int64) string {
	if bytes < binaryUnit {
		return fmt.Sprintf("%d B", bytes)
	}

	value := float64(bytes)
	for _, unit := range []string{"KiB", "MiB", "GiB", "TiB"} {
		value /= binaryUnit
		if value < binaryUnit || unit == "TiB" {
			return fmt.Sprintf("%.1f %s", value, unit)
		}
	}

	return ""
}
