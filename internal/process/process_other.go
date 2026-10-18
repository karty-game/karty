//go:build !unix && !windows

package process

import (
	"context"
	"os/exec"
	"time"
)

const waitDelay = 3 * time.Second

func configure(_ context.Context, command *exec.Cmd) {
	command.WaitDelay = waitDelay
}
