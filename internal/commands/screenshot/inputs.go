package screenshot

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/karty-game/karty/internal/levelbuild"
)

// StageCaptureInputs stages only the verified root and its immutable children.
// Both screenshot and reflection hosts use the ordinary confined content path.
func StageCaptureInputs(directory string, artifact levelbuild.Artifact) (string, error) {
	files := map[string][]byte{artifact.ContentSHA256: artifact.Bytes}
	for _, child := range artifact.Chunks {
		files[child.ContentSHA256] = child.Bytes
	}

	const maximum = 32 << 20

	for digest, data := range files {
		decoded, err := hex.DecodeString(digest)
		if err != nil || len(decoded) != sha256.Size || digest != strings.ToLower(digest) || len(data) == 0 || len(data) > maximum ||
			fmt.Sprintf("%x", sha256.Sum256(data)) != digest {
			return "", fmt.Errorf("invalid capture input identity: %w", os.ErrInvalid)
		}
	}

	content := filepath.Join(directory, "content")
	if err := os.Mkdir(content, 0o700); err != nil {
		return "", err
	}

	for digest, data := range files {
		if err := os.WriteFile(filepath.Join(content, digest+".kld"), data, 0o600); err != nil {
			return "", err
		}
	}

	return filepath.Join(content, artifact.ContentSHA256+".kld"), nil
}
