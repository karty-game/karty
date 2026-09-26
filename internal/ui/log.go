package ui

import (
	"io"
	"log/slog"

	"github.com/charmbracelet/log"
)

// ConfigureLogging installs Charm's structured logger as the slog default.
func ConfigureLogging(output io.Writer) {
	logger := log.NewWithOptions(output, log.Options{
		ReportCaller:    false,
		ReportTimestamp: false,
	})
	log.SetDefault(logger)
	slog.SetDefault(slog.New(logger))
}
