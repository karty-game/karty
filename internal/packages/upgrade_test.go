package packages

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpgradeLatestStableAndRepeat(t *testing.T) {
	t.Parallel()

	fixture := newCatalogImportFixture(t)
	if _, err := importWithCatalog(t.Context(), fixture.project, "placeholder-kit@0.1.0", false,
		fixture.server.URL+"/master.json", fixture.server.Client()); err != nil {
		t.Fatal(err)
	}

	for attempt := range 2 {
		before, module := read(t, filepath.Join(fixture.project, "karty.toml")), read(t, filepath.Join(fixture.project, "go.mod"))

		definition, err := upgradeWithCatalog(t.Context(), fixture.project, "placeholder-kit",
			fixture.server.URL+"/master.json", fixture.server.Client())
		if err != nil {
			t.Fatal(err)
		}

		if definition.Version != "0.2.0" {
			t.Fatalf("upgrade selected %s instead of latest stable", definition.Version)
		}

		assertCatalogImportPin(t, fixture, "0.2.0")

		if attempt == 1 && (!bytes.Equal(before, read(t, filepath.Join(fixture.project, "karty.toml"))) ||
			!bytes.Equal(module, read(t, filepath.Join(fixture.project, "go.mod")))) {
			t.Fatal("repeated upgrade changed an already current package pin")
		}
	}

	if fixture.requestCount("/master.json") != 3 || fixture.requestCount("/archives/placeholder-kit-0.2.0.kpkg") != 1 {
		t.Fatal("upgrade must refresh the catalog and reuse a verified cached archive")
	}
}

func TestUpgradeUsesCatalogDespiteLocalFolder(t *testing.T) {
	fixture := newCatalogImportFixture(t)
	archive := filepath.Join(t.TempDir(), "old.kpkg")
	write(t, archive, fixture.archives["/archives/placeholder-kit-0.1.0.kpkg"])

	if _, err := Import(t.Context(), fixture.project, archive, false); err != nil {
		t.Fatal(err)
	}

	t.Chdir(fixture.project)
	write(t, filepath.Join(fixture.project, "placeholder-kit", ManifestName), []byte("invalid local manifest"))

	if _, err := upgradeWithCatalog(t.Context(), fixture.project, "placeholder-kit",
		fixture.server.URL+"/master.json", fixture.server.Client()); err != nil {
		t.Fatal(err)
	}

	sources, sdk, err := LoadSources(t.Context(), fixture.project)
	if err != nil {
		t.Fatal(err)
	}

	selected := fixture.document.Packages[0].Versions[1]
	if sources["placeholder-kit"] != (Source{Archive: selected.Archive, SHA256: selected.SHA256}) || sdk != "0.0.13" {
		t.Fatal("upgrade did not replace the archive pin while preserving the project SDK")
	}

	assertCatalogModulePin(t, fixture.project, "0.2.0")
}

func TestUpgradeRequiresInstalledArchivedName(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name, input, message string
		linked               bool
	}{
		{"not installed", "placeholder-kit", "not installed", false},
		{"linked folder", "placeholder-kit", "development folder", true},
		{"exact version", "placeholder-kit@0.1.0", "without a version or path", false},
		{"path", "./placeholder-kit", "without a version or path", false},
		{"empty", "", "without a version or path", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fixture := newCatalogImportFixture(t)
			if test.linked {
				if _, err := Import(t.Context(), fixture.project, fixture.kit, false); err != nil {
					t.Fatal(err)
				}
			}

			before, module := read(t, filepath.Join(fixture.project, "karty.toml")), read(t, filepath.Join(fixture.project, "go.mod"))

			_, err := upgradeWithCatalog(
				t.Context(),
				fixture.project,
				test.input,
				fixture.server.URL+"/master.json",
				fixture.server.Client(),
			)
			if !errors.Is(err, ErrPackage) || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("upgrade error = %v, want %s", err, test.message)
			}

			if !bytes.Equal(before, read(t, filepath.Join(fixture.project, "karty.toml"))) ||
				!bytes.Equal(module, read(t, filepath.Join(fixture.project, "go.mod"))) || fixture.totalRequests() != 0 {
				t.Fatal("invalid upgrade changed project files or contacted the catalog")
			}
		})
	}
}

func TestUpgradeFailurePreservesInstalledPackage(t *testing.T) {
	t.Parallel()

	for _, failure := range []string{"checksum", "SDK minimum", "missing catalog entry"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()

			fixture := newCatalogImportFixture(t)
			if _, err := importWithCatalog(t.Context(), fixture.project, "placeholder-kit@0.1.0", false,
				fixture.server.URL+"/master.json", fixture.server.Client()); err != nil {
				t.Fatal(err)
			}

			before, module := read(t, filepath.Join(fixture.project, "karty.toml")), read(t, filepath.Join(fixture.project, "go.mod"))
			installed := filepath.Join(fixture.project, ".karty/packages/placeholder-kit", ManifestName)
			manifest := read(t, installed)

			switch failure {
			case "checksum":
				fixture.document.Packages[0].Versions[1].SHA256 = strings.Repeat("a", 64)
			case "SDK minimum":
				write(t, filepath.Join(fixture.kit, ManifestName), []byte(
					"format_version=1\nname='placeholder-kit'\nversion='0.2.0'\nsdk='0.0.30'\n"))
				archive := filepath.Join(t.TempDir(), "new.kpkg")
				fixture.document.Packages[0].Versions[1].SHA256 = export(t, fixture.kit, archive)
				fixture.archives["/archives/placeholder-kit-0.2.0.kpkg"] = read(t, archive)
			case "missing catalog entry":
				fixture.document.Packages = []catalogImportPackage{}
			}

			if _, err := upgradeWithCatalog(t.Context(), fixture.project, "placeholder-kit",
				fixture.server.URL+"/master.json", fixture.server.Client()); err == nil {
				t.Fatal("invalid upgrade succeeded")
			}

			if !bytes.Equal(before, read(t, filepath.Join(fixture.project, "karty.toml"))) ||
				!bytes.Equal(module, read(t, filepath.Join(fixture.project, "go.mod"))) || !bytes.Equal(manifest, read(t, installed)) {
				t.Fatal("failed upgrade changed authored pins or installed package")
			}
		})
	}
}
