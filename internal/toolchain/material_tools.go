package toolchain

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// MaterialTool identifies a binary tool, not a source build or the Unity editor.
type MaterialTool string

const (
	MaterialToolCrunch      MaterialTool = "crunch"
	MaterialToolMaterialize MaterialTool = "materialize-cli" // AiGameKit Materialize Rust CLI.
)

// MaterialToolArtifact describes a publisher-supplied, checksum-pinned archive.
// Executable is its exact slash-separated member path; all other regular files
// (including licenses and notices) are preserved alongside it.
type MaterialToolArtifact struct {
	URL        string
	SHA256     string
	Format     string // "tar.gz" or "zip"; never inferred from the URL.
	Executable string
}

// MaterialToolSpec pins a full Git commit and per-developer-platform artifacts.
// MaterializeSpec adapts optional released SDK pins to this installer contract;
// the current default SDK does not yet select Materialize. No PATH fallback exists.
type MaterialToolSpec struct {
	Tool      MaterialTool
	Revision  string
	Artifacts map[string]MaterialToolArtifact
}

type InstallMaterialToolOptions struct {
	Spec     MaterialToolSpec
	CacheDir string
	Platform string // Empty selects runtime.GOOS + "-" + runtime.GOARCH.
	Download func(context.Context, string) (io.ReadCloser, error)
}

const (
	materialArchiveLimit  = 256 << 20
	materialExpandedLimit = 512 << 20
	materialEntryLimit    = 4096
	materialControlLimit  = ' '
)

// InstallMaterialTool installs only the explicitly supplied verified binary.
// It never invokes a compiler, executes downloaded code, or resolves PATH.
// The verified archive is retained to recheck every file on warm installs;
// damaged caches fail closed and must be removed before reinstalling.
func InstallMaterialTool(ctx context.Context, options InstallMaterialToolOptions) (string, error) {
	platform := options.Platform
	if platform == "" {
		platform = runtime.GOOS + "-" + runtime.GOARCH
	}

	artifact, err := materialArtifact(options.Spec, platform)
	if err != nil {
		return "", err
	}

	parent, err := materialInstallParent(options)
	if err != nil {
		return "", err
	}

	installDir := filepath.Join(parent, platform)

	warm, err := materialInstallExists(installDir)
	if err != nil {
		return "", err
	}

	contents, err := readMaterialArchive(ctx, options, artifact, installDir, warm)
	if err != nil {
		return "", err
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}

	staging, err := os.MkdirTemp(parent, ".material-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(staging)

	if err := stageMaterialInstall(ctx, staging, contents, artifact); err != nil {
		return "", err
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}

	if err := publishMaterialInstall(staging, installDir, warm); err != nil {
		return "", err
	}

	return filepath.Join(installDir, "files", filepath.FromSlash(artifact.Executable)), nil
}

func materialInstallParent(options InstallMaterialToolOptions) (string, error) {
	home, err := kartyHome(options.CacheDir)
	if err != nil {
		return "", err
	}

	home, err = filepath.Abs(home)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(home, 0o750); err != nil {
		return "", err
	}

	// The caller's cache root is trusted; managed descendants may not be links.
	parent := home
	for _, component := range []string{"tools", string(options.Spec.Tool), options.Spec.Revision} {
		parent = filepath.Join(parent, component)
		if err := os.Mkdir(parent, 0o750); err != nil && !os.IsExist(err) {
			return "", err
		}

		if err := materialDirectory(parent); err != nil {
			return "", err
		}
	}

	return parent, nil
}

func materialInstallExists(installDir string) (bool, error) {
	if _, err := os.Lstat(installDir); err == nil {
		if err := materialDirectory(installDir); err != nil {
			return false, err
		}

		return true, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}

	return false, nil
}

func openMaterialArchive(
	ctx context.Context, options InstallMaterialToolOptions, artifact MaterialToolArtifact, installDir string, warm bool,
) (io.ReadCloser, error) {
	if warm {
		archivePath := filepath.Join(installDir, "artifact.archive")

		info, err := os.Lstat(archivePath)
		if err != nil || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("invalid cached material archive: %w", os.ErrInvalid)
		}

		return os.Open(archivePath)
	}

	download := options.Download
	if download == nil {
		download = downloadURL
	}

	archive, err := download(ctx, artifact.URL)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", options.Spec.Tool, err)
	}

	return archive, nil
}

func readMaterialArchive(
	ctx context.Context, options InstallMaterialToolOptions, artifact MaterialToolArtifact, installDir string, warm bool,
) ([]byte, error) {
	archive, err := openMaterialArchive(ctx, options, artifact, installDir, warm)
	if err != nil {
		return nil, err
	}

	contents, readErr := io.ReadAll(io.LimitReader(archive, materialArchiveLimit+1))
	closeErr := archive.Close()

	if readErr != nil {
		return nil, readErr
	}

	if closeErr != nil {
		return nil, closeErr
	}

	if len(contents) > materialArchiveLimit {
		return nil, fmt.Errorf("material archive exceeds download bound: %w", os.ErrInvalid)
	}

	digest := sha256.Sum256(contents)
	if hex.EncodeToString(digest[:]) != artifact.SHA256 {
		return nil, fmt.Errorf("material archive checksum mismatch: %w", os.ErrInvalid)
	}

	return contents, nil
}

func stageMaterialInstall(ctx context.Context, staging string, contents []byte, artifact MaterialToolArtifact) error {
	payload := filepath.Join(staging, "files")
	if err := os.Mkdir(payload, 0o750); err != nil {
		return err
	}

	if err := extractMaterialArchive(ctx, payload, contents, artifact); err != nil {
		return err
	}

	executable := filepath.Join(payload, filepath.FromSlash(artifact.Executable))

	info, err := os.Lstat(executable)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("material executable missing or empty: %w", os.ErrInvalid)
	}

	if err := os.Chmod(executable, 0o700); err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(staging, "artifact.archive"), contents, 0o600)
}

func publishMaterialInstall(staging, installDir string, warm bool) error {
	if warm {
		if err := compareMaterialInstall(staging, installDir); err != nil {
			return fmt.Errorf("material cache integrity: %w", err)
		}
	} else if err := os.Rename(staging, installDir); err != nil {
		// A concurrent installer may have won. Accept only identical contents.
		if integrityErr := compareMaterialInstall(staging, installDir); integrityErr != nil {
			return fmt.Errorf("publish material tool: %w", err)
		}
	}

	return nil
}

func materialArtifact(spec MaterialToolSpec, platform string) (MaterialToolArtifact, error) {
	if spec.Tool != MaterialToolCrunch && spec.Tool != MaterialToolMaterialize {
		return MaterialToolArtifact{}, fmt.Errorf("unsupported material tool %q: %w", spec.Tool, os.ErrInvalid)
	}

	if !materialHex(spec.Revision, 40) {
		return MaterialToolArtifact{}, fmt.Errorf("material tool requires exact full Git revision: %w", os.ErrInvalid)
	}

	switch platform {
	case "linux-amd64", "linux-arm64", "darwin-arm64", "windows-amd64":
	default:
		return MaterialToolArtifact{}, fmt.Errorf("unsupported material platform %q: %w", platform, os.ErrInvalid)
	}

	artifact, found := spec.Artifacts[platform]
	artifactURL, err := url.Parse(artifact.URL)

	name := string(spec.Tool)
	if platform == "windows-amd64" {
		name += ".exe"
	}

	if !found || err != nil || artifactURL.Scheme != "https" || artifactURL.Host == "" ||
		artifactURL.User != nil || artifactURL.Fragment != "" ||
		!materialHex(artifact.SHA256, 64) || (artifact.Format != "tar.gz" && artifact.Format != "zip") ||
		!materialMemberPath(artifact.Executable) || path.Base(artifact.Executable) != name {
		return MaterialToolArtifact{}, fmt.Errorf("incomplete or invalid material artifact for %s: %w", platform, os.ErrInvalid)
	}

	return artifact, nil
}

func materialHex(value string, length int) bool {
	decoded, err := hex.DecodeString(value)

	return err == nil && len(value) == length && hex.EncodeToString(decoded) == value
}

func materialMemberPath(name string) bool {
	if name == "" || name == "." || name == ".." || strings.HasPrefix(name, "../") ||
		strings.HasPrefix(name, "/") || strings.ContainsAny(name, "\\:\"<>|?*\x00") || path.Clean(name) != name {
		return false
	}

	for _, character := range name {
		if character < materialControlLimit {
			return false
		}
	}

	// Validate Windows names even when installing Windows fixtures on Unix.
	for component := range strings.SplitSeq(name, "/") {
		if strings.TrimRight(component, " .") != component {
			return false
		}

		base, _, _ := strings.Cut(strings.ToUpper(component), ".")
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" {
			return false
		}

		for _, prefix := range []string{"COM", "LPT"} {
			suffix, found := strings.CutPrefix(base, prefix)
			if found && len([]rune(suffix)) == 1 && strings.ContainsAny(suffix, "123456789¹²³") {
				return false
			}
		}
	}

	return true
}

func materialDirectory(name string) error {
	info, err := os.Lstat(name)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		return fmt.Errorf("material cache directory is not a real directory: %w", os.ErrInvalid)
	}

	return nil
}

func extractMaterialArchive(ctx context.Context, destination string, contents []byte, artifact MaterialToolArtifact) error {
	writer := &materialArchiveWriter{
		destination: destination,
		seen:        make(map[string]bool),
	}

	if artifact.Format == "zip" {
		return extractMaterialZip(ctx, contents, writer)
	}

	return extractMaterialTar(ctx, contents, writer)
}

// materialArchiveWriter tracks bounds and member uniqueness across the full archive.
type materialArchiveWriter struct {
	destination string
	seen        map[string]bool
	total       int64
	entries     int
}

func (writer *materialArchiveWriter) write(ctx context.Context, name string, directory bool, size int64, source io.Reader) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	writer.entries++

	if directory {
		name = strings.TrimSuffix(name, "/")
	}

	if writer.entries > materialEntryLimit || !materialMemberPath(name) || writer.seen[name] ||
		size < 0 || size > materialExpandedLimit-writer.total {
		return fmt.Errorf("invalid or oversized material archive member %q: %w", name, os.ErrInvalid)
	}

	writer.seen[name] = true
	writer.total += size

	target := filepath.Join(writer.destination, filepath.FromSlash(name))
	if directory {
		return os.MkdirAll(target, 0o750)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return err
	}

	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}

	_, copyErr := io.CopyN(file, source, size)
	closeErr := file.Close()

	if copyErr != nil {
		return copyErr
	}

	return closeErr
}

func extractMaterialZip(ctx context.Context, contents []byte, writer *materialArchiveWriter) error {
	reader, err := zip.NewReader(bytes.NewReader(contents), int64(len(contents)))
	if err != nil {
		return err
	}

	if len(reader.File) > materialEntryLimit {
		return fmt.Errorf("too many material archive members: %w", os.ErrInvalid)
	}

	for _, entry := range reader.File {
		if err := extractMaterialZipEntry(ctx, entry, writer); err != nil {
			return err
		}
	}

	return nil
}

func extractMaterialZipEntry(ctx context.Context, entry *zip.File, writer *materialArchiveWriter) error {
	if (!entry.Mode().IsRegular() && !entry.Mode().IsDir()) || entry.UncompressedSize64 > materialExpandedLimit {
		return fmt.Errorf("non-regular or oversized material ZIP member: %w", os.ErrInvalid)
	}

	input, err := entry.Open()
	if err != nil {
		return err
	}

	writeErr := writer.write(ctx, entry.Name, entry.Mode().IsDir(), int64(entry.UncompressedSize64), input)
	// Read through EOF to enforce ZIP CRC and declared size.
	if writeErr == nil {
		var extra [1]byte

		n, eofErr := input.Read(extra[:])
		if n != 0 || eofErr != io.EOF {
			writeErr = fmt.Errorf("invalid material ZIP contents: %w", os.ErrInvalid)
		}
	}

	closeErr := input.Close()

	if writeErr != nil {
		return writeErr
	}

	return closeErr
}

func extractMaterialTar(ctx context.Context, contents []byte, writer *materialArchiveWriter) error {
	reader, err := gzip.NewReader(bytes.NewReader(contents))
	if err != nil {
		return err
	}
	defer reader.Close()

	// Bound the entire decompressed stream, including padding and PAX headers.
	limited := &io.LimitedReader{R: reader, N: materialExpandedLimit + 1}
	tarReader := tar.NewReader(limited)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeDir {
			return fmt.Errorf("non-regular material TAR member: %w", os.ErrInvalid)
		}

		if err := writer.write(ctx, header.Name, header.Typeflag == tar.TypeDir, header.Size, tarReader); err != nil {
			return err
		}
	}

	if _, err := io.Copy(io.Discard, limited); err != nil {
		return err
	}

	if limited.N == 0 {
		return fmt.Errorf("material archive exceeds expansion bound: %w", os.ErrInvalid)
	}

	return nil
}

func compareMaterialInstall(expected, actual string) error {
	if err := materialDirectory(actual); err != nil {
		return err
	}

	entries := make(map[string]fs.FileInfo)

	err := filepath.Walk(expected, func(name string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relative, err := filepath.Rel(expected, name)
		if err != nil {
			return err
		}

		entries[relative] = info

		return nil
	})
	if err != nil {
		return err
	}

	err = filepath.Walk(actual, func(name string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relative, err := filepath.Rel(actual, name)
		if err != nil {
			return err
		}

		want, found := entries[relative]
		if !found || info.Mode().Type() != want.Mode().Type() || (!info.IsDir() &&
			(!info.Mode().IsRegular() || info.Size() != want.Size() || (runtime.GOOS != "windows" && info.Mode().Perm() != want.Mode().Perm()))) {
			return fmt.Errorf("changed material cache member %q: %w", relative, os.ErrInvalid)
		}

		delete(entries, relative)

		if info.IsDir() {
			return nil
		}

		wantHash, err := hashMaterialFile(filepath.Join(expected, relative), want.Size())
		if err != nil {
			return err
		}

		actualHash, err := hashMaterialFile(name, want.Size())
		if err != nil {
			return err
		}

		if wantHash != actualHash {
			return fmt.Errorf("changed material cache contents %q: %w", relative, os.ErrInvalid)
		}

		return nil
	})
	if err != nil {
		return err
	}

	if len(entries) != 0 {
		return fmt.Errorf("missing material cache members: %w", os.ErrInvalid)
	}

	return nil
}

func hashMaterialFile(filename string, size int64) ([sha256.Size]byte, error) {
	file, err := os.Open(filename)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	defer file.Close()

	hash := sha256.New()
	_, err = io.Copy(hash, io.LimitReader(file, size+1))

	var sum [sha256.Size]byte
	copy(sum[:], hash.Sum(nil))

	return sum, err
}
