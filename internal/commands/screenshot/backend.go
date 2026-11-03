package screenshot

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
)

const (
	backendHardware = "hardware"
	backendSoftware = "software"
)

// CaptureOptions contains backend arguments admitted for the selected host.
// A zero value selects the current hardware backend.
type CaptureOptions struct{ backend string }

// BackendFlag exposes the same explicit capture choice on both commands.
func BackendFlag() cli.Flag {
	return &cli.StringFlag{
		Name: "backend", Value: backendHardware,
		Usage:     "native capture backend: hardware or software (software requires WebGPU)",
		Validator: validateBackend,
	}
}

func validateBackend(value string) error {
	if value != backendHardware && value != backendSoftware {
		return fmt.Errorf("backend must be hardware or software: %w", os.ErrInvalid)
	}

	return nil
}

// CaptureBackend admits software only after validating the WebGPU host bundle.
// Call it before building level inputs; an unverified host is rejected.
func CaptureBackend(value string, webgpu bool) (CaptureOptions, error) {
	if err := validateBackend(value); err != nil {
		return CaptureOptions{}, err
	}

	if !webgpu {
		return CaptureOptions{}, fmt.Errorf("capture requires a validated WebGPU native host bundle: %w", os.ErrInvalid)
	}

	return CaptureOptions{backend: value}, nil
}

// Arguments returns an explicit current WebGPU backend selection.
func (options CaptureOptions) Arguments() []string {
	if options.backend == "" {
		return []string{"--backend", backendHardware}
	}

	return []string{"--backend", options.backend}
}
