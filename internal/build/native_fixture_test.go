package build_test

import (
	"os"
	"testing"
)

func nativeHostFixture(t *testing.T) string {
	t.Helper()

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}

	file, err := os.CreateTemp(t.TempDir(), "karty-host-*")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := file.Write(contents); err != nil {
		file.Close()
		t.Fatal(err)
	}

	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	return file.Name()
}
