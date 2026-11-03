package toolchain

import (
	"archive/tar"
	"bytes"
	"compress/bzip2"
	"context"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net/url"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/karty-game/karty/internal/contextio"
)

type ktxArchiveFile struct {
	data []byte
	link string
	mode fs.FileMode
}

const (
	ktxAliasDepthLimit = 16
	ktxPathLimit       = 4096
)

// Upstream distributions contain file aliases for shared libraries. Resolve
// those aliases to regular copies; managed installations contain no symlinks.
type ktxArchiveFiles struct {
	files    map[string]ktxArchiveFile
	bytes    int64
	expanded int64
	folded   map[string]bool
}

func (files *ktxArchiveFiles) add(name string, data []byte, mode fs.FileMode) error {
	return files.record(name, ktxArchiveFile{data: data, mode: mode})
}

func (files *ktxArchiveFiles) record(name string, file ktxArchiveFile) error {
	if !materialMemberPath(name) || len(files.files) >= materialEntryLimit ||
		int64(len(file.data)) > materialExpandedLimit-files.bytes {
		return fmt.Errorf("invalid KTX archive member %q: %w", name, os.ErrInvalid)
	}

	if files.folded == nil {
		files.folded = make(map[string]bool, len(files.files)+1)
		for existing := range files.files {
			files.folded[strings.ToLower(existing)] = true
		}
	}

	key := strings.ToLower(name)
	if files.folded[key] {
		return fmt.Errorf("duplicate KTX archive member %q: %w", name, os.ErrInvalid)
	}

	if file.link != "" && !materialMemberPath(file.link) {
		return fmt.Errorf("invalid KTX library alias: %w", os.ErrInvalid)
	}

	files.files[name] = file
	files.folded[key] = true
	files.bytes += int64(len(file.data))

	return nil
}

func (files *ktxArchiveFiles) resolve(name string, seen map[string]bool) (ktxArchiveFile, error) {
	file, found := files.files[name]
	if !found || seen[name] || len(seen) > ktxAliasDepthLimit {
		return ktxArchiveFile{}, fmt.Errorf("invalid KTX library alias: %w", os.ErrInvalid)
	}

	if file.link == "" {
		return file, nil
	}

	seen[name] = true

	return files.resolve(file.link, seen)
}

func extractKTXArchive(ctx context.Context, destination string, contents []byte, artifact MaterialToolArtifact) error {
	files := ktxArchiveFiles{files: make(map[string]ktxArchiveFile)}

	var err error

	switch artifact.Format {
	case "tar.bz2":
		err = extractKTXTar(ctx, contents, artifact.URL, &files)
	case "pkg":
		err = extractKTXPackage(ctx, contents, &files)
	case "nsis":
		err = extractKTXNSIS(ctx, contents, files.add)
	default:
		err = fmt.Errorf("unsupported KTX archive format: %w", os.ErrInvalid)
	}

	if err != nil {
		return err
	}

	writer := materialArchiveWriter{destination: destination, seen: make(map[string]bool)}
	for _, name := range slices.Sorted(maps.Keys(files.files)) {
		file, resolveErr := files.resolve(name, make(map[string]bool))
		if resolveErr != nil {
			return resolveErr
		}

		if err := writer.write(ctx, name, false, int64(len(file.data)), bytes.NewReader(file.data)); err != nil {
			return err
		}
	}

	return nil
}

func extractKTXTar(ctx context.Context, contents []byte, location string, files *ktxArchiveFiles) error {
	resource, err := url.Parse(location)
	if err != nil {
		return err
	}

	prefix := strings.TrimSuffix(path.Base(resource.Path), ".tar.bz2")
	if prefix == "" || !materialMemberPath(prefix) {
		return os.ErrInvalid
	}

	input := bytes.NewReader(contents)
	stream := contextio.Reader(ctx, &ktxExpandedReader{input: bzip2.NewReader(input), files: files})

	reader := tar.NewReader(stream)
	for count := 0; ; count++ {
		header, readErr := reader.Next()
		if readErr == io.EOF {
			if err := finishKTXStream(ctx, stream, 1<<20); err != nil {
				return err
			}

			if input.Len() != 0 {
				return os.ErrInvalid
			}

			return ctx.Err()
		}

		if readErr != nil {
			return readErr
		}

		if count >= materialEntryLimit || header.Size < 0 || header.Size > materialExpandedLimit-files.bytes {
			return os.ErrInvalid
		}

		if err := extractKTXTarMember(ctx, reader, header, prefix, files); err != nil {
			return err
		}
	}
}

func extractKTXTarMember(ctx context.Context, reader io.Reader, header *tar.Header, prefix string, files *ktxArchiveFiles) error {
	name := strings.TrimSuffix(header.Name, "/")
	if !materialMemberPath(name) {
		return os.ErrInvalid
	}

	if name == prefix && header.Typeflag == tar.TypeDir {
		return nil
	}

	relative, found := strings.CutPrefix(name, prefix+"/")
	if !found || !materialMemberPath(relative) {
		return os.ErrInvalid
	}

	switch header.Typeflag {
	case tar.TypeDir:
		if header.Size != 0 {
			return os.ErrInvalid
		}

		return nil
	case tar.TypeReg:
		if header.Size > materialExpandedLimit-files.expanded {
			return os.ErrInvalid
		}

		data, err := contextio.ReadAll(ctx, io.LimitReader(reader, header.Size+1))
		if err != nil {
			return err
		}

		if int64(len(data)) != header.Size {
			return os.ErrInvalid
		}

		return files.add(relative, data, fs.FileMode(header.Mode)&0o777)
	case tar.TypeSymlink:
		if header.Size != 0 {
			return os.ErrInvalid
		}

		target, err := ktxAliasTarget(relative, header.Linkname)
		if err != nil {
			return err
		}

		return files.record(relative, ktxArchiveFile{link: target})
	default:
		return os.ErrInvalid
	}
}

// Library aliases stay in the archive's virtual file map. They are resolved to
// regular byte copies, and are never passed to filesystem symlink creation.
func ktxAliasTarget(name, link string) (string, error) {
	if link == "" || len(link) > ktxPathLimit || strings.HasPrefix(link, "/") || strings.ContainsAny(link, "\\\x00") {
		return "", os.ErrInvalid
	}
	// #nosec G305 -- This is a virtual archive lookup; validation confines the result before any filesystem write.
	target := path.Clean(path.Join(path.Dir(name), link))
	if !materialMemberPath(target) {
		return "", os.ErrInvalid
	}

	return target, nil
}

// Count every expanded byte, including archive headers, names and padding,
// across component payloads. Reaching the bound never synthesizes EOF, so a
// decoder must still consume its real checksum/footer before acceptance.
type ktxExpandedReader struct {
	input io.Reader
	files *ktxArchiveFiles
}

func (reader *ktxExpandedReader) Read(destination []byte) (int, error) {
	remaining := materialExpandedLimit - reader.files.expanded
	if remaining < 0 {
		return 0, os.ErrInvalid
	}

	count, err := reader.input.Read(destination[:min(int64(len(destination)), remaining+1)])
	if int64(count) > remaining {
		return 0, os.ErrInvalid
	}

	reader.files.expanded += int64(count)

	return count, err
}

func finishKTXStream(ctx context.Context, stream io.Reader, maximum int64) error {
	padding, err := contextio.ReadAll(ctx, io.LimitReader(stream, maximum+1))
	if err != nil {
		return err
	}

	if int64(len(padding)) > maximum {
		return os.ErrInvalid
	}

	for _, value := range padding {
		if value != 0 {
			return os.ErrInvalid
		}
	}

	return ctx.Err()
}
