package packages

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type inventory struct {
	FormatVersion int          `json:"formatVersion"`
	Files         []fileRecord `json:"files"`
}
type fileRecord struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

func excluded(name string) bool {
	switch name {
	case ".git", ".karty", "dist", "node_modules", ".codex", ".agents":
		return true
	}

	return false
}

// Export packages source bytes, never generated game outputs. ZIP order,
// timestamps, inventory and permissions are deterministic; links are rejected.
func Export(ctx context.Context, directory, output string) (Manifest, string, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return Manifest{}, "", err
	}

	root, err := os.OpenRoot(absolute)
	if err != nil {
		return Manifest{}, "", err
	}
	defer root.Close()

	definition, err := ReadManifest(ctx, root)
	if err != nil {
		return definition, "", err
	}

	output, err = filepath.Abs(output)
	if err != nil {
		return definition, "", err
	}

	files, err := collectSourceFiles(ctx, root, absolute, output)
	if err != nil {
		return definition, "", err
	}

	encoded, err := encodeArchive(ctx, files)
	if err != nil {
		return definition, "", err
	}
	// Verify our own output through the same complete importer validation.
	if _, err := decodeArchive(ctx, encoded); err != nil {
		return definition, "", err
	}

	parent, err := os.OpenRoot(filepath.Dir(output))
	if err != nil {
		return definition, "", err
	}
	defer parent.Close()

	name := filepath.Base(output)
	if !validPath(name) || !strings.HasSuffix(name, ".kpkg") {
		return definition, "", ErrPackage
	}

	file, err := parent.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return definition, "", err
	}

	_, err = file.Write(encoded)

	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}

	if err != nil {
		_ = parent.Remove(name)

		return definition, "", err
	}

	return definition, Digest(encoded), nil
}

func decodeArchive(ctx context.Context, data []byte) (map[string][]byte, error) {
	if len(data) == 0 || len(data) > MaxArchiveBytes {
		return nil, ErrPackage
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	if len(reader.File) == 0 || len(reader.File) > MaxFiles {
		return nil, ErrPackage
	}

	files := map[string][]byte{}

	if err := validateZIPEntries(reader.File); err != nil {
		return nil, err
	}

	var total uint64

	for _, entry := range reader.File {
		name := entry.Name
		if entry.UncompressedSize64 > MaxFileBytes || entry.CompressedSize64 > MaxArchiveBytes ||
			total > MaxTotalBytes-entry.UncompressedSize64 {
			return nil, fmt.Errorf("%w: archive exceeds extraction bounds", ErrPackage)
		}

		total += entry.UncompressedSize64

		stream, err := entry.Open()
		if err != nil {
			return nil, err
		}

		limit := int64(MaxFileBytes)
		if name == InventoryName {
			limit = MaxManifestBytes
		}

		contents, readErr := readBounded(ctx, stream, limit)
		closeErr := stream.Close()

		if readErr != nil {
			return nil, readErr
		}

		if closeErr != nil {
			return nil, closeErr
		}

		if uint64(len(contents)) != entry.UncompressedSize64 {
			return nil, ErrPackage
		}

		files[name] = contents
	}

	var index inventory

	decoder := json.NewDecoder(bytes.NewReader(files[InventoryName]))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&index); err != nil {
		return nil, err
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("%w: trailing inventory data", ErrPackage)
	}

	if index.FormatVersion != 1 || len(index.Files) != len(files)-1 {
		return nil, ErrPackage
	}

	seen := map[string]bool{}

	for _, record := range index.Files {
		contents, exists := files[record.Path]
		if !exists || seen[record.Path] || record.Path == InventoryName || record.Size != int64(len(contents)) ||
			record.SHA256 != Digest(contents) {
			return nil, fmt.Errorf("%w: inventory mismatch for %s", ErrPackage, record.Path)
		}

		seen[record.Path] = true
	}

	if files[ManifestName] == nil {
		return nil, fmt.Errorf("%w: missing %s", ErrPackage, ManifestName)
	}

	return files, nil
}

func collectSourceFiles(ctx context.Context, root *os.Root, absolute, output string) (map[string][]byte, error) {
	files := map[string][]byte{}

	var total int64

	err := fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		if name == "." {
			return nil
		}

		if excluded(entry.Name()) {
			if entry.IsDir() {
				return fs.SkipDir
			}

			return nil
		}

		if filepath.Join(absolute, filepath.FromSlash(name)) == output {
			return nil
		}

		if !validPath(name) || entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: unsupported source path %s", ErrPackage, name)
		}

		if entry.IsDir() {
			return nil
		}

		if name == InventoryName || len(files) >= MaxFiles-1 {
			return ErrPackage
		}

		data, err := Read(ctx, root, name, MaxFileBytes)
		if err != nil {
			return err
		}

		total += int64(len(data))
		if total > MaxTotalBytes {
			return ErrPackage
		}

		files[name] = data

		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

func encodeArchive(ctx context.Context, files map[string][]byte) ([]byte, error) {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}

	slices.Sort(names)

	index := inventory{FormatVersion: 1}
	folded := map[string]bool{}

	for _, name := range names {
		lower := strings.ToLower(name)
		if folded[lower] {
			return nil, fmt.Errorf("%w: case-colliding path %s", ErrPackage, name)
		}

		folded[lower] = true

		index.Files = append(index.Files, fileRecord{name, Digest(files[name]), int64(len(files[name]))})
	}

	data, err := json.Marshal(index)
	if err != nil {
		return nil, err
	}

	if len(data) > MaxManifestBytes {
		return nil, ErrPackage
	}

	files[InventoryName] = data
	names = append(names, InventoryName)
	slices.Sort(names)

	var buffer bytes.Buffer

	writer := zip.NewWriter(&buffer)

	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.Modified = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
		header.SetMode(0644)

		file, err := writer.CreateHeader(header)
		if err != nil {
			return nil, err
		}

		if _, err := file.Write(files[name]); err != nil {
			return nil, err
		}

		if buffer.Len() > MaxArchiveBytes {
			return nil, ErrPackage
		}
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	if buffer.Len() > MaxArchiveBytes {
		return nil, ErrPackage
	}

	return buffer.Bytes(), nil
}

func validateZIPEntries(entries []*zip.File) error {
	paths := map[string]bool{}
	components := map[string]string{}
	// Preflight every pathname before opening any compressed payload. The v1
	// writer emits regular files only; directory entries and links are rejected.
	for _, entry := range entries {
		name := entry.Name

		key := strings.ToLower(name)
		if !validPath(name) || paths[key] || !entry.Mode().IsRegular() || entry.FileInfo().IsDir() ||
			(entry.Method != zip.Store && entry.Method != zip.Deflate) {
			return fmt.Errorf("%w: invalid ZIP entry %q", ErrPackage, name)
		}

		paths[key] = true

		parts := strings.Split(name, "/")
		for index, part := range parts {
			if excluded(part) {
				return ErrPackage
			}

			prefix := strings.Join(parts[:index+1], "/")

			lower := strings.ToLower(prefix)
			if prior, exists := components[lower]; exists && prior != prefix {
				return fmt.Errorf("%w: case-colliding ZIP path %s", ErrPackage, name)
			}

			components[lower] = prefix
		}
	}

	for name := range paths {
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if paths[parent] {
				return fmt.Errorf("%w: ZIP file/directory collision", ErrPackage)
			}
		}
	}

	return nil
}
