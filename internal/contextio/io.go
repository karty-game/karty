// Package contextio keeps file/archive work responsive to a build's lifetime.
// It does not add goroutines: network readers must also bind their request to
// the context, while ordinary file and decompression reads stop between chunks.
package contextio

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"io/fs"
	"os"
)

const chunkSize = 32 * 1024

type reader struct {
	ctx   context.Context //nolint:containedctx // Each reader carries the caller's I/O lifetime across reads.
	input io.Reader
}

// Reader checks cancellation around every bounded read, including reads that
// also produce bytes or EOF. Wrapping removes optional unbounded fast paths.
func Reader(ctx context.Context, input io.Reader) io.Reader {
	return reader{ctx: ctx, input: input}
}

func (input reader) Read(destination []byte) (int, error) {
	if err := input.ctx.Err(); err != nil {
		return 0, err
	}

	n, err := input.input.Read(destination[:min(len(destination), chunkSize)])
	if canceled := input.ctx.Err(); canceled != nil {
		return n, canceled
	}

	return n, err
}

func ReadAll(ctx context.Context, input io.Reader) ([]byte, error) {
	return io.ReadAll(Reader(ctx, input))
}

func Copy(ctx context.Context, destination io.Writer, input io.Reader) (int64, error) {
	n, err := io.Copy(destination, Reader(ctx, input))
	if err == nil {
		err = ctx.Err()
	}

	return n, err
}

func CopyN(ctx context.Context, destination io.Writer, input io.Reader, size int64) (int64, error) {
	n, err := io.CopyN(destination, Reader(ctx, input), size)
	if err == nil {
		err = ctx.Err()
	}

	return n, err
}

func ReadFile(ctx context.Context, name string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return ReadAll(ctx, file)
}

func WriteFile(ctx context.Context, name string, contents []byte, mode fs.FileMode) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}

	_, writeErr := Copy(ctx, file, bytes.NewReader(contents))
	closeErr := file.Close()

	if writeErr != nil {
		return writeErr
	}

	return closeErr
}

func Sum256(ctx context.Context, contents []byte) ([sha256.Size]byte, error) {
	hash := sha256.New()
	_, err := Copy(ctx, hash, bytes.NewReader(contents))

	var sum [sha256.Size]byte
	copy(sum[:], hash.Sum(nil))

	return sum, err
}

// Filesystem wraps embedded/ZIP resource reads without changing path ownership.
func Filesystem(ctx context.Context, tree fs.FS) fs.FS {
	return filesystem{ctx: ctx, tree: tree}
}

type filesystem struct {
	ctx  context.Context //nolint:containedctx // Each resource tree carries the caller's I/O lifetime across opens.
	tree fs.FS
}

type file struct {
	fs.File

	input io.Reader
}

func (entry file) Read(destination []byte) (int, error) { return entry.input.Read(destination) }

func (tree filesystem) Open(name string) (fs.File, error) {
	if err := tree.ctx.Err(); err != nil {
		return nil, err
	}

	entry, err := tree.tree.Open(name)
	if err != nil {
		return nil, err
	}

	return file{File: entry, input: Reader(tree.ctx, entry)}, nil
}

func (tree filesystem) ReadDir(name string) ([]fs.DirEntry, error) {
	if err := tree.ctx.Err(); err != nil {
		return nil, err
	}

	entries, err := fs.ReadDir(tree.tree, name)
	if err == nil {
		err = tree.ctx.Err()
	}

	return entries, err
}
