package build

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/karty-game/karty/internal/contextio"
)

// Version 1 describes decoded HTTP response bytes. Servers remain responsible
// for transport compression; release and cache identities never depend on it.
const (
	webReleaseVersion = 1
	webReleaseLimit   = 8 << 20
	webReleaseFiles   = 16384
	webReleaseMaxFile = 128 << 20
)

type webReleaseManifest struct {
	Version int              `json:"version"`
	Files   []webReleaseFile `json:"files"`
}

type webReleaseFile struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
	Type   string `json:"type"`
	Role   string `json:"role"`
}

type webReleasePointer struct {
	Version  int    `json:"version"`
	Release  string `json:"release"`
	Manifest string `json:"manifest"`
}

// stageWebRelease publishes the immutable manifest before its discovery
// pointer. Existing release manifests and content-addressed assets are retained
// so deployment tools can keep serving files still used by older sessions.
func stageWebRelease(ctx context.Context, directory string, host *cdnHost) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	root, err := os.OpenRoot(directory)
	if err != nil {
		return fmt.Errorf("open web release directory: %w", err)
	}

	defer root.Close()

	manifest, err := collectWebRelease(ctx, root, host)
	if err != nil {
		return err
	}

	contents, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("encode web release: %w", err)
	}

	contents = append(contents, '\n')

	if len(contents) > webReleaseLimit {
		return fmt.Errorf("web release manifest exceeds its size bound: %w", os.ErrInvalid)
	}

	digest := sha256.Sum256(contents)
	release := hex.EncodeToString(digest[:])
	manifestPath := "karty-releases/" + release + ".json"

	if err := ensureWebReleaseDirectory(root); err != nil {
		return err
	}

	if err := writeWebReleaseManifest(ctx, root, manifestPath, contents); err != nil {
		return err
	}

	pointer, err := json.Marshal(webReleasePointer{Version: webReleaseVersion, Release: release, Manifest: manifestPath})
	if err != nil {
		return fmt.Errorf("encode web release pointer: %w", err)
	}

	return replaceWebReleasePointer(ctx, root, append(pointer, '\n'))
}

func collectWebRelease(ctx context.Context, root *os.Root, host *cdnHost) (webReleaseManifest, error) {
	manifest := webReleaseManifest{Version: webReleaseVersion}

	for _, name := range []string{"index.html", "karty.js", "wasm_exec.js", "game.kart", "karty-web.js"} {
		entry, err := inspectWebReleaseFile(ctx, root, name, "startup")
		if err != nil {
			return manifest, err
		}

		manifest.Files = append(manifest.Files, entry)
	}

	for _, name := range []string{"renderer-debug.js", "karty-web-status.js"} {
		if _, err := root.Lstat(name); err == nil {
			entry, err := inspectWebReleaseFile(ctx, root, name, "startup")
			if err != nil {
				return manifest, err
			}

			manifest.Files = append(manifest.Files, entry)
		} else if !os.IsNotExist(err) {
			return manifest, err
		}
	}

	if host == nil {
		entry, err := inspectWebReleaseFile(ctx, root, "karty-host.wasm", "startup")
		if err != nil {
			return manifest, err
		}

		manifest.Files = append(manifest.Files, entry)
	} else {
		if err := validateCDNURL(host.URL); err != nil {
			return manifest, err
		}

		if !validWebReleaseDigest(host.SHA256) {
			return manifest, fmt.Errorf("shared web host metadata is invalid: %w", os.ErrInvalid)
		}

		// Signed SDK metadata currently has no decoded size. Zero is allowed
		// only for this shared host: the worker still enforces the host's
		// 128 MiB decoded cap and verifies its mandatory digest. CDN builds
		// need not download and duplicate the host just to measure its size.
		manifest.Files = append(manifest.Files, webReleaseFile{
			URL: host.URL, SHA256: host.SHA256, Size: 0, Type: "application/wasm", Role: "startup",
		})
	}

	entries, err := webReleaseContent(root)
	if err != nil {
		return manifest, err
	}

	for _, name := range entries {
		if len(manifest.Files) >= webReleaseFiles {
			return manifest, fmt.Errorf("web release file count exceeds its bound: %w", os.ErrInvalid)
		}

		entry, err := inspectWebReleaseFile(ctx, root, name, "background")
		if err != nil {
			return manifest, err
		}

		if strings.TrimSuffix(path.Base(name), path.Ext(name)) != entry.SHA256 {
			return manifest, fmt.Errorf("web content filename differs from its digest: %w", os.ErrInvalid)
		}

		manifest.Files = append(manifest.Files, entry)
	}

	slices.SortFunc(manifest.Files, func(left, right webReleaseFile) int { return strings.Compare(left.URL, right.URL) })

	for index := 1; index < len(manifest.Files); index++ {
		if manifest.Files[index-1].URL == manifest.Files[index].URL {
			return manifest, fmt.Errorf("duplicate web release URL: %w", os.ErrInvalid)
		}
	}

	return manifest, nil
}

func webReleaseContent(root *os.Root) ([]string, error) {
	info, err := root.Lstat("content")

	if os.IsNotExist(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("web content must be a real directory: %w", os.ErrInvalid)
	}

	directory, err := root.Open("content")
	if err != nil {
		return nil, err
	}

	defer directory.Close()

	entries, err := directory.ReadDir(webReleaseFiles + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}

	if len(entries) > webReleaseFiles {
		return nil, fmt.Errorf("web content file count exceeds its bound: %w", os.ErrInvalid)
	}

	names := make([]string, 0, len(entries))

	for _, entry := range entries {
		extension := path.Ext(entry.Name())

		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() ||
			(extension != ".kld" && extension != ".kaud" && extension != ".kvid") ||
			!validWebReleaseDigest(strings.TrimSuffix(entry.Name(), extension)) {
			return nil, fmt.Errorf("unexpected web content entry %q: %w", entry.Name(), os.ErrInvalid)
		}

		names = append(names, "content/"+entry.Name())
	}

	return names, nil
}

func inspectWebReleaseFile(ctx context.Context, root *os.Root, name, role string) (webReleaseFile, error) {
	entry := webReleaseFile{URL: name, Role: role, Type: "application/octet-stream"}

	info, err := root.Lstat(name)
	if err != nil {
		return entry, fmt.Errorf("inspect web release %s: %w", name, err)
	}

	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > webReleaseMaxFile {
		return entry, fmt.Errorf("web release %s must be a bounded regular file: %w", name, os.ErrInvalid)
	}

	if strings.HasSuffix(name, ".kld") && info.Size() > 32<<20 {
		return entry, fmt.Errorf("web level exceeds its size bound: %w", os.ErrInvalid)
	}

	file, err := root.Open(name)
	if err != nil {
		return entry, err
	}

	defer file.Close()

	digest := sha256.New()

	size, err := contextio.Copy(ctx, digest, io.LimitReader(file, info.Size()+1))
	if err != nil {
		return entry, err
	}

	if size != info.Size() {
		return entry, fmt.Errorf("web release file changed while hashing: %w", os.ErrInvalid)
	}

	entry.Size, entry.SHA256 = size, hex.EncodeToString(digest.Sum(nil))

	switch path.Ext(name) {
	case ".html":
		entry.Type = "text/html; charset=utf-8"
	case ".js":
		entry.Type = "text/javascript; charset=utf-8"
	case ".wasm", ".kart", ".kld":
		entry.Type = "application/wasm"
	}

	return entry, nil
}

func validWebReleaseDigest(value string) bool {
	decoded, err := hex.DecodeString(value)

	return err == nil && len(decoded) == sha256.Size && value == strings.ToLower(value)
}

func ensureWebReleaseDirectory(root *os.Root) error {
	if err := root.Mkdir("karty-releases", 0o750); err != nil && !os.IsExist(err) {
		return err
	}

	info, err := root.Lstat("karty-releases")
	if err != nil {
		return err
	}

	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("web releases must be a real directory: %w", os.ErrInvalid)
	}

	return nil
}

func writeWebReleaseManifest(ctx context.Context, root *os.Root, name string, contents []byte) error {
	info, err := root.Lstat(name)
	if err == nil {
		if !info.Mode().IsRegular() || info.Size() != int64(len(contents)) {
			return fmt.Errorf("immutable web release manifest conflicts: %w", os.ErrInvalid)
		}

		file, err := root.Open(name)
		if err != nil {
			return err
		}

		defer file.Close()

		existing, err := contextio.ReadAll(ctx, io.LimitReader(file, int64(len(contents))+1))
		if err != nil {
			return err
		}

		if !bytes.Equal(existing, contents) {
			return fmt.Errorf("immutable web release manifest changed: %w", os.ErrInvalid)
		}

		return nil
	}

	if !os.IsNotExist(err) {
		return err
	}

	return writeWebReleaseAtomic(ctx, root, name, contents)
}

func replaceWebReleasePointer(ctx context.Context, root *os.Root, contents []byte) error {
	if info, err := root.Lstat("karty-latest.json"); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("web release pointer must be a regular file: %w", os.ErrInvalid)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	return writeWebReleaseAtomic(ctx, root, "karty-latest.json", contents)
}

func writeWebReleaseAtomic(ctx context.Context, root *os.Root, name string, contents []byte) error {
	temporary := path.Join(path.Dir(name), ".karty-release-"+rand.Text())

	file, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}

	defer func() { _ = root.Remove(temporary) }()

	_, writeErr := contextio.Copy(ctx, file, bytes.NewReader(contents))
	closeErr := file.Close()

	if writeErr != nil {
		return writeErr
	}

	if closeErr != nil {
		return closeErr
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	return root.Rename(temporary, name)
}
