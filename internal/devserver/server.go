package devserver

import (
	"bytes"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"path"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	Schema             = "karty.dev@1"
	maxRetainedBytes   = 768 << 20
	maxGenerations     = 4
	heartbeatInterval  = 15 * time.Second
	maxClients         = 64
	maxMessageBytes    = 8192
	streamWriteTimeout = 5 * time.Second
)

//go:embed browser.js
var browserScript []byte

const generationPathParts = 2
const digestHexCharacters = 64

type Status struct {
	Schema     string      `json:"schema"`
	Session    string      `json:"session"`
	State      string      `json:"state"`
	Message    string      `json:"message"`
	Generation *Generation `json:"generation"`
}

type Server struct {
	mu        sync.Mutex
	status    Status
	snapshots []*Snapshot
	clients   map[chan []byte]struct{}
	closed    chan struct{}
	once      sync.Once
}

func NewServer() (*Server, error) {
	var session [16]byte
	if _, err := rand.Read(session[:]); err != nil {
		return nil, err
	}

	return &Server{
		status:  Status{Schema: Schema, Session: hex.EncodeToString(session[:]), State: "building", Message: "Building game…"},
		clients: make(map[chan []byte]struct{}),
		closed:  make(chan struct{}),
	}, nil
}

func New() (*Server, error) { return NewServer() }

func (server *Server) Publish(snapshot *Snapshot) {
	server.mu.Lock()
	defer server.mu.Unlock()

	server.snapshots = append(server.snapshots, snapshot)

	var size int64
	for _, item := range server.snapshots {
		size += item.size
	}

	for len(server.snapshots) > 1 && (len(server.snapshots) > maxGenerations || size > maxRetainedBytes) {
		size -= server.snapshots[0].size
		server.snapshots[0] = nil
		server.snapshots = server.snapshots[1:]
	}

	copyGeneration := snapshot.Generation
	server.status.Generation = &copyGeneration
	server.status.State = "ready"
	server.status.Message = "Preview ready"
	server.broadcastLocked()
}

func (server *Server) SetStatus(state, message string) {
	server.mu.Lock()
	defer server.mu.Unlock()

	if len(message) > maxMessageBytes {
		message = message[:maxMessageBytes] + "\n… See the terminal for the complete diagnostic."
	}

	server.status.State = state
	server.status.Message = message
	server.broadcastLocked()
}

func (server *Server) Close() { server.once.Do(func() { close(server.closed) }) }

func (server *Server) ServeHTTP(output http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		output.WriteHeader(http.StatusMethodNotAllowed)

		return
	}

	switch request.URL.Path {
	case "/__karty_dev/events":
		server.events(output, request)

		return
	case "/__karty_dev/status":
		server.statusHTTP(output)

		return
	case "/__karty_dev/browser.js":
		serveBytes(output, request, "browser.js", browserScript, false)

		return
	}

	server.asset(output, request)
}

func (server *Server) statusBytesLocked() []byte {
	data, err := json.Marshal(server.status)
	if err != nil {
		return []byte(`{"schema":"karty.dev@1","state":"error","message":"Could not encode development status"}`)
	}

	return data
}

func (server *Server) broadcastLocked() {
	data := server.statusBytesLocked()
	for client := range server.clients {
		select {
		case client <- data:
		default:
			// Cached build steps can report faster than a healthy stream writes.
			// Keep only the latest snapshot; actual stalled writes are bounded by
			// writeEvent's deadline rather than by progress-report frequency.
			select {
			case <-client:
			default:
			}

			client <- data
		}
	}
}

func (server *Server) statusHTTP(output http.ResponseWriter) {
	server.mu.Lock()
	data := server.statusBytesLocked()
	server.mu.Unlock()
	output.Header().Set("Content-Type", "application/json")
	output.Header().Set("Cache-Control", "no-store")
	_, _ = output.Write(data)
}

func (server *Server) events(output http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		output.WriteHeader(http.StatusMethodNotAllowed)

		return
	}

	flusher, ok := output.(http.Flusher)
	if !ok {
		output.WriteHeader(http.StatusInternalServerError)

		return
	}

	client := make(chan []byte, 1)

	server.mu.Lock()
	if len(server.clients) >= maxClients {
		server.mu.Unlock()
		output.WriteHeader(http.StatusServiceUnavailable)

		return
	}

	server.clients[client] = struct{}{}
	initial := server.statusBytesLocked()

	server.mu.Unlock()
	defer func() { server.mu.Lock(); delete(server.clients, client); server.mu.Unlock() }()

	output.Header().Set("Content-Type", "text/event-stream")
	output.Header().Set("Cache-Control", "no-store, no-transform")
	output.Header().Set("X-Accel-Buffering", "no")
	flusher.Flush()

	if !writeEvent(output, "snapshot", initial) {
		return
	}

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-request.Context().Done():
			return
		case <-server.closed:
			return
		case data, open := <-client:
			if !open || !writeEvent(output, "snapshot", data) {
				return
			}
		case <-ticker.C:
			if !writeEvent(output, "heartbeat", []byte("{}")) {
				return
			}
		}
	}
}

func writeEvent(output http.ResponseWriter, event string, data []byte) bool {
	controller := http.NewResponseController(output)
	_ = controller.SetWriteDeadline(time.Now().Add(streamWriteTimeout))

	_, err := fmt.Fprintf(output, "event: %s\ndata: %s\n\n", event, data)
	if err != nil {
		return false
	}

	if err := controller.Flush(); err != nil {
		return false
	}

	// A write deadline bounds a stalled write, not the idle heartbeat interval.
	_ = controller.SetWriteDeadline(time.Time{})

	return true
}

func (server *Server) asset(output http.ResponseWriter, request *http.Request) {
	name := strings.TrimPrefix(request.URL.Path, "/")
	if name == "" {
		name = "index.html"
	}

	if path.Clean(name) != name || strings.Contains(name, "\\") {
		http.NotFound(output, request)

		return
	}

	reference := server.lookupAsset(name)
	name = reference.name
	immutable := reference.immutable
	snapshot := reference.snapshot

	// An active runtime can keep a boot-generation URL for chunks or media
	// after several resource patches. Only verified content-addressed bytes
	// can cross generation boundaries; code and HTML remain generation-pinned.
	for _, candidate := range reference.fallback {
		if request.Context().Err() != nil {
			return
		}

		if candidate.checksums[name] == reference.checksum {
			serveBytesWithChecksum(output, request, name, candidate.files[name], true, reference.checksum)

			return
		}
	}

	if snapshot == nil {
		if name == "index.html" && !immutable {
			server.waiting(output, request)

			return
		}

		http.NotFound(output, request)

		return
	}

	data, ok := snapshot.files[name]
	if !ok {
		http.NotFound(output, request)

		return
	}

	if name == "index.html" {
		base := "./"
		if !immutable {
			base = snapshot.Generation.BaseURL
		}

		injection := `<head><base href="` + base + `">` + snapshot.bootstrap()
		if request.URL.Query().Get("spector") == "1" {
			injection += spectorScripts()
		}

		data = bytes.Replace(data, []byte("<head>"), []byte(injection), 1)
		immutable = false
		serveBytes(output, request, name, data, immutable)

		return
	}

	serveBytesWithChecksum(output, request, name, data, immutable, snapshot.checksums[name])
}

type assetReference struct {
	snapshot  *Snapshot
	name      string
	immutable bool
	checksum  string
	fallback  []*Snapshot
}

func (server *Server) lookupAsset(name string) assetReference {
	server.mu.Lock()
	defer server.mu.Unlock()

	reference := assetReference{name: name, immutable: strings.HasPrefix(name, "__karty_dev/generations/")}
	if !reference.immutable {
		if len(server.snapshots) > 0 {
			reference.snapshot = server.snapshots[len(server.snapshots)-1]
		}

		return reference
	}

	parts := strings.SplitN(strings.TrimPrefix(name, "__karty_dev/generations/"), "/", generationPathParts)
	if len(parts) != generationPathParts || !canonicalDigest(parts[0]) {
		return reference
	}

	reference.name = parts[1]
	for _, candidate := range server.snapshots {
		if candidate.Generation.ID == parts[0] {
			reference.snapshot = candidate

			break
		}
	}

	if reference.snapshot != nil {
		if _, present := reference.snapshot.files[reference.name]; present {
			return reference
		}
	}

	reference.checksum = contentDigest(reference.name)
	if reference.checksum == "" {
		return reference
	}

	for _, candidate := range slices.Backward(server.snapshots) {
		if _, ok := candidate.files[reference.name]; ok {
			reference.fallback = append(reference.fallback, candidate)
		}
	}

	return reference
}

func canonicalDigest(value string) bool {
	if len(value) != digestHexCharacters || strings.ToLower(value) != value {
		return false
	}

	_, err := hex.DecodeString(value)

	return err == nil
}

func contentDigest(name string) string {
	if !strings.HasPrefix(name, "content/") {
		return ""
	}

	filename := strings.TrimPrefix(name, "content/")

	extension := path.Ext(filename)
	if extension != ".kld" && extension != ".kaud" && extension != ".kvid" {
		return ""
	}

	checksum := strings.TrimSuffix(filename, extension)
	if !canonicalDigest(checksum) {
		return ""
	}

	return checksum
}

func (server *Server) waiting(output http.ResponseWriter, request *http.Request) {
	snapshot := &Snapshot{}
	data := []byte(
		`<!doctype html><html><head><meta name="viewport" content="width=device-width,initial-scale=1">` + snapshot.bootstrap() + `</head><body style="background:#10131c;color:#edf1fa;font:16px system-ui;padding:24px"><p>Building game…</p></body></html>`,
	)
	serveBytes(output, request, "index.html", data, false)
}

func serveBytes(output http.ResponseWriter, request *http.Request, name string, data []byte, immutable bool) {
	checksum, err := digestContext(request.Context(), data)
	if err != nil {
		return
	}

	serveBytesWithChecksum(output, request, name, data, immutable, checksum)
}

func serveBytesWithChecksum(output http.ResponseWriter, request *http.Request, name string, data []byte, immutable bool, checksum string) {
	if request.Context().Err() != nil {
		return
	}

	contentType := mime.TypeByExtension(path.Ext(name))
	if strings.HasSuffix(name, ".wasm") || strings.HasSuffix(name, ".kart") || strings.HasSuffix(name, ".kld") {
		contentType = "application/wasm"
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	output.Header().Set("Content-Type", contentType)
	output.Header().Set("X-Content-Type-Options", "nosniff")
	output.Header().Set("ETag", `"`+checksum+`"`)
	output.Header().Set("Cache-Control", "no-store")

	if immutable {
		output.Header().Set("Cache-Control", "private,max-age=31536000,immutable")
	}

	http.ServeContent(output, request, name, time.Time{}, bytes.NewReader(data))
}
