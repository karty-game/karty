package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifierRejectsMissingArgumentsAndOversizedInput(t *testing.T) {
	t.Parallel()

	if err := run(nil); err == nil {
		t.Fatal("accepted missing version and files")
	}

	path := filepath.Join(t.TempDir(), "signature")
	if err := os.WriteFile(path, make([]byte, signatureBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := readBounded(path, signatureBytes); err == nil {
		t.Fatal("accepted oversized detached signature")
	}
}
