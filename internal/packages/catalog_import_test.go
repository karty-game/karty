package packages

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"golang.org/x/mod/modfile"
)

type catalogImportVersion struct {
	Version    string `json:"version"`
	Archive    string `json:"archive"`
	SHA256     string `json:"sha256"`
	ReleaseURL string `json:"release_url"` //nolint:tagliatelle // Match the published catalog schema.
}

type catalogImportPackage struct {
	Name        string                 `json:"name"`
	Latest      *string                `json:"latest"`
	Versions    []catalogImportVersion `json:"versions"`
	Description string                 `json:"description"`
	Repository  string                 `json:"repository"`
}

type catalogImportDocument struct {
	FormatVersion int                    `json:"format_version"` //nolint:tagliatelle // Match the published catalog schema.
	Packages      []catalogImportPackage `json:"packages"`
}

type catalogImportFixture struct {
	server   *httptest.Server
	project  string
	kit      string
	document catalogImportDocument
	body     []byte
	archives map[string][]byte
	mu       sync.Mutex
	requests map[string]int
}

func newCatalogImportFixture(t *testing.T) *catalogImportFixture {
	t.Helper()

	kit, project := fixture(t)
	// Catalog selection uses a code package. Resource/archive validation has its
	// own fixtures; repeating world-schema resolution here obscures these checks.
	write(
		t,
		filepath.Join(kit, ManifestName),
		[]byte("format_version = 1\nname = \"placeholder-kit\"\nversion = \"0.1.0\"\nsdk = \"0.0.13\"\n"),
	)

	fixture := &catalogImportFixture{
		project: project, kit: kit,
		archives: make(map[string][]byte), requests: make(map[string]int),
	}
	fixture.server = httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		fixture.mu.Lock()
		fixture.requests[request.URL.Path]++
		fixture.mu.Unlock()

		if request.URL.Path == "/master.json" {
			writer.Header().Set("Content-Type", "application/json")

			if fixture.body != nil {
				_, _ = writer.Write(fixture.body)
			} else {
				_ = json.NewEncoder(writer).Encode(fixture.document)
			}

			return
		}

		if contents, ok := fixture.archives[request.URL.Path]; ok {
			_, _ = writer.Write(contents)

			return
		}

		http.NotFound(writer, request)
	}))
	t.Cleanup(fixture.server.Close)

	latest := "0.2.0"
	fixture.document = catalogImportDocument{
		FormatVersion: 1,
		Packages: []catalogImportPackage{{
			Name: "placeholder-kit", Latest: &latest,
			Description: "Metadata from an external package repository.", Repository: "community/placeholder-kit",
		}},
	}

	original := read(t, filepath.Join(kit, ManifestName))
	for _, version := range []string{"0.1.0", "0.2.0", "0.3.0-rc.1"} {
		write(
			t,
			filepath.Join(kit, ManifestName),
			bytes.Replace(original, []byte(`version = "0.1.0"`), []byte(`version = "`+version+`"`), 1),
		)
		archive := filepath.Join(t.TempDir(), "placeholder-kit-"+version+".kpkg")
		digest := export(t, kit, archive)
		path := "/archives/placeholder-kit-" + version + ".kpkg"
		fixture.archives[path] = read(t, archive)
		fixture.document.Packages[0].Versions = append(fixture.document.Packages[0].Versions, catalogImportVersion{
			Version: version, Archive: fixture.server.URL + path, SHA256: digest,
			ReleaseURL: "https://github.com/community/placeholder-kit/releases/tag/placeholder-kit-v" + version,
		})
	}

	write(t, filepath.Join(kit, ManifestName), original)

	return fixture
}

func (fixture *catalogImportFixture) requestCount(path string) int {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()

	return fixture.requests[path]
}

func (fixture *catalogImportFixture) totalRequests() int {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()

	count := 0
	for _, n := range fixture.requests {
		count += n
	}

	return count
}

func assertCatalogModulePin(t *testing.T, project, version string) {
	t.Helper()

	mod, err := modfile.Parse("go.mod", read(t, filepath.Join(project, "go.mod")), nil)
	if err != nil {
		t.Fatal(err)
	}

	required, replaced := false, false

	for _, entry := range mod.Require {
		if entry.Mod.Path == "github.com/karty-game/stdlib-spike" && entry.Mod.Version == "v"+version {
			required = true
		}
	}

	for _, entry := range mod.Replace {
		if entry.Old.Path == "github.com/karty-game/stdlib-spike" && entry.Old.Version == "" &&
			entry.New.Version == "" && filepath.Clean(entry.New.Path) == filepath.Join(".karty", "packages", "placeholder-kit") {
			replaced = true
		}
	}

	if !required || !replaced {
		t.Fatalf("import did not pin Go module and package mapping for %s: %s", version, read(t, filepath.Join(project, "go.mod")))
	}
}

func assertCatalogImportPin(t *testing.T, fixture *catalogImportFixture, version string) {
	t.Helper()

	sources, _, err := LoadSources(t.Context(), fixture.project)
	if err != nil {
		t.Fatal(err)
	}

	source := sources["placeholder-kit"]

	path := "/archives/placeholder-kit-" + version + ".kpkg"
	if source.Archive != fixture.server.URL+path || source.SHA256 != Digest(fixture.archives[path]) || source.Folder != "" {
		t.Fatalf("import did not persist immutable HTTPS archive and digest: %+v", source)
	}

	manifest := read(t, filepath.Join(fixture.project, "karty.toml"))
	if bytes.Contains(manifest, []byte("master.json")) || !bytes.Contains(manifest, []byte("# Keep project comments")) {
		t.Fatal("project depends on the catalog URL or lost unrelated comments")
	}

	if _, err := os.Stat(filepath.Join(fixture.project, "packages")); !os.IsNotExist(err) {
		t.Fatalf("remote package was copied into authored project/packages: %v", err)
	}

	assertCatalogModulePin(t, fixture.project, version)

	set, err := restoreWithClient(t.Context(), fixture.project, fixture.server.Client())
	if err != nil {
		t.Fatal(err)
	}
	defer set.Close()

	if len(set.Items) != 1 || set.Items["placeholder-kit"].Manifest.Version != version {
		t.Fatalf("resolved package differs from selected catalog version: %+v", set.Items)
	}

	if err := CheckModules(t.Context(), fixture.project, set); err != nil {
		t.Fatal(err)
	}
}

func TestCatalogImportLatestAndExactVersion(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct{ input, version string }{
		{"placeholder-kit", "0.2.0"},
		{"placeholder-kit@0.1.0", "0.1.0"},
		{"placeholder-kit@0.3.0-rc.1", "0.3.0-rc.1"},
	} {
		t.Run(testCase.input, func(t *testing.T) {
			t.Parallel()

			fixture := newCatalogImportFixture(t)

			definition, err := importWithCatalog(
				t.Context(),
				fixture.project,
				testCase.input,
				false,
				fixture.server.URL+"/master.json",
				fixture.server.Client(),
			)
			if err != nil {
				t.Fatal(err)
			}

			if definition.Name != "placeholder-kit" || definition.Version != testCase.version ||
				definition.GoModule != "github.com/karty-game/stdlib-spike" {
				t.Fatalf("wrong selected manifest: %+v", definition)
			}

			assertCatalogImportPin(t, fixture, testCase.version)

			if fixture.requestCount("/master.json") != 1 ||
				fixture.requestCount("/archives/placeholder-kit-"+testCase.version+".kpkg") != 1 ||
				fixture.totalRequests() != 2 {
				t.Fatalf("expected one catalog and selected archive request, got %+v", fixture.requests)
			}
		})
	}
}

func TestCatalogImportRequiresReplaceAndUpdatesPin(t *testing.T) {
	t.Parallel()

	fixture := newCatalogImportFixture(t)

	if _, err := importWithCatalog(
		t.Context(),
		fixture.project,
		"placeholder-kit@0.1.0",
		false,
		fixture.server.URL+"/master.json",
		fixture.server.Client(),
	); err != nil {
		t.Fatal(err)
	}

	beforeManifest := read(t, filepath.Join(fixture.project, "karty.toml"))
	beforeModule := read(t, filepath.Join(fixture.project, "go.mod"))
	link := filepath.Join(fixture.project, ".karty/packages/placeholder-kit")

	beforeTarget, err := filepath.EvalSymlinks(link)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := importWithCatalog(
		t.Context(),
		fixture.project,
		"placeholder-kit",
		false,
		fixture.server.URL+"/master.json",
		fixture.server.Client(),
	); err == nil {
		t.Fatal("updated existing package without --replace")
	}

	if !bytes.Equal(beforeManifest, read(t, filepath.Join(fixture.project, "karty.toml"))) ||
		!bytes.Equal(beforeModule, read(t, filepath.Join(fixture.project, "go.mod"))) {
		t.Fatal("rejected replacement modified project declarations")
	}

	if target, err := filepath.EvalSymlinks(link); err != nil || target != beforeTarget {
		t.Fatalf("rejected replacement changed installed link: %s: %v", target, err)
	}

	if _, err := importWithCatalog(
		t.Context(),
		fixture.project,
		"placeholder-kit",
		true,
		fixture.server.URL+"/master.json",
		fixture.server.Client(),
	); err != nil {
		t.Fatal(err)
	}

	assertCatalogImportPin(t, fixture, "0.2.0")

	if bytes.Equal(beforeManifest, read(t, filepath.Join(fixture.project, "karty.toml"))) {
		t.Fatal("--replace left the old archive pin unchanged")
	}
}

func TestCatalogImportRestoresOfflineWithoutCatalog(t *testing.T) {
	t.Parallel()

	fixture := newCatalogImportFixture(t)

	if _, err := importWithCatalog(
		t.Context(),
		fixture.project,
		"placeholder-kit",
		false,
		fixture.server.URL+"/master.json",
		fixture.server.Client(),
	); err != nil {
		t.Fatal(err)
	}

	beforeManifest := read(t, filepath.Join(fixture.project, "karty.toml"))
	beforeModule := read(t, filepath.Join(fixture.project, "go.mod"))
	requests := fixture.totalRequests()
	fixture.server.Close()

	if err := os.RemoveAll(filepath.Join(fixture.project, ".karty/packages/.store")); err != nil {
		t.Fatal(err)
	}

	set, err := Restore(t.Context(), fixture.project)
	if err != nil {
		t.Fatalf("offline restore required catalog or redownload: %v", err)
	}
	defer set.Close()

	if set.Items["placeholder-kit"].Manifest.Version != "0.2.0" || fixture.totalRequests() != requests {
		t.Fatal("offline restore changed selected version or made network requests")
	}

	if err := CheckModules(t.Context(), fixture.project, set); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(beforeManifest, read(t, filepath.Join(fixture.project, "karty.toml"))) ||
		!bytes.Equal(beforeModule, read(t, filepath.Join(fixture.project, "go.mod"))) {
		t.Fatal("offline restore modified authored declarations")
	}
}

func TestCatalogImportFailuresPreserveProjectAndLinks(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		input  string
		change func(*catalogImportFixture)
	}{
		"malformed JSON": {change: func(fixture *catalogImportFixture) { fixture.body = []byte(`{"format_version":`) }},
		"trailing JSON": {
			change: func(fixture *catalogImportFixture) { fixture.body = []byte(`{"format_version":1,"packages":[]} {}`) },
		},
		"unsupported format": {change: func(fixture *catalogImportFixture) { fixture.document.FormatVersion = 2 }},
		"missing packages":   {change: func(fixture *catalogImportFixture) { fixture.document.Packages = nil }},
		"unpublished": {change: func(fixture *catalogImportFixture) {
			fixture.document.Packages[0].Latest = nil
			fixture.document.Packages[0].Versions = []catalogImportVersion{}
		}},
		"unknown package": {input: "unknown-kit"},
		"unknown version": {input: "placeholder-kit@9.9.9"},
		"duplicate package": {change: func(fixture *catalogImportFixture) {
			fixture.document.Packages = append(fixture.document.Packages, fixture.document.Packages[0])
		}},
		"duplicate version": {change: func(fixture *catalogImportFixture) {
			fixture.document.Packages[0].Versions = append(fixture.document.Packages[0].Versions, fixture.document.Packages[0].Versions[0])
		}},
		"unknown latest": {change: func(fixture *catalogImportFixture) { value := "9.9.9"; fixture.document.Packages[0].Latest = &value }},
		"older latest":   {change: func(fixture *catalogImportFixture) { value := "0.1.0"; fixture.document.Packages[0].Latest = &value }},
		"prerelease latest": {change: func(fixture *catalogImportFixture) {
			value := "0.3.0-rc.1"
			fixture.document.Packages[0].Latest = &value
		}},
		"noncanonical version": {change: func(fixture *catalogImportFixture) { fixture.document.Packages[0].Versions[0].Version = "0.1" }},
		"invalid digest": {
			change: func(fixture *catalogImportFixture) { fixture.document.Packages[0].Versions[1].SHA256 = "not-a-digest" },
		},
		"unsafe archive URL": {change: func(fixture *catalogImportFixture) {
			fixture.document.Packages[0].Versions[1].Archive = "http://example.com/archive.kpkg"
		}},
		"archive checksum mismatch": {
			change: func(fixture *catalogImportFixture) {
				fixture.document.Packages[0].Versions[1].SHA256 = strings.Repeat("a", 64)
			},
		},
		"archive version mismatch": {change: func(fixture *catalogImportFixture) {
			old := fixture.document.Packages[0].Versions[0]
			fixture.document.Packages[0].Versions[1].Archive = old.Archive
			fixture.document.Packages[0].Versions[1].SHA256 = old.SHA256
		}},
		"archive name mismatch": {
			input:  "other-kit",
			change: func(fixture *catalogImportFixture) { fixture.document.Packages[0].Name = "other-kit" },
		},
		"archive invalid zip": {change: func(fixture *catalogImportFixture) {
			path := "/archives/placeholder-kit-0.2.0.kpkg"
			fixture.archives[path] = []byte("not a zip")
			fixture.document.Packages[0].Versions[1].SHA256 = Digest(fixture.archives[path])
		}},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fixture := newCatalogImportFixture(t)
			if _, err := Import(t.Context(), fixture.project, fixture.kit, false); err != nil {
				t.Fatal(err)
			}

			beforeManifest := read(t, filepath.Join(fixture.project, "karty.toml"))
			beforeModule := read(t, filepath.Join(fixture.project, "go.mod"))
			link := filepath.Join(fixture.project, ".karty/packages/placeholder-kit")

			beforeTarget, err := filepath.EvalSymlinks(link)
			if err != nil {
				t.Fatal(err)
			}

			if testCase.change != nil {
				testCase.change(fixture)
			}

			input := testCase.input
			if input == "" {
				input = "placeholder-kit"
			}

			if _, err := importWithCatalog(
				t.Context(),
				fixture.project,
				input,
				true,
				fixture.server.URL+"/master.json",
				fixture.server.Client(),
			); err == nil {
				t.Fatal("invalid catalog import succeeded")
			}

			if !bytes.Equal(beforeManifest, read(t, filepath.Join(fixture.project, "karty.toml"))) ||
				!bytes.Equal(beforeModule, read(t, filepath.Join(fixture.project, "go.mod"))) {
				t.Fatal("rejected import modified authored declarations")
			}

			if target, err := filepath.EvalSymlinks(link); err != nil || target != beforeTarget {
				t.Fatalf("rejected import redirected existing package link: %s: %v", target, err)
			}

			set, err := Restore(t.Context(), fixture.project)
			if err != nil {
				t.Fatalf("existing package no longer restores: %v", err)
			}
			defer set.Close()

			if set.Items["placeholder-kit"].Manifest.Version != "0.1.0" {
				t.Fatal("rejected import changed installed manifest")
			}
		})
	}
}

func TestCatalogImportLocalInputsBypassDiscovery(t *testing.T) {
	for _, mode := range []string{"bare directory", "explicit directory", "explicit archive", "missing explicit directory", "missing explicit archive"} {
		t.Run(mode, func(t *testing.T) {
			fixture := newCatalogImportFixture(t)
			parent := filepath.Dir(fixture.kit)

			local := filepath.Join(parent, "placeholder-kit")
			if err := os.Rename(fixture.kit, local); err != nil {
				t.Fatal(err)
			}

			t.Chdir(parent)

			input := "placeholder-kit"
			wantErr := false

			switch mode {
			case "explicit directory":
				input = "./placeholder-kit"
			case "explicit archive":
				input = filepath.Join(parent, "placeholder-kit.kpkg")
				export(t, local, input)
			case "missing explicit directory":
				input, wantErr = "./missing-kit", true
			case "missing explicit archive":
				input, wantErr = "./missing-kit.kpkg", true
			}

			definition, err := importWithCatalog(
				t.Context(),
				fixture.project,
				input,
				false,
				fixture.server.URL+"/master.json",
				fixture.server.Client(),
			)
			if (err != nil) != wantErr {
				t.Fatalf("local import result: %+v, %v; want error=%t", definition, err, wantErr)
			}

			if !wantErr && (definition.Name != "placeholder-kit" || definition.Version != "0.1.0") {
				t.Fatalf("local source did not take precedence over catalog: %+v", definition)
			}

			if fixture.totalRequests() != 0 {
				t.Fatalf("local path triggered catalog network requests: %d", fixture.totalRequests())
			}

			if !wantErr {
				sources, _, err := LoadSources(t.Context(), fixture.project)
				if err != nil {
					t.Fatal(err)
				}

				if strings.HasPrefix(sources["placeholder-kit"].Archive, "https://") {
					t.Fatal("local input was recorded as a remote catalog source")
				}
			}
		})
	}
}

func TestCatalogImportExtraMetadataIsAllowed(t *testing.T) {
	t.Parallel()

	fixture := newCatalogImportFixture(t)

	encoded, err := json.Marshal(fixture.document)
	if err != nil {
		t.Fatal(err)
	}

	var document map[string]any
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatal(err)
	}

	document["repository"] = "karty-game/karty-packages"

	entries, valid := document["packages"].([]any)
	if !valid || len(entries) != 1 {
		t.Fatal("fixture catalog has no package array")
	}

	entry, valid := entries[0].(map[string]any)
	if !valid {
		t.Fatal("fixture catalog package is not an object")
	}

	entry["tags"] = []string{"input", "3d"}
	entry["readme"] = "https://github.com/community/placeholder-kit/blob/main/README.md"
	entry["source"] = "https://github.com/community/placeholder-kit/tree/main"
	entry["preview"] = "https://example.com/preview.png"

	fixture.body, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := importWithCatalog(
		t.Context(),
		fixture.project,
		"placeholder-kit",
		false,
		fixture.server.URL+"/master.json",
		fixture.server.Client(),
	); err != nil {
		t.Fatalf("catalog metadata rejected: %v", err)
	}

	assertCatalogImportPin(t, fixture, "0.2.0")
}
