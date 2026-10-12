package build

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/sdk"
)

func TestCDNURLValidationAndTargetFailBeforeProjectIO(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"https://cdn.example/host.wasm", "../runtime/0.0.11/host.wasm", "runtime/host.wasm", "/runtime/host.wasm"} {
		if err := validateCDNURL(value); err != nil {
			t.Errorf("valid mirror %q rejected: %v", value, err)
		}
	}

	for _, value := range []string{
		"http://cdn.example/host.wasm", "//cdn.example/host.wasm", "javascript:alert(1)", "https://user@cdn.example/host.wasm", "host.wasm#fragment",
		"../../host.wasm", "foo/../host.wasm", "../%2e%2e/host.wasm", "runtime%2fhost.wasm", "host\\file.wasm", "host\n.wasm",
		"https://github.com/karty-game/karty-sdk/releases/download/sdk-v0.0.11/karty-host-web.wasm",
		"https://api.github.com/repos/karty-game/karty-sdk/releases/assets/1",
		"https://release-assets.githubusercontent.com/expiring-host.wasm?token=temporary",
	} {
		if err := validateCDNURL(value); err == nil {
			t.Errorf("invalid mirror %q accepted", value)
		}
	}

	err := RunWithOptions(
		t.Context(),
		filepath.Join(t.TempDir(), "missing-project"),
		Options{Target: "native", CDN: "https://cdn.example/host.wasm"},
	)
	if err == nil || !strings.Contains(err.Error(), "--cdn requires --target web") {
		t.Fatalf("native CDN build did not fail before project I/O: %v", err)
	}
}

func TestCDNRequiresReleasePinAndRejectsDifferentHostOverride(t *testing.T) {
	t.Parallel()

	contents := []byte("release host fixture")
	digest := sha256.Sum256(contents)
	manifest := sdk.Manifest{Version: "0.0.11"}
	manifest.Host.Version = manifest.Version
	manifest.Artifacts.Host.Web = sdk.ToolArtifact{
		URL:    "https://github.com/karty-game/karty-sdk/releases/download/sdk-v0.0.11/karty-host-web.wasm",
		SHA256: hex.EncodeToString(digest[:]),
	}
	manifest.Artifacts.Host.WebRuntime = sdk.ToolArtifact{
		URL:    "https://github.com/karty-game/karty-sdk/releases/download/sdk-v0.0.11/wasm_exec.js",
		SHA256: strings.Repeat("a", 64),
	}

	localHost := filepath.Join(t.TempDir(), "host.wasm")
	if err := os.WriteFile(localHost, contents, 0o600); err != nil {
		t.Fatal(err)
	}

	if host, err := cdnHostFromManifest(
		manifest,
		"../runtime/host.wasm",
		localHost,
	); err != nil ||
		host.SHA256 != manifest.Artifacts.Host.Web.SHA256 {
		t.Fatalf("matching release override = %+v, %v", host, err)
	}

	if err := os.WriteFile(localHost, []byte("different local host"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := cdnHostFromManifest(
		manifest,
		"../runtime/host.wasm",
		localHost,
	); err == nil ||
		!strings.Contains(err.Error(), "differs from the pinned release") {
		t.Fatalf("mismatched override accepted: %v", err)
	}

	manifest.Artifacts.Host.Web.SHA256 = ""
	if _, err := cdnHostFromManifest(manifest, "../runtime/host.wasm", ""); err == nil {
		t.Fatal("CDN host without a release checksum accepted")
	}
}

func TestCDNStagesLocalRuntimeAndExcludesBundledHostFromSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	raw := filepath.Join(root, "dist", "raw")
	if err := os.MkdirAll(raw, 0o750); err != nil {
		t.Fatal(err)
	}

	game := filepath.Join(raw, "game.kart")
	if err := os.WriteFile(game, []byte("\x00asm\x01\x00\x00\x00"), 0o600); err != nil {
		t.Fatal(err)
	}

	runtime := filepath.Join(root, "wasm_exec.js")
	if err := os.WriteFile(runtime, []byte("matched runtime"), 0o600); err != nil {
		t.Fatal(err)
	}

	web := filepath.Join(root, "dist", "web")
	if err := os.MkdirAll(web, 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(web, "karty-host.wasm"), make([]byte, 1<<20), 0o600); err != nil {
		t.Fatal(err)
	}

	cdn := &cdnHost{URL: "../runtime/0.0.11/host.wasm", SHA256: strings.Repeat("a", 64)}
	if err := stageTargetWithCDN(
		filepath.Join(root, "dist"),
		raw,
		game,
		nil,
		"",
		"web",
		runtime,
		"",
		false,
		"CDN game",
		false,
		cdn,
	); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(web, "karty-host.wasm")); !os.IsNotExist(err) {
		t.Fatalf("CDN build retained bundled host: %v", err)
	}

	launcher, err := os.ReadFile(filepath.Join(web, "karty.js"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(launcher), cdn.URL) || !strings.Contains(string(launcher), cdn.SHA256) ||
		strings.Contains(string(launcher), "@@HOST_SOURCE@@") {
		t.Fatalf("CDN host source missing from generated launcher:\n%s", launcher)
	}

	report, err := summarizeBuild(root, "test", "0.0.11", nil, assetpipeline.Report{}, Options{Target: "web", cdnHost: cdn})
	if err != nil {
		t.Fatal(err)
	}

	if report.Files != 4 || report.Bytes >= 1<<20 || report.HostURL != cdn.URL {
		t.Fatalf("CDN summary includes the absent host: %+v", report)
	}
}

func TestCDNLauncherVerifiesBytesBeforeActualWASMCompilation(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node unavailable; required by mise run check-web")
	}

	command := exec.CommandContext(t.Context(), "node", "testdata/launcher-cdn.test.mjs", "templates/launcher.js.tmpl")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("CDN loader: %v\n%s", err, output)
	}
}

func TestCDNHostLiteralPreservesQuotesAndEscapesScriptSeparators(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node unavailable; required by mise run check-web")
	}

	url := "https://cdn.example/host.wasm?name=\"quoted\"'&markup=<script>&separator=\u2028\u2029&astral=\U000e0001&emoji=\U0001f680"
	if err := validateCDNURL(url); err != nil {
		t.Fatal(err)
	}

	source, err := webHostSource("", &cdnHost{URL: url, SHA256: strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}

	if strings.ContainsAny(source, "<\u2028\u2029") {
		t.Fatalf("host literal contains script or line separators: %s", source)
	}

	// #nosec G204 -- The Node program and URL come only from this constant test fixture.
	command := exec.CommandContext(
		t.Context(),
		"node",
		"-e",
		"const assert = require('node:assert/strict'); const host = "+source+"; assert.equal(host.url, process.argv[1]); assert.equal(host.sha256, 'a'.repeat(64));",
		url,
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("host literal did not preserve the URL: %v\n%s", err, output)
	}
}
