package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/karty-game/karty/internal/commands"
	"github.com/karty-game/karty/internal/ui"
)

func main() {
	ui.ConfigureLogging(os.Stderr)
	ui.ConfigureHelp()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := commands.New().Run(ctx, os.Args)

	stop()

	if err != nil {
		slog.Error("command failed", "error", err)
		os.Exit(1)
	}
}
