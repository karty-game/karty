package toolchain

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestTinyGoArchiveRelativePath(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		want  string
		valid bool
	}{
		{name: "tinygo", valid: true},
		{name: "tinygo/", valid: true},
		{name: "tinygo/bin/tinygo.exe", want: filepath.Join("bin", "tinygo.exe"), valid: true},
		{name: "./tinygo/bin/tinygo", want: filepath.Join("bin", "tinygo"), valid: true},
		{name: "tinygo/lib/../bin/tinygo", want: filepath.Join("bin", "tinygo"), valid: true},
		{name: "other/bin/tinygo"},
		{name: "tinygo-other/bin/tinygo"},
		{name: "/tinygo/bin/tinygo"},
		{name: "tinygo/../../escape"},
		{name: "tinygo/../escape"},
		{name: `tinygo\bin\tinygo.exe`},
		{name: `tinygo/..\escape`},
		{name: "tinygo/C:/escape"},
		{name: "tinygo/C:escape"},
		{name: "tinygo/bin/tinygo.exe:stream"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, valid := archiveRelativePath(test.name, "tinygo")
			if valid != test.valid || (valid && got != test.want) {
				t.Fatalf("archiveRelativePath(%q) = %q, %v; want %q, %v", test.name, got, valid, test.want, test.valid)
			}
		})
	}
}

func TestTinyGoArchiveExtractionPortablePaths(t *testing.T) {
	t.Parallel()

	for _, format := range []string{"tar.gz", "zip"} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()
			archive := tinyGoPortableArchive(t, format, "tinygo/bin/tinygo.exe")
			checksum := fmt.Sprintf("%x", sha256.Sum256(archive))

			destination := t.TempDir()
			if err := extractTinyGo(t.Context(), destination, bytes.NewReader(archive), checksum); err != nil {
				t.Fatal(err)
			}

			contents, err := os.ReadFile(filepath.Join(destination, "tinygo", "bin", "tinygo.exe"))
			if err != nil || string(contents) != "portable executable fixture" {
				t.Fatalf("extracted executable = %q, %v", contents, err)
			}
		})
	}
}

func tinyGoPortableArchive(t *testing.T, format, name string) []byte {
	t.Helper()

	var archive bytes.Buffer

	contents := []byte("portable executable fixture")
	if format == "zip" {
		writeTinyGoPortableZIP(t, &archive, name, contents)
	} else {
		writeTinyGoPortableTar(t, &archive, name, contents)
	}

	return archive.Bytes()
}

func writeTinyGoPortableZIP(t *testing.T, output io.Writer, name string, contents []byte) {
	t.Helper()

	writer := zip.NewWriter(output)
	for _, directory := range []string{"tinygo/", "tinygo/bin/"} {
		if _, err := writer.Create(directory); err != nil {
			t.Fatal(err)
		}
	}

	entry, err := writer.Create(name)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := entry.Write(contents); err != nil {
		t.Fatal(err)
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeTinyGoPortableTar(t *testing.T, output io.Writer, name string, contents []byte) {
	t.Helper()

	gzipWriter := gzip.NewWriter(output)

	writer := tar.NewWriter(gzipWriter)
	for _, directory := range []string{"tinygo/", "tinygo/bin/"} {
		if err := writer.WriteHeader(&tar.Header{Name: directory, Typeflag: tar.TypeDir, Mode: 0o755}); err != nil {
			t.Fatal(err)
		}
	}

	if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(contents))}); err != nil {
		t.Fatal(err)
	}

	if _, err := writer.Write(contents); err != nil {
		t.Fatal(err)
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
}
