package sdk

// VerifyPublishedManifest authenticates local release metadata through the same
// pinned verifier and manifest validation used by ordinary SDK installation.
// It does not download, install or publish anything.
func VerifyPublishedManifest(version string, contents, signature []byte) (Manifest, error) {
	if err := validateVersion(version); err != nil {
		return Manifest{}, err
	}

	key, err := publishedVerifier()
	if err != nil {
		return Manifest{}, err
	}

	return parsePublishedManifest(version, contents, signature, key)
}
