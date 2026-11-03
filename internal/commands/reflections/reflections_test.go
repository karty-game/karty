package reflections

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/commands/screenshot"
	"github.com/karty-game/karty/internal/levelbuild"
)

func TestCapturePassesBoundedProtocolArgumentsWithoutShellExpansion(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixture executable")
	}

	path := filepath.Join(t.TempDir(), "host with spaces")
	// #nosec G306 -- The isolated fixture must be executable to exercise argv transport.
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n"), 0700); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := capture(
		t.Context(),
		path,
		"level with $literal.kld",
		"output with spaces.krc",
		[]string{"world/chunks@1", "world/fixed-lighting@1"},
		&output,
	); err != nil {
		t.Fatal(err)
	}

	expected := "--backend\nhardware\n--reflection-protocol\n1\n--reflection-level\nlevel with $literal.kld\n--reflection-output\noutput with spaces.krc\n--reflection-features\nworld/chunks@1,world/fixed-lighting@1\n"
	if output.String() != expected {
		t.Fatal("capture arguments changed", output.String())
	}
}

func TestStageCaptureUsesContentAddressedLoaderDirectory(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	artifact := levelbuild.Artifact{
		ContentSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("root module"))),
		Bytes:         []byte("root module"),
		Chunks: []levelbuild.ChunkArtifact{
			{ContentSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("child module"))), Bytes: []byte("child module")},
		},
	}

	path, err := stageCaptureInputs(directory, artifact)
	if err != nil {
		t.Fatal(err)
	}

	if path != filepath.Join(directory, "content", artifact.ContentSHA256+".kld") {
		t.Fatal("root bypasses loader confinement", path)
	}

	data, err := os.ReadFile(filepath.Join(filepath.Dir(path), artifact.Chunks[0].ContentSHA256+".kld"))
	if err != nil || string(data) != "child module" {
		t.Fatal("verified child is missing from root content directory", err)
	}
}

func TestReflectionCommandRejectsMissingLevelAndInvalidDeadline(t *testing.T) {
	t.Parallel()

	for _, arguments := range [][]string{{"reflections"}, {"reflections", "room", "--timeout", "0s"}} {
		command := Command()

		err := command.Run(t.Context(), arguments)
		if err == nil || !strings.Contains(err.Error(), "usage:") {
			t.Fatal("invalid capture request accepted", err)
		}
	}
}

func TestReflectionCaptureForwardsAdmittedBackend(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("POSIX argv recorder")
	}

	host := filepath.Join(t.TempDir(), "fake host")
	// #nosec G306 -- The private fixture records argv without executing a renderer.
	if err := os.WriteFile(host, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n"), 0700); err != nil {
		t.Fatal(err)
	}

	for _, mode := range []string{"hardware", "software"} {
		options, err := screenshot.CaptureBackend(mode, true)
		if err != nil {
			t.Fatal(err)
		}

		var output bytes.Buffer

		if err = captureWithOptions(t.Context(), host, "room.kld", "probe.krc", nil, &output, options); err != nil {
			t.Fatal(err)
		}

		if !strings.HasPrefix(output.String(), "--backend\n"+mode+"\n--reflection-protocol\n1\n") {
			t.Fatal("reflection backend was not forwarded", output.String())
		}
	}
}

func TestReflectionBackendRejectsInvalidFlagBeforeBuild(t *testing.T) {
	t.Parallel()

	err := Command().Run(t.Context(), []string{"reflections", "room", "--backend", "invalid"})
	if err == nil || !strings.Contains(err.Error(), "backend must be hardware or software") {
		t.Fatal("invalid backend reached project/build preparation", err)
	}
}
