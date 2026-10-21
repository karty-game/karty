package website

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

const defaultSiteURL = "https://karty-game.github.io/karty/main/"

// Social holds text and absolute URLs for a published page's link preview.
type Social struct {
	Title, Description, URL, ImageURL string
}

// NewSocial uses KARTY_SITE_URL for custom domains and branch previews.
func NewSocial(title, description, page string) (Social, error) {
	base := os.Getenv("KARTY_SITE_URL")
	if base == "" {
		base = defaultSiteURL
	}

	return socialForBase(base, title, description, page)
}

func socialForBase(base, title, description, page string) (Social, error) {
	root, err := url.Parse(base)
	if err != nil || root == nil || (root.Scheme != "https" && root.Scheme != "http") ||
		root.Hostname() == "" || root.User != nil || root.ForceQuery || root.RawQuery != "" || root.Fragment != "" {
		return Social{}, fmt.Errorf(
			"KARTY_SITE_URL must be an absolute HTTP(S) base URL without credentials, query or fragment: %w",
			os.ErrInvalid,
		)
	}

	reference, err := url.Parse(page)
	if err != nil || reference.IsAbs() || reference.Host != "" || reference.ForceQuery || reference.RawQuery != "" ||
		reference.Fragment != "" || strings.HasPrefix(reference.Path, "/") || strings.Contains(reference.Path, "\\") {
		return Social{}, fmt.Errorf("social page must be a relative path %q: %w", page, os.ErrInvalid)
	}

	for part := range strings.SplitSeq(reference.Path, "/") {
		if part == "." || part == ".." {
			return Social{}, fmt.Errorf("social page must stay beneath the site base %q: %w", page, os.ErrInvalid)
		}
	}

	root.Path = strings.TrimRight(root.Path, "/") + "/"
	root.RawPath = ""

	return Social{
		Title: title, Description: description,
		URL:      root.ResolveReference(reference).String(),
		ImageURL: root.ResolveReference(&url.URL{Path: "guide/assets/banner.png"}).String(),
	}, nil
}
