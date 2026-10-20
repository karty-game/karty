package devserver

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

type watchNotificationBackend interface {
	Add(name string) error
	Remove(name string) error
	WatchList() []string
	Close() error
}

// Notifications are hints: Scan still performs every confined read and a full
// periodic verification recovers from missed filesystem events.
type watchNotifications struct {
	watchNotificationBackend

	events     <-chan fsnotify.Event
	errors     <-chan error
	roots      []string
	identities map[string]fs.FileInfo
}

func newWatchNotifications() (*watchNotifications, error) {
	backend, err := fsnotify.NewBufferedWatcher(4096)
	if err != nil {
		return nil, err
	}

	return &watchNotifications{watchNotificationBackend: backend, events: backend.Events, errors: backend.Errors}, nil
}

func (watcher *Watcher) notificationPaths(directories map[string]bool) []string {
	watcher.mutex.Lock()
	defer watcher.mutex.Unlock()

	for directory := range watcher.directories {
		directories[directory] = true
	}

	// Watching each root's parent catches atomic directory replacement. Events
	// from unrelated siblings are filtered by relevant below.
	directories[filepath.Dir(watcher.directory)] = true
	roots := make([]string, 1, len(watcher.children)+1)

	roots[0] = watcher.directory
	for _, child := range watcher.children {
		roots = append(roots, child.notificationPaths(directories)...)
	}

	return roots
}

func (notifications *watchNotifications) sync(ctx context.Context, watcher *Watcher) (bool, error) {
	directories := make(map[string]bool)

	notifications.roots = watcher.notificationPaths(directories)
	if len(directories) > maxWatchFiles {
		return false, errWatchBounds
	}

	changed := false

	if notifications.identities == nil {
		notifications.identities = make(map[string]fs.FileInfo)
	}

	for _, directory := range notifications.WatchList() {
		if err := ctx.Err(); err != nil {
			return false, err
		}

		current, err := os.Stat(directory)

		previous := notifications.identities[directory]
		if directories[directory] && err == nil && previous != nil && os.SameFile(previous, current) {
			delete(directories, directory)
		} else {
			_ = notifications.Remove(directory)
			delete(notifications.identities, directory)

			changed = true
		}
	}

	for directory := range directories {
		if err := ctx.Err(); err != nil {
			return false, err
		}

		info, err := os.Stat(directory)
		if err != nil {
			return false, err
		}

		if err := notifications.Add(directory); err != nil {
			return false, err
		}

		changed = true
		notifications.identities[directory] = info
	}

	// New registrations leave a gap after the scan. Scan again at the next
	// poll to detect writes that happened before those watches were installed.
	return changed, nil
}

func (notifications *watchNotifications) relevant(name string) bool {
	for _, root := range notifications.roots {
		relative, err := filepath.Rel(root, name)
		if err != nil || !filepath.IsLocal(relative) {
			// A watched parent can itself be renamed or removed. Its event
			// affects every source root below it; siblings fail this check.
			descendant, ancestorErr := filepath.Rel(name, root)
			if ancestorErr == nil && filepath.IsLocal(descendant) {
				return true
			}

			continue
		}

		excluded := false

		for part := range strings.SplitSeq(relative, string(filepath.Separator)) {
			if excludedWatchDirectory(part) || excludedWatchFile(part) {
				excluded = true

				break
			}
		}

		if !excluded {
			return true
		}
	}

	return false
}

func waitWatchPoll(ctx context.Context, tick <-chan time.Time, notifications **watchNotifications, dirty *bool) bool {
	for {
		var (
			events   <-chan fsnotify.Event
			failures <-chan error
		)
		if *notifications != nil {
			events, failures = (*notifications).events, (*notifications).errors
		}

		select {
		case <-ctx.Done():
			return false
		case <-tick:
			return true
		case event, open := <-events:
			if !open {
				_ = (*notifications).Close()
				*notifications = nil
			} else if (*notifications).relevant(event.Name) {
				*dirty = true
			}
		case err, open := <-failures:
			if open && err == nil {
				continue
			}

			// Overflow and backend failure both require complete polling. Do not
			// silently trust a partially functioning notification stream.
			_ = (*notifications).Close()
			*notifications = nil
		}
	}
}
