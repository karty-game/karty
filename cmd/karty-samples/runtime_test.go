package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runtimeFixture(t *testing.T) (string, string) {
	t.Helper()

	contents := "\x00asm\x01\x00\x00\x00"
	path := filepath.Join(t.TempDir(), "karty-host.wasm")
	writeFixture(t, path, contents)

	return path, fmt.Sprintf("%x", sha256.Sum256([]byte(contents)))
}

func TestSharedRuntimeIsCopiedOnceAndAssembled(t *testing.T) {
	t.Parallel()

	state, source := sampleFixture(t)
	host, checksum := runtimeFixture(t)

	wantURL := "../runtime/0.0.11/" + checksum + ".wasm"
	for range 2 {
		url, err := copyRuntimeHost(source, "0.0.11", checksum, host)
		if err != nil || url != wantURL {
			t.Fatalf("shared host: %q, %v", url, err)
		}
	}

	files, err := os.ReadDir(filepath.Join(source, "runtime", "0.0.11"))
	if err != nil || len(files) != 1 {
		t.Fatalf("duplicate host copies: %v, %v", files, err)
	}

	if err = assemble(state, source, "pr/5", false, nil); err != nil {
		t.Fatal(err)
	}

	if err = requireFile(filepath.Join(state, "pr", "5", "runtime", "0.0.11", checksum+".wasm")); err != nil {
		t.Fatal(err)
	}
}

func TestSharedRuntimeRejectsChangedOrEscapingArtifacts(t *testing.T) {
	t.Parallel()

	for _, scenario := range []string{"version", "checksum", "contents", "source link", "runtime link", "version link", "output link"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			site := t.TempDir()
			host, checksum := runtimeFixture(t)
			version := "0.0.11"

			switch scenario {
			case "version":
				version = "../outside"
			case "checksum":
				checksum = strings.Repeat("0", 64)
			case "contents":
				writeFixture(t, host, "changed")
			case "source link", "runtime link", "version link", "output link":
				runtimeSymlinkFixture(t, site, host, version, checksum, scenario)
			}

			if _, err := copyRuntimeHost(site, version, checksum, host); err == nil {
				t.Fatal("invalid shared runtime accepted")
			}
		})
	}
}

func runtimeSymlinkFixture(t *testing.T, site, host, version, checksum, scenario string) {
	t.Helper()

	link := filepath.Join(site, "runtime")

	target := t.TempDir()
	if scenario == "source link" {
		link, target = host, filepath.Join(target, "linked.wasm")
		writeFixture(t, target, "\x00asm\x01\x00\x00\x00")

		if err := os.Remove(host); err != nil {
			t.Fatal(err)
		}
	}

	if scenario == "version link" || scenario == "output link" {
		if err := os.MkdirAll(link, 0o755); err != nil {
			t.Fatal(err)
		}

		link = filepath.Join(link, version)
	}

	if scenario == "output link" {
		if err := os.Mkdir(link, 0o755); err != nil {
			t.Fatal(err)
		}

		link, target = filepath.Join(link, checksum+".wasm"), host
	}

	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
}

func TestPublisherRejectsInvalidSharedRuntimeBeforeReplacingPreview(t *testing.T) {
	t.Parallel()

	for _, scenario := range []string{"missing", "name", "checksum", "header", "extra file", "nested", "version"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()

			state, source := sampleFixture(t)
			if err := assemble(state, source, "main", false, nil); err != nil {
				t.Fatal(err)
			}

			host, checksum := runtimeFixture(t)
			if _, err := copyRuntimeHost(source, "0.0.11", checksum, host); err != nil {
				t.Fatal(err)
			}

			path := filepath.Join(source, "runtime", "0.0.11", checksum+".wasm")

			switch scenario {
			case "missing":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "name":
				if err := os.Rename(path, filepath.Join(filepath.Dir(path), "host.wasm")); err != nil {
					t.Fatal(err)
				}
			case "checksum":
				writeFixture(t, path, "\x00asm\x01\x00\x00\x00changed")
			case "header":
				writeFixture(t, path, "not wasm")
			case "extra file":
				writeFixture(t, filepath.Join(filepath.Dir(path), "notes.md"), "notes")
			case "nested":
				writeFixture(t, filepath.Join(filepath.Dir(path), "extra", "host.wasm"), "nested")
			case "version":
				if err := os.Rename(filepath.Dir(path), filepath.Join(source, "runtime", "latest")); err != nil {
					t.Fatal(err)
				}
			}

			if err := assemble(state, source, "main", false, nil); err == nil {
				t.Fatal("invalid runtime accepted")
			}

			if err := requireFile(filepath.Join(state, "main", "ui-demo", "index.html")); err != nil {
				t.Fatal("working preview replaced before runtime validation", err)
			}
		})
	}
}

func TestSharedRuntimeAloneIsNotASampleSite(t *testing.T) {
	t.Parallel()

	site := t.TempDir()
	writeFixture(t, filepath.Join(site, "index.html"), "index")

	host, checksum := runtimeFixture(t)
	if _, err := copyRuntimeHost(site, "0.0.11", checksum, host); err != nil {
		t.Fatal(err)
	}

	if err := validateSamples(os.DirFS(site)); err == nil {
		t.Fatal("runtime-only site accepted")
	}
}
