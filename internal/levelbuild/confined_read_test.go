package levelbuild

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestConfinedReadEnforcesSourceBounds(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		size    int
		maximum int
		valid   bool
	}{
		{"empty", 0, 32, false},
		{"one byte", 1, 32, true},
		{"exact maximum", 32, 32, true},
		{"past maximum", 33, 32, false},
		{"zero maximum", 1, 0, false},
		{"negative maximum", 1, -1, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()

			source := bytes.Repeat([]byte("x"), test.size)
			if err := os.WriteFile(filepath.Join(root, "data.bin"), source, 0o600); err != nil {
				t.Fatal(err)
			}

			data, err := readConfinedFile(root, "data.bin", test.maximum)
			if test.valid {
				if err != nil || !bytes.Equal(data, source) {
					t.Fatalf("valid bounded source changed: %v", err)
				}
			} else if !errors.Is(err, ErrSource) || data != nil {
				t.Fatalf("invalid source exposed data: %v", err)
			}
		})
	}
}

func TestConfinedReadPreservesContainedSymlinksAndRejectsEscapes(t *testing.T) {
	t.Parallel()

	root, outside := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "data"), 0o700); err != nil {
		t.Fatal(err)
	}

	source := []byte("confined bytes")
	if err := os.WriteFile(filepath.Join(root, "data", "source.bin"), source, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(outside, "escape.bin"), []byte("outside bytes"), 0o600); err != nil {
		t.Fatal(err)
	}

	for alias, target := range map[string]string{
		"file-alias.bin":   filepath.Join(root, "data", "source.bin"),
		"directory-alias":  filepath.Join(root, "data"),
		"file-escape.bin":  filepath.Join(outside, "escape.bin"),
		"directory-escape": outside,
	} {
		if err := os.Symlink(target, filepath.Join(root, alias)); err != nil {
			t.Skip("symlinks unavailable:", err)
		}
	}

	escapingRelative, err := filepath.Rel(root, filepath.Join(outside, "escape.bin"))
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		path  string
		valid bool
	}{
		{"data/source.bin", true},
		{"file-alias.bin", true},
		{"directory-alias/source.bin", true},
		{"file-escape.bin", false},
		{"directory-escape/escape.bin", false},
		{filepath.Join(outside, "escape.bin"), false},
		{escapingRelative, false},
		{"data", false},
		{"missing.bin", false},
	} {
		t.Run(test.path, func(t *testing.T) {
			t.Parallel()

			data, err := readConfinedFile(root, test.path, 32)
			if test.valid {
				if err != nil || !bytes.Equal(data, source) {
					t.Fatalf("contained source alias changed behavior: %v", err)
				}
			} else if !errors.Is(err, ErrSource) || data != nil {
				t.Fatalf("escaping or nonregular source exposed bytes: %v", err)
			}
		})
	}
}
