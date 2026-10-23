package packages

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	remoteArchiveTimeout = 30 * time.Second
	maxArchiveRedirects  = 5
)

func validRemoteArchiveSource(source string) bool {
	parsed, err := url.Parse(source)
	if err != nil || !safeArchiveURL(parsed) || parsed.RawQuery != "" || parsed.ForceQuery || strings.Contains(source, "#") ||
		strings.Contains(parsed.EscapedPath(), "%") || !strings.HasSuffix(parsed.Path, ".kpkg") {
		return false
	}

	return validPath(strings.TrimPrefix(parsed.Path, "/"))
}

func safeArchiveURL(parsed *url.URL) bool {
	return parsed != nil && parsed.Scheme == "https" && parsed.Hostname() != "" && parsed.User == nil &&
		parsed.Fragment == "" && parsed.RawFragment == "" && parsed.Opaque == ""
}

func remoteArchiveClient(client *http.Client) *http.Client {
	result := http.Client{}
	if client != nil {
		result = *client
	}

	if result.Timeout <= 0 || result.Timeout > remoteArchiveTimeout {
		result.Timeout = remoteArchiveTimeout
	}

	result.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		// GitHub release assets redirect to HTTPS CDN URLs with signed queries.
		if len(via) >= maxArchiveRedirects || !safeArchiveURL(request.URL) {
			return fmt.Errorf("%w: unsafe or excessive archive redirects", ErrPackage)
		}

		return nil
	}

	return &result
}

func remoteArchiveCachePath(digest string) string {
	return ".karty/packages/.downloads/" + digest + ".kpkg"
}

func readArchiveSource(ctx context.Context, root *os.Root, source Source, client *http.Client) ([]byte, error) {
	if !validRemoteArchiveSource(source.Archive) {
		return Read(ctx, root, source.Archive, MaxArchiveBytes)
	}

	cache := remoteArchiveCachePath(source.SHA256)

	info, err := root.Lstat(cache)
	if err == nil {
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%w: nonregular archive cache", ErrPackage)
		}

		data, err := Read(ctx, root, cache, MaxArchiveBytes)
		if err != nil {
			return nil, err
		}

		if Digest(data) != source.SHA256 {
			return nil, fmt.Errorf("%w: cached archive checksum mismatch", ErrPackage)
		}

		return data, nil
	}

	if !os.IsNotExist(err) {
		return nil, err
	}

	return downloadRemoteArchive(ctx, client, source.Archive, MaxArchiveBytes)
}

func downloadRemoteArchive(ctx context.Context, client *http.Client, source string, maximum int64) ([]byte, error) {
	if maximum < 1 || maximum > MaxArchiveBytes {
		return nil, ErrPackage
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, err
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("download package archive: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK || response.ContentLength > maximum {
		return nil, fmt.Errorf("%w: archive response %s exceeds bounds or is unsuccessful", ErrPackage, response.Status)
	}

	return readBounded(ctx, response.Body, maximum)
}
