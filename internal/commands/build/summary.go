package build

import (
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	buildservice "github.com/karty-game/karty/internal/build"
	"github.com/karty-game/karty/internal/ui"
)

func writeSummary(output io.Writer, report buildservice.Summary) error {
	var text strings.Builder

	fmt.Fprintf(
		&text,
		"Built %s for %s in %s using SDK %s\n",
		report.Project,
		report.Target,
		report.Duration.Round(time.Millisecond),
		report.SDK,
	)
	fmt.Fprintf(&text, "  Levels: %d | Chunks: %d\n", report.Levels, report.Chunks)
	fmt.Fprintf(&text, "  Textures: %d | Sounds: %d | Audio streams: %d\n", report.Textures, report.Sounds, report.Streams)
	fmt.Fprintf(&text, "  Output: %s, %d files, %s\n", filepath.ToSlash(report.Directory), report.Files, ui.Size(report.Bytes))
	fmt.Fprintf(&text, "  Game cartridge: %s\n", ui.Size(report.CartridgeBytes))

	if report.HostURL != "" {
		fmt.Fprintf(&text, "  Remote host: %s, verified against SDK %s\n", report.HostURL, report.SDK)
	}

	switch report.Target {
	case "web":
		switch {
		case report.PreviewURL != "":
			fmt.Fprintf(&text, "\nPreview: %s (reloads after a successful build)\n", report.PreviewURL)
		case strings.HasPrefix(report.HostURL, "../"):
			fmt.Fprintf(
				&text,
				"\nNext: karty serve %s\n",
				commandPath(filepath.ToSlash(filepath.Dir(report.Directory)), runtime.GOOS == "windows"),
			)
			fmt.Fprintln(&text, "Open http://127.0.0.1:8080/web/. Place the relative runtime mirror alongside the game directory.")
		default:
			fmt.Fprintln(&text, "\nNext: karty serve")
			fmt.Fprintln(&text, "Open http://127.0.0.1:8080/. Upload the entire output directory to a static web host to share it.")
		}

		if report.HostURL != "" {
			fmt.Fprintln(&text, "Keep the remote host URL reachable. Serve a relative mirror alongside the game.")
		}
	case "native":
		writeNativeNext(&text, report)
	default:
		fmt.Fprintln(&text, "\nNext: karty dev for a browser preview, or karty build --target native for a desktop game.")
	}

	_, err := io.WriteString(output, text.String())

	return err
}

func writeNativeNext(text *strings.Builder, report buildservice.Summary) {
	platform := report.Platform
	if platform == "" {
		platform = runtime.GOOS + "-" + runtime.GOARCH
	}

	windows := strings.HasPrefix(platform, "windows-")
	name := "karty-host"

	if windows {
		name += ".exe"
	}

	path := filepath.ToSlash(filepath.Join(report.Directory, name))
	if !filepath.IsAbs(path) {
		path = "./" + path
	}

	fmt.Fprintf(text, "\nNext on %s", platform)

	if windows {
		fmt.Fprint(text, " in PowerShell")
	}

	fmt.Fprintln(text, ":")

	if windows {
		fmt.Fprint(text, "  & ")
	} else {
		fmt.Fprint(text, "  ")
	}

	fmt.Fprintf(
		text,
		"%s --cartridge %s\n",
		commandPath(path, windows),
		commandPath(filepath.ToSlash(filepath.Join(report.Directory, "game.kart")), windows),
	)
	fmt.Fprintln(text, "Share this platform's host, cartridge and any content folder with the runtime license.")
}

// commandPath quotes a literal path for the displayed POSIX or PowerShell command.
func commandPath(path string, windows bool) string {
	if windows {
		return "'" + strings.ReplaceAll(path, "'", "''") + "'"
	}

	return "'" + strings.ReplaceAll(path, "'", "'\\''") + "'"
}
