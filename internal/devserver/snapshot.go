// Package devserver serves immutable development builds and reconnecting updates.
package devserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/karty-game/karty-sdk/format/cartridge"
)

const (
	maxArtifactBytes     = 128 << 20
	maxSnapshotBytes     = 512 << 20
	maxSnapshotFiles     = 8192
	maxLevelBytes        = 32 << 20
	shellHashLength      = 16
	maxWASMSectionLength = 1<<32 - 1
)

var ErrSnapshot = errors.New("invalid or oversized development build")

type Level struct {
	Name            string `json:"name"`
	Kind            string `json:"kind"`
	ContentSHA256   string `json:"contentSha256"`
	Size            uint64 `json:"size"`
	EnvelopeVersion uint16 `json:"envelopeVersion"`
	Artifact        string `json:"artifact"`
}

type Generation struct {
	Revision        uint64     `json:"revision"`
	ID              string     `json:"id"`
	RuntimeIdentity string     `json:"runtimeIdentity"`
	BaseURL         string     `json:"baseURL"` //nolint:tagliatelle // The versioned tooling schema spells URL as an initialism.
	Levels          []Level    `json:"levels"`
	UI              UIResource `json:"ui"`
}

type Snapshot struct {
	Generation Generation
	files      map[string][]byte
	size       int64
	ui         UIResource
}

// Capture owns all served bytes before publication. No requests read dist again.
func Capture(ctx context.Context, directory string, revision uint64) (*Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	root, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}

	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrSnapshot
	}

	confined, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer confined.Close()

	snapshot := &Snapshot{files: make(map[string][]byte)}

	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink in build: %w", ErrSnapshot)
		}

		if entry.IsDir() {
			return nil
		}

		if !entry.Type().IsRegular() {
			return ErrSnapshot
		}

		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		if len(snapshot.files) >= maxSnapshotFiles {
			return ErrSnapshot
		}

		data, err := readSnapshotFile(ctx, confined, name, maxSnapshotBytes-snapshot.size)
		if err != nil {
			return err
		}

		if strings.HasPrefix(filepath.ToSlash(name), "__karty_dev/") {
			return ErrSnapshot
		}

		snapshot.files[filepath.ToSlash(name)] = data
		snapshot.size += int64(len(data))

		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := snapshot.identify(ctx, revision); err != nil {
		return nil, err
	}

	return snapshot, nil
}

func readSnapshotFile(ctx context.Context, root *os.Root, path string, remaining int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Recheck the opened descriptor as well as WalkDir's entry before reading.
	file, err := root.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if !info.Mode().IsRegular() || info.Size() > maxArtifactBytes || info.Size() > remaining {
		return nil, ErrSnapshot
	}

	limit := min(int64(maxArtifactBytes), remaining)

	data, err := readSnapshotContents(ctx, file, limit)
	if err != nil {
		return nil, err
	}

	if int64(len(data)) > limit {
		return nil, ErrSnapshot
	}

	return data, nil
}

func (snapshot *Snapshot) identify(ctx context.Context, revision uint64) error {
	for _, name := range []string{"index.html", "karty.js", "wasm_exec.js", "game.kart"} {
		if len(snapshot.files[name]) == 0 {
			return fmt.Errorf("missing %s: %w", name, ErrSnapshot)
		}
	}

	manifestBytes, _, err := manifestSection(ctx, snapshot.files["game.kart"])
	if err != nil {
		return err
	}

	manifest, err := cartridge.DecodeManifest(manifestBytes)
	if err != nil {
		return err
	}

	levels := make([]Level, 0, len(manifest.Levels))
	for _, entry := range manifest.Levels {
		artifact := "content/" + entry.ContentSHA256 + ".kld"

		data, ok := snapshot.files[artifact]
		if !ok || entry.Size > maxLevelBytes || uint64(len(data)) != entry.Size {
			return fmt.Errorf("level %s: %w", entry.Name, ErrSnapshot)
		}

		checksum, err := digestContext(ctx, data)
		if err != nil {
			return err
		}

		if checksum != entry.ContentSHA256 {
			return fmt.Errorf("level %s: %w", entry.Name, ErrSnapshot)
		}

		levels = append(levels, Level{entry.Name, entry.Kind, entry.ContentSHA256, entry.Size, entry.EnvelopeVersion, artifact})
	}

	if err := snapshot.captureUI(ctx); err != nil {
		return err
	}

	runtime, err := snapshot.runtimeIdentity(ctx, manifest)
	if err != nil {
		return err
	}

	names := make([]string, 0, len(snapshot.files))
	for name := range snapshot.files {
		names = append(names, name)
	}

	sort.Strings(names)

	identity := sha256.New()

	for _, name := range names {
		checksum, err := digestContext(ctx, snapshot.files[name])
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintf(identity, "%s\x00%s\n", name, checksum)
	}

	id := hex.EncodeToString(identity.Sum(nil))
	snapshot.Generation = Generation{
		Revision:        revision,
		ID:              id,
		RuntimeIdentity: runtime,
		BaseURL:         "__karty_dev/generations/" + id + "/",
		Levels:          levels,
		UI:              snapshot.ui,
	}

	return nil
}

func (snapshot *Snapshot) runtimeIdentity(ctx context.Context, manifest cartridge.Manifest) (string, error) {
	for index := range manifest.Levels {
		manifest.Levels[index].ContentSHA256 = strings.Repeat("0", 64)
		manifest.Levels[index].Size = 1
	}

	normalized, err := cartridge.EncodeManifest(manifest)
	if err != nil {
		return "", err
	}

	wasm, err := normalizeManifest(ctx, snapshot.files["game.kart"], normalized)
	if err != nil {
		return "", err
	}

	wasm, err = normalizeUIRuntime(ctx, wasm)
	if err != nil {
		return "", err
	}

	clientHash, err := digestContext(ctx, snapshot.files["game.kart"])
	if err != nil {
		return "", err
	}

	launcherHash, err := digestContext(ctx, snapshot.files["karty.js"])
	if err != nil {
		return "", err
	}

	launcher, err := normalizeShellHash(ctx, snapshot.files["karty.js"], "game.kart", clientHash)
	if err != nil {
		return "", err
	}

	index, err := normalizeShellHash(ctx, snapshot.files["index.html"], "karty.js", launcherHash)
	if err != nil {
		return "", err
	}
	// All non-level bytes participate, including host/glue/media/debug scripts.
	// Content-addressed KLDs carry both parent levels and subordinate chunks.
	files := map[string][]byte{"game.kart": wasm, "karty.js": launcher, "index.html": index}
	for name, data := range snapshot.files {
		if name == snapshot.ui.Artifact {
			continue
		}

		if _, ok := files[name]; ok {
			continue
		}

		if strings.HasPrefix(name, "content/") && strings.HasSuffix(name, ".kld") {
			continue
		}

		files[name] = data
	}

	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}

	sort.Strings(names)

	hash := sha256.New()

	for _, name := range names {
		checksum, err := digestContext(ctx, files[name])
		if err != nil {
			return "", err
		}

		_, _ = fmt.Fprintf(hash, "%s\x00%s\n", name, checksum)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func normalizeShellHash(ctx context.Context, data []byte, artifact, checksum string) ([]byte, error) {
	// The production shell uses the first eight SHA-256 bytes as its cache key.
	// Recognize only a complete known token, never an arbitrary hex substring.
	for _, token := range []string{checksum, checksum[:shellHashLength]} {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		data = bytes.ReplaceAll(data, []byte(artifact+`?v=`+token+`"`), []byte(artifact+`?v=CONTENT"`))
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return data, nil
}

func normalizeManifest(ctx context.Context, wasm, normalized []byte) ([]byte, error) {
	original, offset, err := manifestSection(ctx, wasm)
	if err != nil {
		return nil, err
	}

	if len(normalized) != len(original) {
		return nil, ErrSnapshot
	}

	result, err := cloneContext(ctx, wasm)
	if err != nil {
		return nil, err
	}

	copy(result[offset:offset+len(normalized)], normalized)

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func manifestSection(ctx context.Context, wasm []byte) ([]byte, int, error) {
	return customSection(ctx, wasm, cartridge.ManifestSectionName)
}

func customSection(ctx context.Context, wasm []byte, wanted string) ([]byte, int, error) {
	if len(wasm) < 8 || !bytes.Equal(wasm[:8], []byte{0, 'a', 's', 'm', 1, 0, 0, 0}) {
		return nil, 0, cartridge.ErrSection
	}

	var bundle []byte

	var contentOffset int

	for offset := 8; offset < len(wasm); {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}

		kind := wasm[offset]
		offset++

		length, count := binary.Uvarint(wasm[offset:])
		if count < 1 || count > 5 || length > maxWASMSectionLength || length > uint64(len(wasm)-offset-count) {
			return nil, 0, cartridge.ErrSection
		}

		offset += count

		end := offset + int(length)
		if kind == 0 {
			nameLength, nameCount := binary.Uvarint(wasm[offset:end])
			if nameCount < 1 || nameCount > 5 || nameLength > maxWASMSectionLength || nameLength > uint64(end-offset-nameCount) {
				return nil, 0, cartridge.ErrSection
			}

			content := offset + nameCount + int(nameLength)
			if string(wasm[offset+nameCount:content]) == wanted {
				if bundle != nil {
					return nil, 0, cartridge.ErrSection
				}

				bundle = wasm[content:end]
				contentOffset = content
			}
		}

		offset = end
	}

	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}

	if bundle == nil {
		return nil, 0, cartridge.ErrSectionMissing
	}

	return bundle, contentOffset, nil
}

func (snapshot *Snapshot) bootstrap() string {
	generation, err := json.Marshal(snapshot.Generation)
	if err != nil {
		return ""
	}

	return `<script>(function(){globalThis.kartyDevelopmentConfig={schema:"karty.dev@1"};const marker="/__karty_dev/generations/";const p=location.pathname;const i=p.indexOf(marker);const root=new URL(i<0?"./":p.slice(0,i+1),location.href);globalThis.kartyDevelopmentBoot={appRoot:root.href,generation:` + string(
		generation,
	) + `};const s=document.createElement("script");s.src=new URL("__karty_dev/browser.js",root);document.head.appendChild(s)})();</script>`
}
