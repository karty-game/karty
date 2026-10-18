//go:build unix

package process

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

const waitDelay = 3 * time.Second

func configure(_ context.Context, command *exec.Cmd) {
	// Build tools can start compiler, linker or asset-processing children. Cancel
	// the invocation together so superseded work cannot keep writing staging files.
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}

		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}

		return err
	}
	command.WaitDelay = waitDelay
}
