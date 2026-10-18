// Package process runs context-owned build tools and cancels their descendants.
package process

import (
	"context"
	"os/exec"
)

// CommandContext creates a command whose cancellation stops the entire tool
// invocation on supported developer platforms. The caller must Run it, or pair
// Start with Wait, before reusing any staging directory the command could write.
// WaitDelay also bounds inherited output pipes during cancellation cleanup.
func CommandContext(ctx context.Context, executable string, arguments ...string) *exec.Cmd {
	// #nosec G204 G702 -- Callers supply their validated tool executable and argv; this helper never invokes a shell.
	command := exec.CommandContext(ctx, executable, arguments...)
	configure(ctx, command)

	return command
}
