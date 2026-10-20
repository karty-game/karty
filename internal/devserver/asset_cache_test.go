package devserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImmutableArtifactCachedHashesPreserveHTTPBehavior(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fixtureBuild(t, root, []byte("level"), []byte("code"), nil)
	snapshot := captureFixture(t, root, 1)
	server := fixtureServer(t)
	server.Publish(snapshot)

	for name, data := range snapshot.files {
		if snapshot.checksums[name] != digest(data) {
			t.Fatalf("capture did not hash artifact %s", name)
		}
	}

	uiRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/"+snapshot.Generation.BaseURL+snapshot.Generation.UI.Artifact, nil)
	uiRequest.Header.Set("If-None-Match", `""`)

	uiResponse := httptest.NewRecorder()
	server.ServeHTTP(uiResponse, uiRequest)

	if uiResponse.Code != http.StatusOK || uiResponse.Header().Get("ETag") != `"`+snapshot.Generation.UI.ContentSHA256+`"` {
		t.Fatal("synthesized UI resource has an invalid cached ETag")
	}

	name := "karty-host.wasm"
	// Served bytes and their hash must remain independent of later disk writes.
	if err := os.WriteFile(filepath.Join(root, name), []byte("changed host"), 0o600); err != nil {
		t.Fatal(err)
	}

	etag := `"` + digest([]byte("host")) + `"`
	for _, test := range []struct {
		name, method, header, value, body string
		status                            int
	}{
		{name: "get", method: http.MethodGet, status: http.StatusOK, body: "host"},
		{name: "head", method: http.MethodHead, status: http.StatusOK},
		{name: "conditional", method: http.MethodGet, header: "If-None-Match", value: etag, status: http.StatusNotModified},
		{name: "range", method: http.MethodGet, header: "Range", value: "bytes=1-2", status: http.StatusPartialContent, body: "os"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequestWithContext(t.Context(), test.method, "/"+snapshot.Generation.BaseURL+name, nil)
			if test.header != "" {
				request.Header.Set(test.header, test.value)
			}

			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)

			if response.Code != test.status || response.Body.String() != test.body || response.Header().Get("ETag") != etag {
				t.Fatalf("HTTP response = %d, %q, %q", response.Code, response.Body.String(), response.Header().Get("ETag"))
			}
		})
	}
}

func BenchmarkImmutableArtifactHEAD(bench *testing.B) {
	for _, size := range []int{1024, 64 << 20} {
		name := "small"
		if size > 1024 {
			name = "64MiB"
		}

		bench.Run(name, func(bench *testing.B) {
			data := make([]byte, size)
			snapshot := &Snapshot{
				Generation: Generation{ID: strings.Repeat("a", digestHexCharacters)},
				files:      map[string][]byte{"karty-host.wasm": data},
				checksums:  map[string]string{"karty-host.wasm": digest(data)},
			}

			server, err := NewServer()
			if err != nil {
				bench.Fatal(err)
			}
			defer server.Close()

			server.Publish(snapshot)
			request := httptest.NewRequestWithContext(bench.Context(), http.MethodHead,
				"/__karty_dev/generations/"+snapshot.Generation.ID+"/karty-host.wasm", nil)

			bench.ReportAllocs()

			for bench.Loop() {
				response := httptest.NewRecorder()
				server.ServeHTTP(response, request)

				if response.Code != http.StatusOK {
					bench.Fatal(response.Code)
				}
			}
		})
	}
}
