package devserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	maxWatchFiles        = 100000
	maxPackageWatchRoots = 16
	maxWatchDepth        = 64
	maxWatchFileBytes    = 256 * 1024 * 1024
	maxWatchBytes        = int64(4 * 1024 * 1024 * 1024)
	maxWatchHashAge      = 10 * time.Second
)

var errWatchBounds = errors.New("project watch inputs exceed development limits")

type watchedFile struct {
	info     fs.FileInfo
	digest   [sha256.Size]byte
	hashedAt time.Time
}

// Watcher detects content changes, including additions and deletion. Metadata
// avoids rereading unchanged large assets on each poll. Hashes expire so edits
// that preserve inode/size/mtime are still detected within the verification age.
// os.Root confines every read even if a symlink changes during a scan.
type Watcher struct {
	mutex                sync.Mutex
	root                 *os.Root
	directory            string
	files                map[string]watchedFile
	now                  func() time.Time
	additionalRoots      func(context.Context) (map[string]string, error)
	children             map[string]*Watcher
	lastBytes            int64
	directories          map[string]bool
	newNotifications     func() (*watchNotifications, error)
	verificationInterval time.Duration
}

func NewWatcher(directory string) (*Watcher, error) {
	absolute, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return nil, err
	}

	absolute, err = filepath.Abs(absolute)
	if err != nil {
		return nil, err
	}

	root, err := os.OpenRoot(absolute)
	if err != nil {
		return nil, err
	}

	return &Watcher{root: root, directory: absolute, files: make(map[string]watchedFile), now: time.Now}, nil
}

func (watcher *Watcher) Close() error {
	for _, child := range watcher.children {
		_ = child.Close()
	}

	return watcher.root.Close()
}

func excludedWatchDirectory(name string) bool {
	switch name {
	case ".git", ".karty", "dist", ".codex", ".agents", "node_modules", ".idea", ".vscode":
		return true
	default:
		return false
	}
}

func excludedWatchFile(name string) bool {
	return name == "api_codegen.go" || name == ".DS_Store" || strings.HasSuffix(name, "~") ||
		strings.HasSuffix(name, ".swp") || strings.HasSuffix(name, ".tmp") || strings.HasSuffix(name, ".log")
}

type sourceChange struct {
	digest string
	err    error
}

type watchScan struct {
	watcher     *Watcher
	files       map[string]watchedFile
	bytes       int64
	now         time.Time
	buffer      []byte
	directories map[string]bool
	forceHash   bool
}

func (watcher *Watcher) Scan(ctx context.Context) (string, error) {
	return watcher.scan(ctx, false)
}

func (watcher *Watcher) scan(ctx context.Context, forceHash bool) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	watcher.mutex.Lock()
	defer watcher.mutex.Unlock()

	scan := watchScan{
		watcher:     watcher,
		files:       make(map[string]watchedFile),
		now:         watcher.now(),
		directories: make(map[string]bool),
		forceHash:   forceHash,
	}
	if err := scan.walk(ctx, ".", 0, make(map[string]bool)); err != nil {
		return "", err
	}

	names := make([]string, 0, len(scan.files))
	for name := range scan.files {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		names = append(names, name)
	}

	slices.Sort(names)

	hash := sha256.New()

	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		_, _ = io.WriteString(hash, name+"\x00")
		file := scan.files[name]
		_, _ = hash.Write(file.digest[:])
	}

	if watcher.additionalRoots != nil {
		if err := watcher.scanAdditional(ctx, hash, scan.bytes, len(scan.files), forceHash); err != nil {
			return "", err
		}
	}

	watcher.files = scan.files
	watcher.lastBytes = scan.bytes
	watcher.directories = scan.directories

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (watcher *Watcher) scanAdditional(ctx context.Context, hash io.Writer, total int64, count int, forceHash bool) error {
	roots, err := watcher.additionalRoots(ctx)
	if err != nil {
		return err
	}

	if len(roots) > maxPackageWatchRoots {
		return errWatchBounds
	}

	if watcher.children == nil {
		watcher.children = map[string]*Watcher{}
	}

	aliases := make([]string, 0, len(roots))
	for alias := range roots {
		aliases = append(aliases, alias)
	}

	slices.Sort(aliases)

	for alias, child := range watcher.children {
		if _, exists := roots[alias]; !exists {
			_ = child.Close()

			delete(watcher.children, alias)
		}
	}

	for _, alias := range aliases {
		directory, err := filepath.EvalSymlinks(roots[alias])
		if err != nil {
			return err
		}

		directory, err = filepath.Abs(directory)
		if err != nil {
			return err
		}

		child := watcher.children[alias]

		reopen := child == nil || child.directory != directory
		if !reopen {
			opened, openErr := child.root.Stat(".")
			current, currentErr := os.Stat(directory)
			reopen = openErr != nil || currentErr != nil || !os.SameFile(opened, current)
		}

		if reopen {
			fresh, err := NewWatcher(directory)
			if err != nil {
				return err
			}

			if child != nil {
				_ = child.Close()
			}

			child = fresh
			watcher.children[alias] = child
		}

		digest, err := child.scan(ctx, forceHash)
		if err != nil {
			return fmt.Errorf("watch package %s: %w", alias, err)
		}

		total += child.lastBytes

		count += len(child.files)
		if total > maxWatchBytes || count > maxWatchFiles {
			return errWatchBounds
		}

		_, _ = fmt.Fprintf(hash, "\x00package:%s\x00%s\x00%s", alias, directory, digest)
	}

	return nil
}

func (scan *watchScan) walk(ctx context.Context, relative string, depth int, ancestors map[string]bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if depth > maxWatchDepth {
		return errWatchBounds
	}

	resolved, excluded, err := scan.resolve(relative)
	if err != nil {
		return err
	}

	if excluded {
		return nil
	}

	if ancestors[resolved] {
		return fmt.Errorf("watch directory cycle at %q: %w", relative, os.ErrInvalid)
	}

	ancestors[resolved] = true

	scan.directories[resolved] = true
	defer delete(ancestors, resolved)

	entries, err := fs.ReadDir(scan.watcher.root.FS(), relative)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if err := scan.entry(ctx, relative, entry, depth, ancestors); err != nil {
			return err
		}
	}

	return nil
}

func (scan *watchScan) entry(ctx context.Context, relative string, entry fs.DirEntry, depth int, ancestors map[string]bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	name := path.Join(relative, entry.Name())
	if excludedWatchDirectory(entry.Name()) || excludedWatchFile(entry.Name()) {
		return nil
	}

	if entry.Type()&os.ModeSymlink != 0 {
		_, excluded, err := scan.resolve(name)
		if err != nil {
			return err
		}

		if excluded {
			return nil
		}
	}

	info, err := fs.Stat(scan.watcher.root.FS(), name)
	if err != nil {
		return fmt.Errorf("watch %s: %w", name, err)
	}

	if info.IsDir() {
		return scan.walk(ctx, name, depth+1, ancestors)
	}

	if !info.Mode().IsRegular() {
		return nil
	}

	if err := scan.read(ctx, name, info); err != nil {
		return fmt.Errorf("watch %s: %w", name, err)
	}

	return nil
}

func (scan *watchScan) resolve(relative string) (string, bool, error) {
	// Resolve for loop/exclusion detection. Rooted opens independently confine
	// every read even when a link changes between resolution and opening.
	resolved, err := filepath.EvalSymlinks(filepath.Join(scan.watcher.directory, filepath.FromSlash(relative)))
	if err != nil {
		return "", false, err
	}

	confined, err := filepath.Rel(scan.watcher.directory, resolved)
	if err != nil || confined == ".." || strings.HasPrefix(confined, ".."+string(filepath.Separator)) {
		return "", false, fmt.Errorf("watch path %q escapes the project: %w", relative, os.ErrPermission)
	}

	excluded := slices.ContainsFunc(strings.Split(confined, string(filepath.Separator)), excludedWatchDirectory) ||
		excludedWatchFile(filepath.Base(confined))

	return resolved, excluded, nil
}

func (scan *watchScan) read(ctx context.Context, name string, info fs.FileInfo) error {
	if info.Size() > maxWatchFileBytes || len(scan.files) >= maxWatchFiles || scan.bytes+info.Size() > maxWatchBytes {
		return errWatchBounds
	}

	scan.bytes += info.Size()

	cached, exists := scan.watcher.files[name]
	if scan.forceHash || !exists || !sameWatchFile(cached.info, info) || scan.now.Sub(cached.hashedAt) >= maxWatchHashAge {
		var err error

		cached, err = scan.hashFile(ctx, name, info)
		if err != nil {
			return err
		}
	}

	scan.files[name] = cached

	return nil
}

func sameWatchFile(first, second fs.FileInfo) bool {
	return os.SameFile(first, second) && first.Size() == second.Size() &&
		first.ModTime().Equal(second.ModTime()) && first.Mode() == second.Mode()
}

func (scan *watchScan) hashFile(ctx context.Context, name string, expected fs.FileInfo) (watchedFile, error) {
	file, err := scan.watcher.root.Open(name)
	if err != nil {
		return watchedFile{}, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return watchedFile{}, err
	}

	if !info.Mode().IsRegular() || !sameWatchFile(info, expected) {
		return watchedFile{}, fmt.Errorf("source changed while scanning: %w", os.ErrInvalid)
	}

	if scan.buffer == nil {
		scan.buffer = make([]byte, contextChunkBytes)
	}

	digest, length, err := hashWatchContentsBuffer(ctx, io.LimitReader(file, maxWatchFileBytes+1), scan.buffer)
	if err != nil {
		return watchedFile{}, err
	}

	after, err := file.Stat()
	if err != nil {
		return watchedFile{}, err
	}

	if length != expected.Size() || !sameWatchFile(after, expected) {
		return watchedFile{}, fmt.Errorf("source changed while scanning: %w", os.ErrInvalid)
	}

	return watchedFile{info: after, digest: digest, hashedAt: scan.now}, nil
}

func hashWatchContents(ctx context.Context, reader io.Reader) ([sha256.Size]byte, int64, error) {
	return hashWatchContentsBuffer(ctx, reader, make([]byte, contextChunkBytes))
}

func hashWatchContentsBuffer(ctx context.Context, reader io.Reader, buffer []byte) ([sha256.Size]byte, int64, error) {
	hash := sha256.New()

	var length int64

	for {
		if err := ctx.Err(); err != nil {
			return [sha256.Size]byte{}, length, err
		}

		count, err := reader.Read(buffer)

		if canceled := ctx.Err(); canceled != nil {
			return [sha256.Size]byte{}, length, canceled
		}

		_, _ = hash.Write(buffer[:count])
		length += int64(count)

		if errors.Is(err, io.EOF) {
			var digest [sha256.Size]byte
			copy(digest[:], hash.Sum(nil))

			return digest, length, nil
		}

		if err != nil {
			return [sha256.Size]byte{}, length, err
		}
	}
}

func (watcher *Watcher) changes(ctx context.Context, interval time.Duration, output chan sourceChange) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	watcher.changesWithTicks(ctx, ticker.C, output)
}

func (watcher *Watcher) changesWithTicks(ctx context.Context, ticks <-chan time.Time, output chan sourceChange) {
	defer close(output)

	previous := ""

	openNotifications := watcher.newNotifications
	if openNotifications == nil {
		openNotifications = newWatchNotifications
	}

	notifications, _ := openNotifications()
	defer func() {
		if notifications != nil {
			_ = notifications.Close()
		}
	}()

	dirty := true

	verificationInterval := watcher.verificationInterval
	if verificationInterval <= 0 {
		verificationInterval = maxWatchHashAge
	}

	var verified time.Time

	for {
		forceHash := time.Since(verified) >= verificationInterval
		if dirty || notifications == nil || forceHash {
			scanStarted := time.Now()

			change, retry := watcher.scanChange(ctx, &notifications, forceHash)
			if ctx.Err() != nil {
				return
			}

			dirty = retry

			if change.err == nil && forceHash {
				verified = scanStarted
			}

			key := change.digest
			if change.err != nil {
				key = "error:" + change.err.Error()
			}

			if key != previous && !publishSourceChange(ctx, output, change) {
				return
			}

			previous = key
		}

		if !waitWatchPoll(ctx, ticks, &notifications, &dirty) {
			return
		}
	}
}

func (watcher *Watcher) scanChange(ctx context.Context, notifications **watchNotifications, forceHash bool) (sourceChange, bool) {
	digest, err := watcher.scan(ctx, forceHash)

	change := sourceChange{digest: digest, err: err}
	if err != nil {
		return change, true
	}

	if *notifications == nil {
		return change, false
	}

	dirty, err := (*notifications).sync(ctx, watcher)
	if err != nil {
		_ = (*notifications).Close()
		*notifications = nil

		return change, true
	}

	return change, dirty
}

func publishSourceChange(ctx context.Context, output chan sourceChange, change sourceChange) bool {
	// Only the newest source state matters; the build owner cancels and joins
	// obsolete work before starting another generation.
	select {
	case <-output:
	default:
	}

	select {
	case output <- change:
		return true
	case <-ctx.Done():
		return false
	}
}
