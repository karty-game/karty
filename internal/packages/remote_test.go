package packages

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func remoteFixture(t *testing.T) ([]byte, string, string) {
	t.Helper()

	kit, project := fixture(t)
	archive := filepath.Join(t.TempDir(), "kit.kpkg")
	digest := export(t, kit, archive)

	return read(t, archive), digest, project
}

func declareRemote(t *testing.T, project, source, digest string) {
	t.Helper()

	write(t, filepath.Join(project, "karty.toml"), []byte(fmt.Sprintf(
		"[sdk]\nversion='0.0.13'\n[packages.placeholder-kit]\narchive=%q\nsha256=%q\n", source, digest)))
}

func remoteRestore(t *testing.T, project string, client *http.Client) *Set {
	t.Helper()

	set, err := restoreWithClient(t.Context(), project, client)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(set.Close)

	return set
}

func TestRemoteArchiveColdWarmOfflineAndTamper(t *testing.T) {
	t.Parallel()

	data, digest, project := remoteFixture(t)

	var requests atomic.Int32

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)

		_, _ = w.Write(data)
	}))
	t.Cleanup(server.Close)
	declareRemote(t, project, server.URL+"/kit.kpkg", digest)

	set := remoteRestore(t, project, server.Client())
	if set.Items["placeholder-kit"].Manifest.Version != "0.1.0" || requests.Load() != 1 {
		t.Fatal("remote archive did not restore its pinned manifest")
	}

	set.Close()

	cache := filepath.Join(project, filepath.FromSlash(remoteArchiveCachePath(digest)))
	if !bytes.Equal(read(t, cache), data) {
		t.Fatal("download cache differs from the pinned archive")
	}

	server.Close()

	if err := os.RemoveAll(filepath.Join(project, ".karty/packages/.store")); err != nil {
		t.Fatal(err)
	}

	remoteRestore(t, project, server.Client())

	if requests.Load() != 1 {
		t.Fatal("offline restoration attempted another download")
	}

	write(t, cache, []byte("tampered"))

	if next, err := restoreWithClient(t.Context(), project, server.Client()); err == nil {
		next.Close()
		t.Fatal("warm download cache bypassed checksum validation")
	}

	if err := os.Remove(cache); err != nil {
		t.Fatal(err)
	}

	outside := filepath.Join(t.TempDir(), "outside.kpkg")
	write(t, outside, data)

	if err := os.Symlink(outside, cache); err != nil {
		t.Fatal(err)
	}

	if next, err := restoreWithClient(t.Context(), project, server.Client()); err == nil {
		next.Close()
		t.Fatal("download cache followed a symlink outside the project")
	}
}

func TestRemoteArchiveURLAndChecksumValidation(t *testing.T) {
	t.Parallel()

	valid := "https://github.com/karty-game/karty-fps-kit/releases/download/v0.1.0/fps-kit-0.1.0.kpkg"
	if !validRemoteArchiveSource(valid) {
		t.Fatal("immutable HTTPS release URL rejected")
	}

	for _, source := range []string{
		"http://example.com/kit.kpkg", "file:///kit.kpkg", "https:///kit.kpkg", "https://user:pass@example.com/kit.kpkg",
		"https://example.com/kit.kpkg?token=x", "https://example.com/kit.kpkg?", "https://example.com/kit.kpkg#fragment",
		"https://example.com/kit.kpkg#",
		"https://example.com/../kit.kpkg", "https://example.com/a//kit.kpkg", "https://example.com/%2e%2e/kit.kpkg",
		"https://example.com/a%2fkit.kpkg", "https://example.com/a%5ckit.kpkg", "https://example.com/kit.zip",
	} {
		if validRemoteArchiveSource(source) || validateSources(map[string]Source{
			"kit": {Archive: source, SHA256: strings.Repeat("a", 64)},
		}, nil) == nil {
			t.Fatalf("accepted unsafe remote source %q", source)
		}
	}

	for _, digest := range []string{"", strings.Repeat("A", 64), strings.Repeat("g", 64)} {
		if err := validateSources(map[string]Source{"kit": {Archive: valid, SHA256: digest}}, nil); err == nil {
			t.Fatalf("accepted remote source without a valid pinned checksum: %q", digest)
		}
	}
}

func TestRemoteArchiveRejectsInvalidResponsesBeforeCaching(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"checksum", "truncated", "oversized", "status", "invalid-zip"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()

			data, digest, project := remoteFixture(t)
			if mode == "invalid-zip" {
				data = []byte("not an archive")
				digest = Digest(data)
			}

			server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				switch mode {
				case "checksum":
					_, _ = writer.Write([]byte("wrong archive"))
				case "truncated":
					writer.Header().Set("Content-Length", strconv.Itoa(len(data)+1))
					_, _ = writer.Write(data)
				case "oversized":
					writer.Header().Set("Content-Length", strconv.Itoa(MaxArchiveBytes+1))
				case "status":
					writer.WriteHeader(http.StatusNotFound)
				default:
					_, _ = writer.Write(data)
				}
			}))
			t.Cleanup(server.Close)
			declareRemote(t, project, server.URL+"/kit.kpkg", digest)

			if set, err := restoreWithClient(t.Context(), project, server.Client()); err == nil {
				set.Close()
				t.Fatal("invalid remote archive was accepted")
			}

			if _, err := os.Stat(filepath.Join(project, filepath.FromSlash(remoteArchiveCachePath(digest)))); !os.IsNotExist(err) {
				t.Fatalf("invalid remote archive was cached: %v", err)
			}
		})
	}
}

func TestRemoteArchiveStreamingLimitAndCancellation(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = http.NewResponseController(w).Flush()
		_, _ = w.Write(bytes.Repeat([]byte("x"), 64))
	}))
	t.Cleanup(server.Close)

	if _, err := downloadRemoteArchive(
		t.Context(),
		remoteArchiveClient(server.Client()),
		server.URL+"/kit.kpkg",
		32,
	); !errors.Is(
		err,
		ErrPackage,
	) {
		t.Fatalf("unannounced oversized response exceeded the streaming bound: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	started := make(chan struct{})
	blocked := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		_ = http.NewResponseController(w).Flush()

		close(started)
		<-request.Context().Done()
	}))
	t.Cleanup(blocked.Close)

	result := make(chan error, 1)

	go func() {
		_, err := downloadRemoteArchive(ctx, remoteArchiveClient(blocked.Client()), blocked.URL+"/kit.kpkg", MaxArchiveBytes)
		result <- err
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("download did not start")
	}

	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("download lost cancellation: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation did not stop the download")
	}
}

func TestRemoteArchiveRedirectSecurity(t *testing.T) {
	t.Parallel()

	data, digest, _ := remoteFixture(t)

	var server *httptest.Server

	server = httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/release.kpkg":
			http.Redirect(writer, request, server.URL+"/cdn?signature=x", http.StatusFound)
		case "/http.kpkg":
			http.Redirect(writer, request, "http://example.com/kit.kpkg", http.StatusFound)
		case "/credentials.kpkg":
			http.Redirect(writer, request, strings.Replace(server.URL, "https://", "https://user:pass@", 1)+"/cdn", http.StatusFound)
		case "/fragment.kpkg":
			http.Redirect(writer, request, server.URL+"/cdn#fragment", http.StatusFound)
		case "/loop.kpkg":
			http.Redirect(writer, request, server.URL+"/loop.kpkg", http.StatusFound)
		default:
			_, _ = writer.Write(data)
		}
	}))
	t.Cleanup(server.Close)
	project := t.TempDir()
	declareRemote(t, project, server.URL+"/release.kpkg", digest)
	remoteRestore(t, project, server.Client())

	for _, path := range []string{"http", "credentials", "fragment", "loop"} {
		project := t.TempDir()
		declareRemote(t, project, server.URL+"/"+path+".kpkg", digest)

		if set, err := restoreWithClient(t.Context(), project, server.Client()); err == nil {
			set.Close()
			t.Fatalf("accepted unsafe/excessive redirect %q", path)
		}
	}
}

func TestRemoteArchiveBadLaterDependencyPreservesExistingLinks(t *testing.T) {
	t.Parallel()

	kit, project := fixture(t)
	if _, err := Import(t.Context(), project, kit, false); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(project, ".karty/packages/placeholder-kit")

	previous, err := filepath.EvalSymlinks(link)
	if err != nil {
		t.Fatal(err)
	}

	name := filepath.Join(kit, ManifestName)
	write(t, name, bytes.ReplaceAll(read(t, name), []byte(`version = "0.1.0"`), []byte(`version = "0.1.1"`)))
	archive := filepath.Join(t.TempDir(), "updated.kpkg")
	digest := export(t, kit, archive)
	data := read(t, archive)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(data) }))
	t.Cleanup(server.Close)
	declareRemote(t, project, server.URL+"/updated.kpkg", digest)
	contents := read(t, filepath.Join(project, "karty.toml"))
	contents = append(
		contents,
		fmt.Appendf(nil, "[packages.zzbad]\narchive=%q\nsha256=%q\n", server.URL+"/bad.kpkg", strings.Repeat("a", 64))...)
	write(t, filepath.Join(project, "karty.toml"), contents)

	if set, err := restoreWithClient(t.Context(), project, server.Client()); err == nil {
		set.Close()
		t.Fatal("bad later dependency was accepted")
	}

	current, err := filepath.EvalSymlinks(link)
	if err != nil || current != previous {
		t.Fatalf("earlier package link changed before complete validation: %q: %v", current, err)
	}
}
