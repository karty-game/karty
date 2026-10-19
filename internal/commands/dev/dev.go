// Package dev defines the persistent build-and-serve development command.
package dev

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	buildservice "github.com/karty-game/karty/internal/build"
	"github.com/karty-game/karty/internal/devserver"
	"github.com/karty-game/karty/internal/packages"
	"github.com/urfave/cli/v3"
)

type staticError string

func (err staticError) Error() string { return string(err) }

const (
	defaultPort                        = "4242"
	previewHeaderTimeout               = 10 * time.Second
	previewWriteTimeout                = 30 * time.Second
	previewIdleTimeout                 = 60 * time.Second
	previewShutdownTimeout             = 3 * time.Second
	errUnsupportedTarget   staticError = "unsupported dev target; only web is available (use karty build for native)"
	errHostNotFound        staticError = "web host not found; pass --host /path/to/karty-host.wasm or use the project's managed SDK"
	errHostNotFile         staticError = "web host is not a file"
	errInvalidPort         staticError = "dev port must be a number between 0 and 65535"
)

// Command creates the development server command. It needs no external watcher.
func Command() *cli.Command {
	return &cli.Command{
		Name: "dev", Usage: "preview your game as you edit",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "compiler", Value: "go", Usage: "development cartridge compiler: go or tinygo"},
			&cli.StringFlag{Name: "target", Value: "web", Usage: "development target"},
			&cli.StringFlag{Name: "host", Usage: "path to a pre-built host artifact"},
			&cli.StringFlag{Name: "go", Usage: "path to a Go executable"},
			&cli.StringFlag{Name: "tinygo", Usage: "path to a TinyGo executable"},
			&cli.StringFlag{Name: "wasm-tools", Usage: "path to a wasm-tools executable"},
			&cli.StringFlag{Name: "bind", Value: "127.0.0.1", Usage: "HTTP bind address (use 0.0.0.0 to share on your network)"},
			&cli.StringFlag{Name: "port", Value: defaultPort, Usage: "HTTP port (0 selects an available port)"},
		}, Action: run,
	}
}

type configuration struct {
	directory string
	address   string
	build     buildservice.Options
}

func run(ctx context.Context, command *cli.Command) error {
	config, err := configure(command)
	if err != nil {
		return err
	}

	return serveDevelopment(ctx, command, config)
}

func configure(command *cli.Command) (configuration, error) {
	if command.String("target") != "web" {
		return configuration{}, errUnsupportedTarget
	}

	compiler := command.String("compiler")
	if compiler != "go" && compiler != "tinygo" {
		return configuration{}, fmt.Errorf("development compiler must be go or tinygo: %w", os.ErrInvalid)
	}

	port, err := strconv.Atoi(command.String("port"))
	if err != nil || port < 0 || port > 65535 {
		return configuration{}, errInvalidPort
	}

	host, err := devHost(command.String("host"))
	if err != nil {
		return configuration{}, err
	}

	directory, err := os.Getwd()
	if err != nil {
		return configuration{}, err
	}

	return configuration{directory: directory, address: net.JoinHostPort(command.String("bind"), strconv.Itoa(port)),
		build: developmentOptions(host, compiler, command.String("go"), command.String("tinygo"), command.String("wasm-tools"))}, nil
}

func serveDevelopment(ctx context.Context, command *cli.Command, config configuration) error {
	return serve(ctx, command, config, func(buildCtx context.Context, progress func(string)) error {
		options := config.build
		options.Progress = progress

		return buildservice.RunWithOptions(buildCtx, config.directory, options)
	})
}

func serve(ctx context.Context, command *cli.Command, config configuration, build func(context.Context, func(string)) error) error {
	if ctx.Err() != nil {
		return nil
	}

	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	listenerConfig := net.ListenConfig{}

	listener, err := listenerConfig.Listen(workCtx, "tcp", config.address)
	if err != nil {
		return err
	}
	defer listener.Close()

	transport, err := devserver.NewServer()
	if err != nil {
		return err
	}
	defer transport.Close()

	server := &http.Server{Handler: transport, ReadHeaderTimeout: previewHeaderTimeout,
		WriteTimeout: previewWriteTimeout, IdleTimeout: previewIdleTimeout,
		BaseContext: func(net.Listener) context.Context { return workCtx }}
	defer server.Close()

	serving := make(chan error, 1)
	go func() { serving <- server.Serve(listener); cancel() }()

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)

		<-workCtx.Done()
		transport.Close()

		// The work context is already canceled. Give HTTP handlers a bounded
		// cleanup window while the build worker joins its own canceled work.
		cleanupCtx, stop := context.WithTimeout(context.WithoutCancel(workCtx), previewShutdownTimeout)
		defer stop()

		_ = server.Shutdown(cleanupCtx)
		_ = server.Close()
	}()

	printAddress(command, listener.Addr().String())

	err = devserver.Run(workCtx, transport, devserver.RunOptions{
		Directory: config.directory,
		Build:     build,
		Log:       func(message string) { fmt.Fprintln(command.ErrWriter, message) },
		WatchRoots: func(ctx context.Context) (map[string]string, error) {
			return packages.FolderRoots(ctx, config.directory)
		},
	})

	cancel()
	<-stopped

	serveErr := <-serving

	if err != nil {
		return err
	}

	if !errors.Is(serveErr, http.ErrServerClosed) {
		return serveErr
	}

	return nil
}

func printAddress(command *cli.Command, address string) {
	fmt.Fprintf(command.Writer, "Karty dev watching project files at http://%s/\n", address)

	if template := os.Getenv("VSCODE_PROXY_URI"); strings.Contains(template, "{{port}}") {
		_, port, err := net.SplitHostPort(address)
		if err == nil {
			fmt.Fprintf(command.Writer, "Workspace preview: %s\n", strings.ReplaceAll(template, "{{port}}", port))
		}
	}
}

func developmentOptions(host, compiler, goPath, tinygo, wasmTools string) buildservice.Options {
	return buildservice.Options{Target: "web", Compiler: compiler, Go: goPath, TinyGo: tinygo, WasmTools: wasmTools,
		Host: host, Development: true, DevelopmentBakes: true, BundledRuntime: true}
}

// Managed hosts stay SDK-pinned; local overrides are validated before listening.
func devHost(override string) (string, error) {
	host, err := findHost(override)
	if errors.Is(err, errHostNotFound) {
		return "", nil
	}

	return host, err
}

func findHost(explicit string) (string, error) {
	if explicit != "" {
		return existingHost(explicit)
	}

	if environment := os.Getenv("KARTY_HOST_WEB"); environment != "" {
		return existingHost(environment)
	}

	if executable, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(executable), "..", "..", "host", "web", "karty-host.wasm")
		if path, pathErr := existingHost(candidate); pathErr == nil {
			return path, nil
		}
	}

	directory, err := os.Getwd()
	if err == nil {
		for current := directory; current != filepath.Dir(current); current = filepath.Dir(current) {
			candidate := filepath.Join(current, "dist", "host", "web", "karty-host.wasm")
			if path, pathErr := existingHost(candidate); pathErr == nil {
				return path, nil
			}
		}
	}

	return "", errHostNotFound
}

func existingHost(path string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}

	//nolint:gosec // The caller explicitly selects a host executable; regular-file validation follows.
	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}

	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s: %w", absolute, errHostNotFile)
	}

	return absolute, nil
}
