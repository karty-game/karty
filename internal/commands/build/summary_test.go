package build

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	buildservice "github.com/karty-game/karty/internal/build"
	"github.com/karty-game/karty/internal/release"
)

func TestSummaryShowsOutputAndTargetNextSteps(t *testing.T) {
	t.Parallel()

	for _, example := range []struct {
		name, target, platform, directory string
		want                              []string
	}{
		{name: "web", target: "web", directory: "dist/web", want: []string{"Next: karty serve", "http://127.0.0.1:8080/", "static web host"}},
		{name: "unix", target: "native", platform: "linux-arm64", directory: "my game's/dist/native/linux-arm64", want: []string{
			"Next on linux-arm64:", "'./my game'\\''s/dist/native/linux-arm64/karty-host'", "--cartridge 'my game'\\''s/dist/native/linux-arm64/game.kart'",
		}},
		{name: "windows", target: "native", platform: "windows-arm64", directory: "my game's/dist/native/windows-arm64", want: []string{
			"Next on windows-arm64 in PowerShell:", "& './my game''s/dist/native/windows-arm64/karty-host.exe'", "--cartridge 'my game''s/dist/native/windows-arm64/game.kart'",
		}},
		{name: "raw", target: "raw", directory: "dist/raw", want: []string{"karty dev", "karty build --target native"}},
	} {
		t.Run(example.name, func(t *testing.T) {
			t.Parallel()

			var output bytes.Buffer

			report := buildservice.Summary{
				Project:        "starter",
				SDK:            release.SDKVersion(),
				Target:         example.target,
				Platform:       example.platform,
				Directory:      filepath.FromSlash(example.directory),
				Levels:         2,
				Chunks:         7,
				Files:          12,
				Textures:       4,
				Sounds:         1,
				Streams:        2,
				Bytes:          2048,
				CartridgeBytes: 512,
			}
			if err := writeSummary(&output, report); err != nil {
				t.Fatal(err)
			}

			for _, expected := range append(example.want, "using SDK "+release.SDKVersion(), "Levels: 2 | Chunks: 7", "Textures: 4 | Sounds: 1 | Audio streams: 2", "12 files, 2.0 KiB", "Game cartridge: 512 B") {
				if !strings.Contains(output.String(), expected) {
					t.Errorf("summary missing %q:\n%s", expected, &output)
				}
			}
		})
	}
}

func TestSummaryReturnsOutputError(t *testing.T) {
	t.Parallel()

	if err := writeSummary(failedSummaryWriter{}, buildservice.Summary{}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("summary output error = %v", err)
	}
}

func TestCDNSummaryReportsRemoteDependencyAndRelativePreviewRoot(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer

	report := buildservice.Summary{
		Project:   "starter",
		SDK:       release.SDKVersion(),
		Target:    "web",
		Directory: "dist/web",
		HostURL:   "../runtime/" + release.SDKVersion() + "/release.wasm",
		Files:     4,
		Bytes:     2048,
	}
	if err := writeSummary(&output, report); err != nil {
		t.Fatal(err)
	}

	for _, expected := range []string{"4 files, 2.0 KiB", "Remote host: ../runtime/" + release.SDKVersion() + "/release.wasm", "verified against SDK " + release.SDKVersion(), "Next: karty serve 'dist'", "http://127.0.0.1:8080/web/", "mirror alongside the game"} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("CDN summary missing %q:\n%s", expected, &output)
		}
	}
}

type failedSummaryWriter struct{}

func (failedSummaryWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}
