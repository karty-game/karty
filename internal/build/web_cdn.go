package build

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/karty-game/karty/internal/sdk"
)

type cdnHost struct {
	URL, SHA256 string
	Manifest    sdk.Manifest
}

func validateCDNURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl) || strings.Contains(value, "\\") {
		return fmt.Errorf("invalid --cdn host URL: %w", os.ErrInvalid)
	}

	if parsed.User != nil || parsed.Fragment != "" || parsed.Path == "" {
		return fmt.Errorf("--cdn requires an HTTPS URL or relative host path without credentials or a fragment: %w", os.ErrInvalid)
	}

	if parsed.IsAbs() {
		if parsed.Scheme != "https" || parsed.Host == "" {
			return fmt.Errorf("absolute --cdn URLs must use HTTPS: %w", os.ErrInvalid)
		}

		hostname := strings.ToLower(parsed.Hostname())
		if hostname == "github.com" && strings.Contains(parsed.Path, "/releases/download/") ||
			hostname == "api.github.com" && strings.Contains(parsed.Path, "/releases/assets/") ||
			hostname == "release-assets.githubusercontent.com" {
			return fmt.Errorf(
				"GitHub release downloads block browser CORS; --cdn needs an HTTPS mirror of the release host: %w",
				os.ErrInvalid,
			)
		}

		return nil
	}

	// Relative mirrors may sit alongside sample directories, one parent above.
	// Reject implicit cross-origin URLs and ambiguous or escaping path forms.
	encodedPath := strings.ToLower(parsed.RawPath)
	if parsed.Host != "" || path.Clean(parsed.Path) != parsed.Path || parsed.Path == "." || parsed.Path == ".." ||
		strings.HasPrefix(parsed.Path, "../../") || strings.Contains(encodedPath, "%2e") || strings.Contains(encodedPath, "%2f") ||
		strings.Contains(encodedPath, "%5c") {
		return fmt.Errorf("relative --cdn paths must stay in this directory or one parent directory: %w", os.ErrInvalid)
	}

	return nil
}

func resolveCDNHost(ctx context.Context, selected sdk.Manifest, hostURL, localHost string) (*cdnHost, error) {
	// Use signed release metadata even when the installed SDK was built locally.
	published, err := sdk.ResolvePublished(ctx, selected.Version)
	if err != nil {
		return nil, fmt.Errorf("--cdn requires a signed SDK release for %s: %w", selected.Version, err)
	}

	if published.Host.Version != selected.Host.Version {
		return nil, fmt.Errorf("--cdn release host version differs from the project SDK: %w", os.ErrInvalid)
	}

	return cdnHostFromManifest(published, hostURL, localHost)
}

func cdnHostFromManifest(manifest sdk.Manifest, hostURL, localHost string) (*cdnHost, error) {
	artifact := manifest.Artifacts.Host.Web

	digest, err := hex.DecodeString(artifact.SHA256)
	if err != nil || len(digest) != sha256.Size || artifact.SHA256 != strings.ToLower(artifact.SHA256) ||
		artifact.URL != "https://github.com/karty-game/karty-sdk/releases/download/sdk-v"+manifest.Version+"/karty-host-web.wasm" ||
		manifest.Artifacts.Host.WebRuntime.URL == "" || manifest.Artifacts.Host.WebRuntime.SHA256 == "" {
		return nil, fmt.Errorf("SDK %s lacks pinned web host and runtime release metadata: %w", manifest.Version, os.ErrInvalid)
	}

	if localHost != "" {
		if err := checkCDNOverride(localHost, artifact.SHA256); err != nil {
			return nil, err
		}
	}

	return &cdnHost{URL: hostURL, SHA256: artifact.SHA256, Manifest: manifest}, nil
}

func checkCDNOverride(path, checksum string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("inspect --cdn host override: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("inspect --cdn host override: %w", err)
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("--cdn host override must be a regular file: %w", os.ErrInvalid)
	}

	digest := sha256.New()

	if _, err := io.Copy(digest, file); err != nil {
		return fmt.Errorf("hash --cdn host override: %w", err)
	}

	if hex.EncodeToString(digest.Sum(nil)) != checksum {
		return fmt.Errorf("--host differs from the pinned release; omit --cdn to package this custom host: %w", os.ErrInvalid)
	}

	return nil
}

func webHostSource(directory string, cdn *cdnHost) (string, error) {
	var sourceURL, checksum string

	if cdn != nil {
		sourceURL, checksum = cdn.URL, cdn.SHA256
	} else {
		hash, err := fileHash(filepath.Join(directory, "karty-host.wasm"))
		if err != nil {
			return "", fmt.Errorf("hash staged host: %w", err)
		}

		sourceURL = "karty-host.wasm?v=" + hash
	}

	return "{url:" + javascriptString(sourceURL) + ",sha256:" + javascriptString(checksum) + "}", nil
}

func javascriptString(value string) string {
	var quoted strings.Builder

	quoted.WriteByte('"')

	for _, character := range value {
		switch {
		case character == '"' || character == '\\':
			quoted.WriteByte('\\')
			quoted.WriteRune(character)
		case character == '<' || character == '\u2028' || character == '\u2029' || unicode.IsControl(character):
			fmt.Fprintf(&quoted, "\\u%04x", character)
		default:
			quoted.WriteRune(character)
		}
	}

	quoted.WriteByte('"')

	return quoted.String()
}
