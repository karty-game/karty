// Package serve defines the local web preview command.
package serve

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/urfave/cli/v3"
)

type staticError string

func (err staticError) Error() string {
	return string(err)
}

const (
	errTooManyDirectories staticError = "serve accepts at most one directory"
	errServeArguments     staticError = "serve directory and address are required"
	serverTimeout                     = 10 * time.Second
)

// Command creates the web preview command.
func Command() *cli.Command {
	return &cli.Command{
		Name:      "serve",
		Usage:     "serve a built web target",
		ArgsUsage: "[directory]",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "dir", Value: "dist/web", Usage: "web target directory"},
			&cli.StringFlag{Name: "addr", Value: "127.0.0.1:8080", Usage: "HTTP listen address"},
		},
		Action: run,
	}
}

func run(ctx context.Context, command *cli.Command) error {
	directory := command.String("dir")

	address := command.String("addr")
	if command.NArg() > 1 {
		return errTooManyDirectories
	}

	if command.NArg() == 1 {
		directory = command.Args().First()
	}

	if directory == "" || address == "" {
		return errServeArguments
	}

	return Serve(ctx, directory, address)
}

// Serve hosts a directory until the context is canceled.
func Serve(ctx context.Context, directory, address string) error {
	if directory == "" || address == "" {
		return errServeArguments
	}

	server := &http.Server{
		Addr:              address,
		Handler:           http.FileServer(http.Dir(directory)),
		ReadHeaderTimeout: serverTimeout,
	}

	go func() {
		<-ctx.Done()

		shutdownContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), serverTimeout)
		defer cancel()

		_ = server.Shutdown(shutdownContext)
	}()

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
