package devserver

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type watchBenchmarkZeros struct{}

func (watchBenchmarkZeros) Read(destination []byte) (int, error) {
	clear(destination)

	return len(destination), nil
}

// BenchmarkWatcherLargeProject measures both fast polling and the periodic
// content verification against a warmed local filesystem. It makes no device
// throughput claim: the generated asset remains in the operating system cache.
func BenchmarkWatcherLargeProject(bench *testing.B) {
	const files = 10000

	const assetBytes = 128 << 20

	root := bench.TempDir()
	for index := range files {
		name := filepath.Join(root, fmt.Sprintf("source-%05d.go", index))
		if err := os.WriteFile(name, []byte("package fixture\n"), 0o600); err != nil {
			bench.Fatal(err)
		}
	}

	asset, err := os.Create(filepath.Join(root, "asset.bin"))
	if err != nil {
		bench.Fatal(err)
	}

	_, copyErr := io.CopyN(asset, watchBenchmarkZeros{}, assetBytes)

	closeErr := asset.Close()
	if copyErr != nil || closeErr != nil {
		bench.Fatal(copyErr, closeErr)
	}

	watcher, err := NewWatcher(root)
	if err != nil {
		bench.Fatal(err)
	}

	bench.Cleanup(func() { _ = watcher.Close() })

	now := time.Now()
	watcher.now = func() time.Time { return now }

	baseline, err := watcher.Scan(bench.Context())
	if err != nil {
		bench.Fatal(err)
	}

	for _, verify := range []bool{false, true} {
		name := "metadata"
		if verify {
			name = "expired-hashes"
		}

		bench.Run(name, func(bench *testing.B) {
			bench.ReportAllocs()
			bench.ReportMetric(files+1, "files/op")

			if verify {
				bench.SetBytes(int64(assetBytes + files*len("package fixture\n")))
			}

			for bench.Loop() {
				if verify {
					now = now.Add(maxWatchHashAge)
				}

				current, scanErr := watcher.Scan(bench.Context())
				if scanErr != nil || current != baseline {
					bench.Fatalf("unchanged project scan: digest=%s error=%v", current, scanErr)
				}
			}
		})
	}
}
