package packages

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
)

func TestPackageSDKMinimumImportAndRestore(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name, minimum, version string
		allowed                bool
	}{
		{"same", "0.0.17", "0.0.17", true},
		{"next", "0.0.17", "0.0.18", true},
		{"later", "0.0.17", "0.0.30", true},
		{"numeric", "0.0.99", "0.0.100", true},
		{"older", "0.0.17", "0.0.16", false},
		{"prerelease before stable", "0.0.17", "0.0.17-rc.1", false},
		{"stable after prerelease", "0.0.17-rc.1", "0.0.17", true},
		{"numeric prerelease", "0.0.17-rc.2", "0.0.17-rc.10", true},
		{"older prerelease", "0.0.17-rc.2", "0.0.17-rc.1", false},
		{"invalid project version", "0.0.17", "invalid", false},
		{"short project version", "0.0.17", "0.17", false},
		{"prefixed project version", "0.0.17", "v0.0.17", false},
		{"missing project version", "0.0.17", "", false},
		{"no minimum", "", "0.0.13", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			kit, project := fixture(t)
			write(t, filepath.Join(kit, ManifestName), []byte(fmt.Sprintf(
				"format_version=1\nname='placeholder-kit'\nversion='0.1.0'\nsdk=%q\n", test.minimum)))
			projectManifest := []byte(fmt.Sprintf("[sdk]\nversion=%q\n", test.version))
			write(t, filepath.Join(project, "karty.toml"), projectManifest)
			module := read(t, filepath.Join(project, "go.mod"))

			_, err := Import(t.Context(), project, kit, false)
			if test.allowed {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				if !errors.Is(err, ErrPackage) {
					t.Fatalf("import error = %v, want incompatible package", err)
				}

				if !bytes.Equal(projectManifest, read(t, filepath.Join(project, "karty.toml"))) ||
					!bytes.Equal(module, read(t, filepath.Join(project, "go.mod"))) {
					t.Fatal("incompatible import changed project files")
				}
			}

			// Exercise restore independently, including declarations authored by hand.
			write(t, filepath.Join(project, "karty.toml"), append(projectManifest,
				[]byte(fmt.Sprintf("[packages.placeholder-kit]\nfolder=%q\n", filepath.ToSlash(kit)))...))

			set, err := Restore(t.Context(), project)
			if set != nil {
				set.Close()
			}

			if test.allowed && err != nil {
				t.Fatal(err)
			}

			if !test.allowed && !errors.Is(err, ErrPackage) {
				t.Fatalf("restore error = %v, want incompatible package", err)
			}
		})
	}
}
