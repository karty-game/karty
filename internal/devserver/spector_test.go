package devserver

import (
	"net/http"
	"strings"
	"testing"
)

func TestSpectorOptInPrecedesHostWithoutChangingGeneration(t *testing.T) {
	t.Parallel()

	server := fixtureServer(t)
	root := t.TempDir()
	fixtureBuild(t, root, []byte("level"), []byte("code"), nil)
	snapshot := captureFixture(t, root, 1)
	server.Publish(snapshot)

	for _, prefix := range []string{"/", "/" + snapshot.Generation.BaseURL + "index.html"} {
		for _, query := range []string{"", "?spector=0", "?spector=true", "?spector=1&debug=1"} {
			response := request(server, prefix+query)
			body := response.Body.String()
			bundle := strings.Index(body, "spectorjs@0.9.33/dist/spector.bundle.js")
			spy := strings.Index(body, "spector.spyCanvases()")
			launcher := strings.Index(body, `<script src="karty.js?v=`)
			enabled := query == "?spector=1&debug=1"

			if response.Code != http.StatusOK || (bundle >= 0) != enabled || (spy >= 0) != enabled {
				t.Fatalf("%s%s: incorrect Spector opt-in", prefix, query)
			}

			if enabled && (bundle >= spy || spy >= launcher) {
				t.Fatal("Spector must observe context creation before the host launcher executes")
			}
		}
	}

	if len(server.snapshots) != 1 || strings.Contains(string(snapshot.files["index.html"]), "SPECTOR") {
		t.Fatal("request instrumentation changed the stored build snapshot")
	}

	launcher := request(server, "/"+snapshot.Generation.BaseURL+"karty.js?spector=1").Body.String()
	if launcher != string(snapshot.files["karty.js"]) {
		t.Fatal("Spector opt-in changed generation-pinned launcher bytes")
	}
}
