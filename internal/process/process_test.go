package process

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestCanceledContextDoesNotStartTool(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	command := CommandContext(ctx, os.Args[0], "-test.run=^$")
	if err := command.Run(); !errors.Is(err, context.Canceled) {
		t.Fatalf("already-canceled command returned %v", err)
	}

	if command.Process != nil {
		t.Fatal("a tool was started after its context was canceled")
	}
}
