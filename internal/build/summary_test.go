package build

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/levelbuild"
	"github.com/karty-game/karty/internal/release"
)

func TestSummaryCountsStagedTargetAndLogicalChunks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	raw := filepath.Join(root, "dist", "raw")

	if err := os.MkdirAll(raw, 0o750); err != nil {
		t.Fatal(err)
	}

	game := []byte("\x00asm\x01\x00\x00\x00")
	gamePath := filepath.Join(raw, "game.kart")

	writeSummaryFixture(t, gamePath, game)

	shared, other := []byte("shared chunk"), []byte("other chunk")

	levels := []levelbuild.Artifact{
		{
			Name:          "first",
			Bytes:         []byte("first level"),
			ContentSHA256: summaryDigest([]byte("first level")),
			Chunks: []levelbuild.ChunkArtifact{
				{ID: 1, Bytes: shared, ContentSHA256: summaryDigest(shared)},
				{ID: 2, Bytes: other, ContentSHA256: summaryDigest(other)},
			},
		},
		{
			Name:          "second",
			Bytes:         []byte("second level"),
			ContentSHA256: summaryDigest([]byte("second level")),
			Chunks: []levelbuild.ChunkArtifact{
				{ID: 1, Bytes: shared, ContentSHA256: summaryDigest(shared)},
			},
		},
	}
	if err := stageLevels(t.Context(), raw, levels, ""); err != nil {
		t.Fatal(err)
	}

	hostPath := filepath.Join(root, "host")

	for _, platform := range []string{"linux-arm64", "darwin-arm64"} {
		writeSummaryFixture(t, hostPath, nativeHeader(t, platform))

		if err := stageTarget(t.Context(),
			filepath.Join(root, "dist"),
			raw,
			gamePath,
			levels,
			hostPath,
			"native",
			"",
			platform,
			"test",
			false,
		); err != nil {
			t.Fatal(err)
		}
	}

	// Other targets and the raw asset report must not inflate this platform's size.
	writeSummaryFixture(t, filepath.Join(raw, "asset-report.toml"), []byte("raw asset report"))

	web := filepath.Join(root, "dist", "web")
	if err := os.MkdirAll(web, 0o750); err != nil {
		t.Fatal(err)
	}

	writeSummaryFixture(t, filepath.Join(web, "unrelated"), make([]byte, 4096))

	assets := assetpipeline.Report{
		Textures: []assetpipeline.Texture{{}}, Levels: []assetpipeline.LevelTexture{{}, {}},
		Sounds: []assetpipeline.Sound{{}}, AudioStreams: []assetpipeline.AudioStream{{}, {}},
	}

	report, err := summarizeBuild(
		t.Context(), root, "test", release.SDKVersion(), levels, assets, Options{Target: "native", Platform: "darwin-arm64"},
	)
	if err != nil {
		t.Fatal(err)
	}

	// Shared chunks have two logical references and one content-addressed file.
	wantBytes := int64(
		len(game) + len(nativeHeader(t, "darwin-arm64")) + len("first level") + len("second level") + len(shared) + len(other),
	)
	if report.Levels != 2 || report.Chunks != 3 || report.Files != 6 || report.Bytes != wantBytes ||
		report.CartridgeBytes != int64(len(game)) {
		t.Fatalf("staged target summary = %+v, want 2 levels, 3 chunks, 6 files, %d bytes", report, wantBytes)
	}

	if report.Textures != 3 || report.Sounds != 1 || report.Streams != 2 ||
		report.Directory != filepath.Join(root, "dist", "native", "darwin-arm64") {
		t.Fatalf("asset or output summary = %+v", report)
	}

	// A default native build must also exclude older platform subdirectories.
	defaultHost := nativeHeader(t, runtime.GOOS+"-"+runtime.GOARCH)
	writeSummaryFixture(t, hostPath, defaultHost)

	if err := stageTarget(t.Context(),
		filepath.Join(root, "dist"),
		raw,
		gamePath,
		levels,
		hostPath,
		"native",
		"",
		"",
		"test",
		false,
	); err != nil {
		t.Fatal(err)
	}

	notes := filepath.Join(root, "dist", "native", "notes")
	if err := os.MkdirAll(notes, 0o750); err != nil {
		t.Fatal(err)
	}

	writeSummaryFixture(t, filepath.Join(notes, "README"), []byte("notes"))

	defaultReport, err := summarizeBuild(t.Context(), root, "test", release.SDKVersion(), levels, assets, Options{Host: hostPath})
	if err != nil {
		t.Fatal(err)
	}

	wantBytes += int64(len(defaultHost) - len(nativeHeader(t, "darwin-arm64")) + len("notes"))
	if defaultReport.Files != 7 || defaultReport.Bytes != wantBytes || defaultReport.Target != "native" {
		t.Fatalf("default native summary = %+v, want 7 files, %d bytes", defaultReport, wantBytes)
	}
}

func TestSummaryRejectsEscapingOutputAndNonRegularEntries(t *testing.T) {
	t.Parallel()

	for _, symlinkTarget := range []bool{false, true} {
		t.Run(map[bool]string{false: "entry", true: "target"}[symlinkTarget], func(t *testing.T) {
			t.Parallel()

			root, outside := t.TempDir(), t.TempDir()
			output := filepath.Join(root, "dist", "web")

			if err := os.MkdirAll(filepath.Dir(output), 0o750); err != nil {
				t.Fatal(err)
			}

			if symlinkTarget {
				writeSummaryFixture(t, filepath.Join(outside, "game.kart"), []byte("outside cartridge"))

				if err := os.Symlink(outside, output); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.MkdirAll(output, 0o750); err != nil {
					t.Fatal(err)
				}

				writeSummaryFixture(t, filepath.Join(output, "game.kart"), []byte("cartridge"))

				if err := os.Symlink(outside, filepath.Join(output, "escaped")); err != nil {
					t.Fatal(err)
				}
			}

			if _, err := summarizeBuild(t.Context(),
				root,
				"test",
				release.SDKVersion(),
				nil,
				assetpipeline.Report{},
				Options{Target: "web"},
			); err == nil {
				t.Fatal("summary accepted an escaping output symlink")
			}
		})
	}
}

func TestSummaryDoesNotCompleteFailedBuild(t *testing.T) {
	t.Parallel()

	completed := false

	err := RunWithOptions(t.Context(), t.TempDir(), Options{
		Target: "web", Platform: "linux-arm64", Summary: func(Summary) { completed = true },
	})
	if err == nil || completed {
		t.Fatalf("failed build = %v, summary callback = %v", err, completed)
	}
}

func writeSummaryFixture(t *testing.T, path string, contents []byte) {
	t.Helper()

	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
}

func summaryDigest(contents []byte) string {
	digest := sha256.Sum256(contents)

	return hex.EncodeToString(digest[:])
}
