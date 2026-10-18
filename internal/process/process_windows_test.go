package process

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

const compilerWindowsHelper = "KARTY_COMPILER_WINDOWS_HELPER"

type windowsCompilerReadyWriter struct {
	mutex    sync.Mutex
	once     sync.Once
	ready    chan uint32
	contents bytes.Buffer
}

func (writer *windowsCompilerReadyWriter) Write(contents []byte) (int, error) {
	writer.mutex.Lock()
	defer writer.mutex.Unlock()

	_, _ = writer.contents.Write(contents)
	_, suffix, found := strings.Cut(writer.contents.String(), "compiler child started ")

	pidText, _, line := strings.Cut(suffix, "\n")
	if found && line {
		pid, err := strconv.ParseUint(strings.TrimSpace(pidText), 10, 32)
		if err == nil {
			writer.once.Do(func() { writer.ready <- uint32(pid) })
		}
	}

	return len(contents), nil
}

func TestCancellationClosesWindowsDescendantPipes(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// #nosec G204 G702 -- Reexecute this test binary to qualify real Windows descendant cleanup.
	command := CommandContext(ctx, os.Args[0], "-test.run=^TestWindowsProcessHelper$")

	command.Env = append(os.Environ(), compilerWindowsHelper+"=parent")

	writer := &windowsCompilerReadyWriter{ready: make(chan uint32, 1)}
	command.Stdout, command.Stderr = writer, writer
	finished := make(chan error, 1)

	go func() { finished <- command.Run() }()

	var childPID uint32
	select {
	case childPID = <-writer.ready:
	case err := <-finished:
		t.Fatalf("Windows compiler helper exited before its child started: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("Windows compiler helper did not start")
	}

	child, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE, false, childPID)
	if err != nil {
		t.Fatalf("open live Windows descendant: %v", err)
	}

	t.Cleanup(func() {
		_ = windows.TerminateProcess(child, 1)
		_ = windows.CloseHandle(child)
	})

	if state, waitErr := windows.WaitForSingleObject(child, 0); waitErr != nil || state != uint32(windows.WAIT_TIMEOUT) {
		t.Fatalf("Windows descendant exited before cancellation: state=%d error=%v", state, waitErr)
	}

	cancel()

	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("canceled Windows compiler succeeded")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Windows compiler descendant retained its output pipe after cancellation")
	}

	// WaitDelay can close inherited pipes even if the descendant survives. An
	// owned process handle proves that the real tool child exited as well.
	state, waitErr := windows.WaitForSingleObject(child, uint32(time.Second/time.Millisecond))
	if waitErr != nil || state != windows.WAIT_OBJECT_0 {
		t.Fatalf("Windows descendant survived cancellation: state=%d error=%v", state, waitErr)
	}
}

func TestWindowsProcessHelper(t *testing.T) {
	t.Parallel()

	switch os.Getenv(compilerWindowsHelper) {
	case "child":
		// The parent cancellation must terminate this process before the outer
		// test's bounded wait. It keeps the inherited output pipe open.
		time.Sleep(time.Minute)
	case "parent":
		// #nosec G204 G702 -- Reexecute this test binary as a descendant of the compiler fixture.
		child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestWindowsProcessHelper$")

		child.Env = append(os.Environ(), compilerWindowsHelper+"=child")

		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}

		fmt.Fprintf(os.Stdout, "compiler child started %d\n", child.Process.Pid)

		_ = child.Wait()
	default:
		t.Skip("subprocess fixture for actual Windows process-tree cancellation")
	}
}
