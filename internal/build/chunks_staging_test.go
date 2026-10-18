package build

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty/internal/levelbuild"
)

func fixtureContentDigest(data []byte) string {
	digest := sha256.Sum256(data)

	return hex.EncodeToString(digest[:])
}

func TestChunkStagingPreservesNativeAndWebContentParity(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	parentBytes, childBytes := []byte("parent catalog"), []byte("child render resource")
	parentHash, childHash := fixtureContentDigest(parentBytes), fixtureContentDigest(childBytes)
	artifacts := []levelbuild.Artifact{{Name: "levels.one", Bytes: parentBytes, ContentSHA256: parentHash,
		Chunks: []levelbuild.ChunkArtifact{{ID: 0, Bytes: childBytes, ContentSHA256: childHash}}}}

	for _, target := range []string{"raw", "native", "web"} {
		destination := filepath.Join(root, target)
		if err := os.Mkdir(destination, 0o700); err != nil {
			t.Fatal(err)
		}

		source := filepath.Join(root, "raw")
		if target == "raw" {
			source = ""
		}

		if err := stageLevels(t.Context(), destination, artifacts, source); err != nil {
			t.Fatal(err)
		}

		for _, data := range [][]byte{parentBytes, childBytes} {
			staged, err := os.ReadFile(filepath.Join(destination, "content", fixtureContentDigest(data)+".kld"))
			if err != nil || !bytes.Equal(staged, data) {
				t.Fatalf("%s content parity: %v", target, err)
			}
		}
	}

	if err := os.WriteFile(filepath.Join(root, "raw", "content", childHash+".kld"), []byte("tampered child"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := stageLevels(t.Context(), filepath.Join(root, "native"), artifacts, filepath.Join(root, "raw")); err == nil {
		t.Fatal("staging accepted a changed child")
	}

	preserved, err := os.ReadFile(filepath.Join(root, "native", "content", childHash+".kld"))
	if err != nil || !bytes.Equal(preserved, childBytes) {
		t.Fatalf("failed staging replaced the previous complete distribution: %v", err)
	}
}

func TestChunkStagingRejectsEscapingDigestsAndSymlinks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	data := []byte("chunk")
	if err := stageLevels(
		t.Context(), root, []levelbuild.Artifact{{Name: "root", Bytes: data, ContentSHA256: "../escape"}}, "",
	); err == nil {
		t.Fatal("escaping digest accepted")
	}

	source, destination := filepath.Join(root, "source"), filepath.Join(root, "destination")
	if err := os.MkdirAll(filepath.Join(source, "content"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.Mkdir(destination, 0o700); err != nil {
		t.Fatal(err)
	}

	outside := filepath.Join(root, "outside.kld")
	if err := os.WriteFile(outside, data, 0o600); err != nil {
		t.Fatal(err)
	}

	digest := fixtureContentDigest(data)
	if err := os.Symlink(outside, filepath.Join(source, "content", digest+".kld")); err != nil {
		t.Fatal(err)
	}

	if err := stageLevels(
		t.Context(), destination, []levelbuild.Artifact{{Name: "root", Bytes: data, ContentSHA256: digest}}, source,
	); err == nil {
		t.Fatal("staging followed a content symlink outside its root")
	}
}
