// Package dev defines the build-and-serve development command.
package dev

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/karty-game/karty/internal/project"
	"github.com/karty-game/karty/internal/sdk"
	toolchainservice "github.com/karty-game/karty/internal/toolchain"
	"github.com/urfave/cli/v3"
)

const (
	defaultPort         = "4242"
	airRuntimeDirectory = ".karty/log"
	airBuildLog         = "build-errors.log"
)

type staticError string

func (err staticError) Error() string {
	return string(err)
}

const (
	errUnsupportedTarget staticError = "unsupported dev target; only web is available (build native with `karty build --host /path/to/karty-host`, then run the staged host)"
	errHostNotFound      staticError = "web host not found; build it with `mise run build-host-web` or pass --host /path/to/karty-host.wasm"
	errHostNotFile       staticError = "web host is not a file"
	errAirNotFound       staticError = "managed Air is unavailable; pass --air /path/to/air"
	errInvalidPort       staticError = "dev port must be a number between 1 and 65534"
	errPortInUse         staticError = "dev port is already in use"
)

// Command creates the development server command.
func Command() *cli.Command {
	return &cli.Command{
		Name:  "dev",
		Usage: "build and serve a project for development",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "target", Value: "web", Usage: "development target"},
			&cli.StringFlag{Name: "host", Usage: "path to a pre-built host artifact"},
			&cli.StringFlag{Name: "air", Usage: "path to an Air executable"},
			&cli.StringFlag{Name: "go", Usage: "path to a Go executable"},
			&cli.StringFlag{Name: "tinygo", Usage: "path to a TinyGo executable"},
			&cli.StringFlag{Name: "wasm-tools", Usage: "path to a wasm-tools executable"},
			&cli.StringFlag{Name: "bind", Value: "0.0.0.0", Usage: "HTTP bind address"},
			&cli.StringFlag{Name: "port", Value: defaultPort, Usage: "HTTP port"},
		},
		Action: run,
	}
}

func run(ctx context.Context, command *cli.Command) error {
	target := command.String("target")
	if target != "web" {
		return fmt.Errorf("%q: %w", target, errUnsupportedTarget)
	}

	config, err := project.Load(".")
	if err != nil {
		return err
	}

	manifest, err := sdk.Resolve(config.SDK.Version)
	if err != nil {
		return err
	}

	host, err := devHost(command.String("host"))
	if err != nil {
		return err
	}

	_, err = toolchainservice.Ensure(ctx, manifest, toolchainservice.EnsureOptions{
		GoOverride:        command.String("go"),
		TinyGoOverride:    command.String("tinygo"),
		WasmToolsOverride: command.String("wasm-tools"),
		AirOverride:       command.String("air"),
		NeedGo:            config.Project.Compiler == "go",
		NeedTinyGo:        config.Project.Compiler != "go",
		NeedWasmTools:     true,
		NeedAir:           true,
	})
	if err != nil {
		return fmt.Errorf("prepare SDK toolchain: %w", err)
	}

	goPath := command.String("go")
	if config.Project.Compiler == "go" && goPath == "" {
		goPath, err = toolchainservice.Go(ctx, toolchainservice.GoOptions{Version: manifest.Tools.Go})
		if err != nil {
			return fmt.Errorf("resolve Go for dev: %w", err)
		}
	}

	air, err := findAir(command.String("air"), manifest.Tools.Air)
	if err != nil {
		return err
	}

	port := command.String("port")

	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65534 {
		return errInvalidPort
	}

	appAddress := net.JoinHostPort("127.0.0.1", strconv.Itoa(portNumber+1))

	proxyAddress := net.JoinHostPort(command.String("bind"), port)
	if err := ensurePortAvailable(ctx, command.String("bind"), portNumber); err != nil {
		return err
	}

	if err := ensurePortAvailable(ctx, "127.0.0.1", portNumber+1); err != nil {
		return err
	}

	currentExecutable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve karty executable: %w", err)
	}

	buildCommand := devBuildCommand(currentExecutable, host)

	for _, option := range []struct{ name, value string }{
		{name: "--go", value: goPath},
		{name: "--tinygo", value: command.String("tinygo")},
		{name: "--wasm-tools", value: command.String("wasm-tools")},
	} {
		if option.value != "" {
			buildCommand = append(buildCommand, option.name, shellQuote(option.value))
		}
	}

	serveCommand := strings.Join([]string{
		shellQuote(currentExecutable),
		"serve dist/web --addr", appAddress,
	}, " ")

	args := []string{
		"-root", ".",
		"-build.cmd", strings.Join(buildCommand, " "),
		"-build.full_bin", serveCommand,
		"-build.include_dir", "src,assets,levels,ui",
		"-build.include_ext", "go,ui,json,toml,png,jpg,jpeg,webp,wav,mpg",
		"-build.include_file", "karty.toml",
		"-build.exclude_regex", generatedSourcePattern,
		"-build.exclude_dir", "dist",
		"-tmp_dir", airRuntimeDirectory,
		"-build.log", airBuildLog,
		"-build.stop_on_error", "false",
		"-proxy.enabled", "true",
		"-proxy.proxy_port", port,
		"-proxy.app_port", strconv.Itoa(portNumber + 1),
		"-proxy.app_start_timeout", "10000",
		"-log.time", "false",
	}

	fmt.Fprintf(os.Stderr, "Karty dev watching src, assets, levels and ui, proxying at http://%s/\n", proxyAddress)

	process := exec.CommandContext(context.WithoutCancel(ctx), air, args...)
	prepareProcess(process)

	process.Dir, err = os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve project directory: %w", err)
	}

	process.Stdin = os.Stdin
	process.Stdout = os.Stdout
	process.Stderr = os.Stderr

	if err := runProcess(ctx, process); err != nil && ctx.Err() == nil {
		return fmt.Errorf("run air: %w", err)
	}

	return nil
}

// devHost returns only local overrides. Managed artifacts are resolved by each
// build from the project SDK, including its separately cached wasm_exec.js.
func devHost(override string) (string, error) {
	host, err := findHost(override)
	if errors.Is(err, errHostNotFound) {
		return "", nil
	}

	return host, err
}

func devBuildCommand(executable, host string) []string {
	command := []string{shellQuote(executable), "build", "--target", "web"}
	if host != "" {
		command = append(command, "--host", shellQuote(host))
	}

	return command
}

func runProcess(ctx context.Context, process *exec.Cmd) error {
	if err := process.Start(); err != nil {
		return err
	}

	wait := make(chan error, 1)
	go func() {
		wait <- process.Wait()
	}()

	select {
	case err := <-wait:
		return err
	case <-ctx.Done():
		interruptProcess(process)

		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()

		select {
		case <-wait:
		case <-timer.C:
			killProcess(process)
			<-wait
		}

		return nil
	}
}

func findAir(override, version string) (string, error) {
	path, err := toolchainservice.Air(toolchainservice.AirOptions{Override: override, Version: version})
	if err != nil {
		return "", errAirNotFound
	}

	return path, nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func ensurePortAvailable(ctx context.Context, host string, port int) error {
	address := net.JoinHostPort(host, strconv.Itoa(port))
	listenerConfig := net.ListenConfig{}

	listener, err := listenerConfig.Listen(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("%s: %w", address, errPortInUse)
	}

	return listener.Close()
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

	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}

	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s: %w", absolute, errHostNotFile)
	}

	return absolute, nil
}
