package devserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixtureServer(t *testing.T) *Server {
	t.Helper()

	server, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(server.Close)

	return server
}

func request(server *Server, path string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil))

	return response
}

func TestServerKeepsLastGoodSnapshotAndPinsHTML(t *testing.T) {
	t.Parallel()

	server := fixtureServer(t)
	root := t.TempDir()
	fixtureBuild(t, root, []byte("old level"), []byte("code"), nil)
	first := captureFixture(t, root, 1)
	server.Publish(first)

	response := request(server, "/")
	if !strings.Contains(response.Body.String(), `<base href="`+first.Generation.BaseURL+`">`) ||
		!strings.Contains(response.Body.String(), "kartyDevelopmentBoot") {
		t.Fatal("page assets are not pinned to generation")
	}

	config := strings.Index(response.Body.String(), `globalThis.kartyDevelopmentConfig={schema:"karty.dev@1"}`)

	launcher := strings.Index(response.Body.String(), `<script src="karty.js?v=`)
	if config < 0 || launcher < 0 || config > launcher {
		t.Fatal("development opt-in must execute synchronously before the host launcher")
	}

	server.SetStatus("error", "<script>build failed</script>")

	var status Status
	if err := json.Unmarshal(request(server, "/__karty_dev/status").Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}

	if status.State != "error" || status.Generation.ID != first.Generation.ID {
		t.Fatal("build failure discarded current generation")
	}

	nextRoot := t.TempDir()
	fixtureBuild(t, nextRoot, []byte("new level"), []byte("code"), nil)
	server.Publish(captureFixture(t, nextRoot, 2))

	old := request(server, "/"+first.Generation.BaseURL+first.Generation.Levels[0].Artifact)
	if old.Body.String() != "old level" || !strings.Contains(old.Header().Get("Cache-Control"), "immutable") {
		t.Fatal("previous immutable resource was lost")
	}

	for _, path := range []string{"/../game.kart", "/a/../game.kart", "/a\\game.kart", "/__karty_dev/generations/unknown/game.kart"} {
		if got := request(server, path).Code; got != http.StatusNotFound {
			t.Fatalf("unsafe path %s returned %d", path, got)
		}
	}
}

type eventRecorder struct {
	*httptest.ResponseRecorder

	flushed chan struct{}
}

func (recorder *eventRecorder) Flush() {
	recorder.ResponseRecorder.Flush()

	select {
	case recorder.flushed <- struct{}{}:
	default:
	}
}

func TestSSEStartsWithSnapshotAndShutsDown(t *testing.T) {
	t.Parallel()

	server := fixtureServer(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	recorder := &eventRecorder{ResponseRecorder: httptest.NewRecorder(), flushed: make(chan struct{}, 4)}

	done := make(chan struct{})
	go func() {
		defer close(done)

		server.ServeHTTP(recorder, httptest.NewRequestWithContext(ctx, http.MethodGet, "/__karty_dev/events", nil))
	}()

	for range 2 {
		select {
		case <-recorder.flushed:
		case <-time.After(time.Second):
			t.Fatal("initial SSE snapshot buffered")
		}
	}

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SSE did not stop with client")
	}

	if !strings.Contains(recorder.Body.String(), "event: snapshot\n") ||
		recorder.Header().Get("Cache-Control") != "no-store, no-transform" {
		t.Fatal("missing snapshot or proxy compression guard")
	}
}

func TestProgressBurstsCoalesceWithoutDisconnectingAndGenerationEviction(t *testing.T) {
	t.Parallel()

	server := fixtureServer(t)
	server.snapshots = make([]*Snapshot, 0, maxGenerations+1)
	backing := server.snapshots[:cap(server.snapshots)]
	client := make(chan []byte, 1)
	server.clients[client] = struct{}{}
	server.SetStatus("building", "one")
	server.SetStatus("building", "two")

	if len(server.clients) != 1 || len(client) != 1 {
		t.Fatal("progress burst disconnected a healthy stream or exceeded its queue bound")
	}

	var latest Status
	if err := json.Unmarshal(<-client, &latest); err != nil {
		t.Fatal(err)
	}

	if latest.State != "building" || latest.Message != "two" {
		t.Fatal("queued status did not coalesce to the latest progress")
	}

	for index := range maxGenerations + 1 {
		snapshot := &Snapshot{Generation: Generation{ID: string(rune('a' + index))}, size: 1}
		server.Publish(snapshot)
	}

	if len(server.snapshots) != maxGenerations || server.snapshots[0].Generation.ID != "b" {
		t.Fatal("old generations did not expire")
	}

	if backing[0] != nil {
		t.Fatal("evicted snapshot remains reachable through slice backing storage")
	}

	server.Publish(&Snapshot{Generation: Generation{ID: "large"}, size: maxSnapshotBytes})
	server.Publish(&Snapshot{Generation: Generation{ID: "newest"}, size: maxSnapshotBytes})

	if len(server.snapshots) != 1 || server.snapshots[0].Generation.ID != "newest" {
		t.Fatal("retained bytes exceeded budget")
	}

	if err := json.Unmarshal(<-client, &latest); err != nil {
		t.Fatal(err)
	}

	if latest.State != "ready" || latest.Generation.ID != "newest" || len(server.clients) != 1 {
		t.Fatal("generation burst did not deliver the latest complete snapshot on the same stream")
	}
}

func TestSSEConnectionsRemainBounded(t *testing.T) {
	t.Parallel()

	server := fixtureServer(t)
	for range maxClients {
		server.clients[make(chan []byte, 1)] = struct{}{}
	}

	response := request(server, "/__karty_dev/events")
	if response.Code != http.StatusServiceUnavailable || len(server.clients) != maxClients {
		t.Fatal("SSE connection limit was exceeded")
	}
}

func TestEvictedGenerationServesOnlyVerifiedContentAddressedResources(t *testing.T) {
	t.Parallel()

	server := fixtureServer(t)
	resources := map[string][]byte{}

	for _, extension := range []string{".kld", ".kaud", ".kvid"} {
		data := []byte("shared content " + extension)
		resources["content/"+digest(data)+extension] = data
	}

	corrupt := "content/" + digest([]byte("expected content")) + ".kaud"
	resources[corrupt] = []byte("corrupt stored content")
	unsupported := "content/" + digest([]byte("unhashed extension")) + ".txt"
	resources[unsupported] = []byte("unhashed extension")

	var boot *Snapshot

	for index := range maxGenerations + 1 {
		root := t.TempDir()
		fixtureBuild(t, root, []byte{byte(index)}, []byte("code"), nil)

		for name, data := range resources {
			if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), data, 0o600); err != nil {
				t.Fatal(err)
			}
		}

		snapshot := captureFixture(t, root, uint64(index+1))
		if boot == nil {
			boot = snapshot
		}

		server.Publish(snapshot)
	}

	if server.snapshots[0].Generation.ID == boot.Generation.ID {
		t.Fatal("boot generation was not evicted")
	}

	for name, data := range resources {
		response := request(server, "/"+boot.Generation.BaseURL+name)
		if name == corrupt || name == unsupported {
			if response.Code != http.StatusNotFound {
				t.Fatalf("unsafe fallback %s accepted", name)
			}

			continue
		}

		if response.Code != http.StatusOK || response.Body.String() != string(data) ||
			!strings.Contains(response.Header().Get("Cache-Control"), "immutable") {
			t.Fatalf("unchanged content unavailable after boot eviction: %s", name)
		}
	}

	for _, name := range []string{"index.html", "karty.js", "game.kart", "content/" + strings.Repeat("e", 64) + ".kld"} {
		if response := request(server, "/"+boot.Generation.BaseURL+name); response.Code != http.StatusNotFound {
			t.Fatalf("unavailable generation artifact unexpectedly fell back: %s", name)
		}
	}

	for _, id := range []string{"unknown", strings.Repeat("A", 64), strings.Repeat("g", 64)} {
		for name := range resources {
			if response := request(server, "/__karty_dev/generations/"+id+"/"+name); response.Code != http.StatusNotFound {
				t.Fatal("malformed generation ID accepted")
			}
		}
	}

	if len(server.snapshots) != maxGenerations {
		t.Fatal("fallback revived an evicted snapshot")
	}
}

func TestRetainedBootGenerationResolvesNewContentAddressedChunk(t *testing.T) {
	t.Parallel()

	server := fixtureServer(t)
	root := t.TempDir()
	fixtureBuild(t, root, []byte("level"), []byte("code"), nil)
	boot := captureFixture(t, root, 1)
	server.Publish(boot)

	chunk := []byte("future level chunk")

	artifact := "content/" + digest(chunk) + ".kld"
	if err := os.WriteFile(filepath.Join(root, artifact), chunk, 0o600); err != nil {
		t.Fatal(err)
	}

	server.Publish(captureFixture(t, root, 2))

	response := request(server, "/"+boot.Generation.BaseURL+artifact)
	if response.Code != http.StatusOK || response.Body.String() != string(chunk) {
		t.Fatal("future chunk was unavailable through retained boot base")
	}

	if len(server.snapshots) != 2 {
		t.Fatal("fallback altered generation retention")
	}
}

type deadlineRecorder struct {
	*httptest.ResponseRecorder

	deadlines []time.Time
	failFlush bool
}

func (recorder *deadlineRecorder) SetWriteDeadline(deadline time.Time) error {
	recorder.deadlines = append(recorder.deadlines, deadline)

	return nil
}

func (recorder *deadlineRecorder) FlushError() error {
	if recorder.failFlush {
		return io.ErrClosedPipe
	}

	recorder.Flush()

	return nil
}

func TestSSEWriteDeadlineBoundsWritesAndAllowsIdleHeartbeat(t *testing.T) {
	t.Parallel()

	recorder := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	if !writeEvent(recorder, "snapshot", []byte("{}")) || !writeEvent(recorder, "heartbeat", []byte("{}")) {
		t.Fatal("healthy stream write failed")
	}

	if len(recorder.deadlines) != 4 || recorder.deadlines[0].IsZero() || !recorder.deadlines[1].IsZero() ||
		recorder.deadlines[2].IsZero() || !recorder.deadlines[3].IsZero() {
		t.Fatal("write deadlines must clear between heartbeat writes")
	}

	recorder.failFlush = true
	if writeEvent(recorder, "heartbeat", []byte("{}")) {
		t.Fatal("failed stream flush was ignored")
	}
}
