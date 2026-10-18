package process

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

const waitDelay = 3 * time.Second

func configure(ctx context.Context, command *exec.Cmd) {
	if command.Err != nil {
		return
	}

	systemDirectory, err := windows.GetSystemDirectory()
	if err != nil {
		command.Err = fmt.Errorf("locate Windows process cleanup tool: %w", err)

		return
	}

	taskkill := filepath.Join(systemDirectory, "taskkill.exe")
	if _, err := os.Stat(taskkill); err != nil {
		command.Err = fmt.Errorf("locate Windows process cleanup tool: %w", err)

		return
	}

	command.Cancel = func() error { return cancelWindowsTree(ctx, command, taskkill) }
	command.WaitDelay = waitDelay
}

func cancelWindowsTree(ctx context.Context, command *exec.Cmd, taskkill string) error {
	if command.Process == nil {
		return os.ErrProcessDone
	}

	// The build context is already canceled. Give Windows its own bounded
	// cleanup window, and wait for /T to terminate tool descendants.
	cleanupContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), waitDelay)
	defer cancel()

	// #nosec G204 G702 -- The executable is preflighted in the OS system directory; the only variable argument is our child PID.
	cleanup := exec.CommandContext(cleanupContext, taskkill, "/PID", strconv.Itoa(command.Process.Pid), "/T", "/F")
	cleanup.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cleanup.WaitDelay = waitDelay

	if output, err := cleanup.CombinedOutput(); err != nil {
		// Always stop the directly owned tool even if the OS tree operation
		// is unavailable or fails; return its diagnostic instead of hiding it.
		_ = command.Process.Kill()

		return fmt.Errorf("terminate Windows tool process tree: %w\n%s", err, output)
	}

	return nil
}
