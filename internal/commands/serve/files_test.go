package serve

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestPreviewServesExplicitIndexWithoutRedirect(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}

	for name, body := range map[string]string{"index.html": "root shell", "nested/index.html": "nested shell"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	handler := previewFiles(directory)

	for _, target := range []string{"/", "/index.html", "/nested/", "/nested/index.html"} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			t.Run(method+target, func(t *testing.T) {
				t.Parallel()

				response := httptest.NewRecorder()
				handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), method, target, nil))

				if response.Code != http.StatusOK || response.Header().Get("Location") != "" {
					t.Fatal("startup asset redirected or failed", response.Code, response.Header())
				}

				body := "root shell"
				if target == "/nested/" || target == "/nested/index.html" {
					body = "nested shell"
				}

				if response.Header().Get("Content-Length") != strconv.Itoa(len(body)) {
					t.Fatal("startup asset length changed", response.Header())
				}

				if method == http.MethodHead {
					body = ""
				}

				if response.Body.String() != body {
					t.Fatal("incorrect startup asset body", response.Body.String())
				}
			})
		}
	}
}

func TestPreviewMissingIndexDoesNotRedirectOrListDirectory(t *testing.T) {
	t.Parallel()

	handler := previewFiles(t.TempDir())

	for _, target := range []string{"/index.html", "/missing/index.html"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))

		if response.Code != http.StatusNotFound || response.Header().Get("Location") != "" {
			t.Fatal("missing index redirected or listed directory", target, response.Code, response.Header())
		}
	}
}
