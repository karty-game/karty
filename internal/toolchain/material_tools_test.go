package toolchain

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type materialFixtureMember struct {
	name     string
	body     string
	mode     os.FileMode
	typeflag byte
}

func materialFixture(t *testing.T, format string, members ...materialFixtureMember) []byte {
	t.Helper()

	var buffer bytes.Buffer

	if format == "zip" {
		writeMaterialZipFixture(t, &buffer, members)
	} else {
		writeMaterialTarFixture(t, &buffer, members)
	}

	return buffer.Bytes()
}

func writeMaterialZipFixture(t *testing.T, destination io.Writer, members []materialFixtureMember) {
	t.Helper()

	writer := zip.NewWriter(destination)

	for _, member := range members {
		header := &zip.FileHeader{Name: member.name, Method: zip.Deflate}
		header.SetMode(member.mode)

		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := io.WriteString(entry, member.body); err != nil {
			t.Fatal(err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeMaterialTarFixture(t *testing.T, destination io.Writer, members []materialFixtureMember) {
	t.Helper()

	compressed := gzip.NewWriter(destination)
	writer := tar.NewWriter(compressed)

	for _, member := range members {
		writeMaterialTarFixtureMember(t, writer, member)
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeMaterialTarFixtureMember(t *testing.T, writer *tar.Writer, member materialFixtureMember) {
	t.Helper()

	header := &tar.Header{Name: member.name, Mode: 0o600, Size: int64(len(member.body)), Typeflag: member.typeflag}
	if member.typeflag == tar.TypeSymlink || member.typeflag == tar.TypeLink {
		header.Linkname = "bin/crunch"
		header.Size = 0
	}

	if err := writer.WriteHeader(header); err != nil {
		t.Fatal(err)
	}

	if header.Size != 0 {
		if _, err := io.WriteString(writer, member.body); err != nil {
			t.Fatal(err)
		}
	}
}

func materialFixtureOptions(t *testing.T, tool MaterialTool, platform, format string, contents []byte) InstallMaterialToolOptions {
	t.Helper()

	name := string(tool)
	if platform == "windows-amd64" {
		name += ".exe"
	}

	return InstallMaterialToolOptions{
		Spec: MaterialToolSpec{
			Tool: tool, Revision: strings.Repeat("a", 40),
			Artifacts: map[string]MaterialToolArtifact{platform: {
				URL:    "https://artifacts.example/" + platform + "/tool." + format,
				SHA256: fmt.Sprintf("%x", sha256.Sum256(contents)), Format: format, Executable: "bin/" + name,
			}},
		},
		CacheDir: t.TempDir(), Platform: platform,
		Download: func(_ context.Context, _ string) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(contents)), nil
		},
	}
}

func TestMaterialToolPlatformsAndWarmIntegrity(t *testing.T) {
	t.Parallel()

	for _, platform := range []string{"linux-amd64", "linux-arm64", "darwin-arm64", "windows-amd64"} {
		for _, tool := range []MaterialTool{MaterialToolCrunch, MaterialToolMaterialize} {
			t.Run(platform+"/"+string(tool), func(t *testing.T) {
				t.Parallel()
				checkMaterialPlatformWarmIntegrity(t, platform, tool)
			})
		}
	}
}

func checkMaterialPlatformWarmIntegrity(t *testing.T, platform string, tool MaterialTool) {
	t.Helper()

	format, name := "tar.gz", string(tool)
	if platform == "windows-amd64" {
		format, name = "zip", name+".exe"
	}

	contents := materialFixture(t, format,
		materialFixtureMember{name: "bin/" + name, body: "fixture binary", mode: 0o600},
		materialFixtureMember{name: "NOTICE.txt", body: "upstream notice", mode: 0o600})
	options := materialFixtureOptions(t, tool, platform, format, contents)
	calls := 0
	options.Download = func(_ context.Context, url string) (io.ReadCloser, error) {
		calls++

		if !strings.Contains(url, "/"+platform+"/") {
			t.Fatalf("wrong platform URL: %s", url)
		}

		return io.NopCloser(bytes.NewReader(contents)), nil
	}

	installed, err := InstallMaterialTool(t.Context(), options)
	if err != nil {
		t.Fatal(err)
	}

	if filepath.Base(installed) != name || !filepath.IsAbs(installed) {
		t.Fatalf("wrong executable path %s", installed)
	}

	notice := filepath.Join(filepath.Dir(filepath.Dir(installed)), "NOTICE.txt")
	if data, err := os.ReadFile(notice); err != nil || string(data) != "upstream notice" {
		t.Fatalf("notice not preserved: %q, %v", data, err)
	}

	checkMaterialWarmIntegrity(t, options, installed, &calls)
}

func checkMaterialWarmIntegrity(t *testing.T, options InstallMaterialToolOptions, installed string, calls *int) {
	t.Helper()

	if warm, err := InstallMaterialTool(t.Context(), options); err != nil || warm != installed || *calls != 1 {
		t.Fatalf("warm install: %s, %v; downloads=%d", warm, err, *calls)
	}

	// Same-length edits must not be accepted based on existence/size. WriteFile
	// preserves the installed executable's existing mode when overwriting it.
	if err := os.WriteFile(installed, []byte("corrupt binary"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := InstallMaterialTool(t.Context(), options); err == nil || *calls != 1 {
		t.Fatalf("tampered executable accepted or downloaded again: %v", err)
	}
}

func TestMaterialToolRejectsInvalidPins(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		edit func(*InstallMaterialToolOptions)
	}{
		{"platform", func(o *InstallMaterialToolOptions) { o.Platform = "darwin-amd64" }},
		{"missing artifact", func(o *InstallMaterialToolOptions) { delete(o.Spec.Artifacts, o.Platform) }},
		{"revision", func(o *InstallMaterialToolOptions) { o.Spec.Revision = "main" }},
		{"revision traversal", func(o *InstallMaterialToolOptions) { o.Spec.Revision = "../../escape" }},
		{"unity", func(o *InstallMaterialToolOptions) { o.Spec.Tool = "materialize-unity" }},
		{"checksum", func(o *InstallMaterialToolOptions) {
			a := o.Spec.Artifacts[o.Platform]
			a.SHA256 = "bad"
			o.Spec.Artifacts[o.Platform] = a
		}},
		{"url", func(o *InstallMaterialToolOptions) {
			a := o.Spec.Artifacts[o.Platform]
			a.URL = "http://artifacts.example/tool"
			o.Spec.Artifacts[o.Platform] = a
		}},
		{"executable", func(o *InstallMaterialToolOptions) {
			a := o.Spec.Artifacts[o.Platform]
			a.Executable = "../crunch"
			o.Spec.Artifacts[o.Platform] = a
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			options := materialFixtureOptions(t, MaterialToolCrunch, "linux-amd64", "tar.gz", nil)
			test.edit(&options)
			options.Download = func(context.Context, string) (io.ReadCloser, error) {
				t.Fatal("invalid spec attempted a download")

				return nil, os.ErrInvalid
			}

			if _, err := InstallMaterialTool(t.Context(), options); err == nil {
				t.Fatal("invalid pin accepted")
			}
		})
	}
}

func TestMaterialToolChecksumAndArchiveRejection(t *testing.T) {
	t.Parallel()

	for _, format := range []string{"tar.gz", "zip"} {
		for _, test := range []string{"checksum", "traversal", "absolute", "backslash", "drive", "device", "alias", "symlink", "duplicate", "missing executable", "hardlink"} {
			if format == "zip" && test == "hardlink" {
				continue
			}

			t.Run(format+"/"+test, func(t *testing.T) {
				t.Parallel()

				members := make([]materialFixtureMember, 0, 2)
				members = append(members, materialFixtureMember{name: "bin/crunch", body: "binary", mode: 0o600})
				bad := materialFixtureMember{name: "NOTICE.txt", body: "notice", mode: 0o600}

				switch test {
				case "traversal":
					bad.name = "../escape"
				case "absolute":
					bad.name = "/escape"
				case "backslash":
					bad.name = `..\escape`
				case "drive":
					bad.name = "C:/escape"
				case "device":
					bad.name = "NUL.txt"
				case "alias":
					bad.name = "bin/crunch."
				case "symlink":
					bad.mode, bad.typeflag = os.ModeSymlink|0o777, tar.TypeSymlink
				case "hardlink":
					bad.typeflag = tar.TypeLink
				case "duplicate":
					bad.name = "bin/crunch"
				case "missing executable":
					members = nil
				}

				contents := materialFixture(t, format, append(members, bad)...)

				options := materialFixtureOptions(t, MaterialToolCrunch, "linux-amd64", format, contents)
				if test == "checksum" {
					a := options.Spec.Artifacts[options.Platform]
					a.SHA256 = strings.Repeat("0", 64)
					options.Spec.Artifacts[options.Platform] = a
				}

				if _, err := InstallMaterialTool(t.Context(), options); err == nil {
					t.Fatal("bad archive accepted")
				}

				parent := filepath.Join(options.CacheDir, "tools", "crunch", options.Spec.Revision)
				if entries, err := os.ReadDir(parent); err != nil || len(entries) != 0 {
					t.Fatalf("failed install left published/staging files: %v, %v", entries, err)
				}
			})
		}
	}
}

func TestMaterialToolWarmArchiveAndNoticeIntegrity(t *testing.T) {
	t.Parallel()

	for _, member := range []string{"artifact.archive", "files/NOTICE", "files/extra", "files/crunch-link"} {
		t.Run(member, func(t *testing.T) {
			t.Parallel()

			contents := materialFixture(t, "zip", materialFixtureMember{name: "bin/crunch", body: "binary", mode: 0o600},
				materialFixtureMember{name: "NOTICE", body: "notice", mode: 0o600})
			options := materialFixtureOptions(t, MaterialToolCrunch, "linux-amd64", "zip", contents)

			installed, err := InstallMaterialTool(t.Context(), options)
			if err != nil {
				t.Fatal(err)
			}

			root := filepath.Dir(filepath.Dir(filepath.Dir(installed)))

			target := filepath.Join(root, filepath.FromSlash(member))
			if member == "files/crunch-link" {
				if err := os.Symlink(installed, target); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			} else if err := os.WriteFile(target, []byte("tamper"), 0o600); err != nil {
				t.Fatal(err)
			}

			options.Download = func(context.Context, string) (io.ReadCloser, error) {
				t.Fatal("warm cache must not silently replace corruption")

				return nil, os.ErrInvalid
			}

			if _, err := InstallMaterialTool(t.Context(), options); err == nil {
				t.Fatal("corrupt warm cache accepted")
			}
		})
	}
}

func TestMaterialToolExpansionBound(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	compressed := gzip.NewWriter(&buffer)

	writer := tar.NewWriter(compressed)
	if err := writer.WriteHeader(&tar.Header{Name: "bin/crunch", Mode: 0o700, Size: materialExpandedLimit + 1}); err != nil {
		t.Fatal(err)
	}

	// Deliberately omit the oversized body: rejection must occur at its header.
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}

	options := materialFixtureOptions(t, MaterialToolCrunch, "linux-amd64", "tar.gz", buffer.Bytes())
	if _, err := InstallMaterialTool(t.Context(), options); err == nil || !strings.Contains(err.Error(), "oversized") {
		t.Fatalf("oversized member not rejected at header: %v", err)
	}
}
