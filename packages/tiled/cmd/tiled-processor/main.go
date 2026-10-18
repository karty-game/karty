// tiled-processor converts confined Tiled source documents into compiled map records.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/karty-game/tiled"
	"github.com/karty-game/tiled/source"
)

const maxBytes = 16 << 20
const maxFiles = 256

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(input io.Reader, output io.Writer) error {
	data, err := io.ReadAll(io.LimitReader(input, maxBytes+maxFiles*262+9))
	if err != nil {
		return err
	}
	files, err := decodeRequest(data)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(files))
	for name := range files {
		if strings.HasSuffix(name, ".tmj") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return fmt.Errorf("Tiled processor requires a .tmj input")
	}
	response := []byte{'K', 'P', 'R', '1', 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(response[4:], uint32(len(names)))
	total := 0
	for _, name := range names {
		m, err := source.Load(files, name)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		encoded, err := tiled.Encode(m)
		if err != nil {
			return err
		}
		if len(encoded) > maxBytes-total {
			return fmt.Errorf("compiled Tiled maps exceed processor bounds")
		}
		total += len(encoded)
		name = strings.TrimSuffix(name, ".tmj") + ".bin"
		response = binary.LittleEndian.AppendUint16(response, uint16(len(name)))
		response = binary.LittleEndian.AppendUint32(response, uint32(len(encoded)))
		response = append(response, name...)
		response = append(response, encoded...)
	}
	_, err = output.Write(response)
	return err
}
func decodeRequest(data []byte) (memoryFS, error) {
	bad := fmt.Errorf("invalid bounded KPR1 Tiled request")
	if len(data) < 8 || len(data) > maxBytes+maxFiles*262+8 || string(data[:4]) != "KPR1" {
		return nil, bad
	}
	count := binary.LittleEndian.Uint32(data[4:8])
	if count > maxFiles {
		return nil, bad
	}
	data = data[8:]
	files := memoryFS{}
	total := uint64(0)
	for range count {
		if len(data) < 6 {
			return nil, bad
		}
		n := int(binary.LittleEndian.Uint16(data))
		size := uint64(binary.LittleEndian.Uint32(data[2:]))
		data = data[6:]
		total += size
		if n == 0 || n > 256 || total > maxBytes || uint64(n)+size > uint64(len(data)) {
			return nil, bad
		}
		name := string(data[:n])
		if !fs.ValidPath(name) || strings.ContainsAny(name, "\\:\x00") {
			return nil, bad
		}
		if _, ok := files[name]; ok {
			return nil, bad
		}
		files[name] = data[n : uint64(n)+size]
		data = data[uint64(n)+size:]
	}
	if len(data) != 0 {
		return nil, bad
	}
	return files, nil
}

type memoryFS map[string][]byte

func (m memoryFS) Open(name string) (fs.File, error) {
	data, ok := m[name]
	if !ok || !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return &memoryFile{Reader: bytes.NewReader(data), name: name, size: int64(len(data))}, nil
}

type memoryFile struct {
	*bytes.Reader
	name string
	size int64
}

func (f *memoryFile) Close() error               { return nil }
func (f *memoryFile) Stat() (fs.FileInfo, error) { return f, nil }
func (f *memoryFile) Name() string               { return path.Base(f.name) }
func (f *memoryFile) Size() int64                { return f.size }
func (f *memoryFile) Mode() fs.FileMode          { return 0444 }
func (f *memoryFile) ModTime() time.Time         { return time.Time{} }
func (f *memoryFile) IsDir() bool                { return false }
func (f *memoryFile) Sys() any                   { return nil }
