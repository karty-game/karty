package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type hostedReleaseFile struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
	Type   string `json:"type"`
	Role   string `json:"role"`
}
type hostedRelease struct {
	Version int                 `json:"version"`
	Files   []hostedReleaseFile `json:"files"`
}
type hostedPointer struct {
	Version  int    `json:"version"`
	Release  string `json:"release"`
	Manifest string `json:"manifest"`
}

func restageHostedSample(directory, shared string, inventory hostedRuntimeInventory) error {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()

	pointer, release, err := readHostedRelease(root)
	if err != nil {
		return err
	}

	index, err := root.ReadFile("index.html")
	if err != nil {
		return err
	}

	scripts := `[["wasm_exec.js", false], ["bootstrap.mjs", true]]`
	if strings.Count(string(index), scripts) != 1 {
		return fmt.Errorf("unsupported WebGPU sample shell: %w", os.ErrInvalid)
	}

	index = []byte(strings.Replace(string(index), scripts, `[["`+shared+`wasm_exec.js", false], ["`+shared+`bootstrap.mjs", true]]`, 1))
	if err = root.WriteFile("index.html", index, 0o600); err != nil {
		return err
	}

	files, keep, err := relocateHostedFiles(release.Files, index, shared, inventory)
	if err != nil {
		return err
	}

	release.Files = files

	data, err := json.Marshal(release)
	if err != nil {
		return err
	}

	data = append(data, '\n')
	pointer.Release = hashBytes(data)

	pointer.Manifest = "karty-releases/" + pointer.Release + ".json"
	if err = root.WriteFile(pointer.Manifest, data, 0o600); err != nil {
		return err
	}

	keep[pointer.Manifest] = true

	pointerData, err := json.Marshal(pointer)
	if err != nil {
		return err
	}

	if err = root.WriteFile("karty-latest.json", append(pointerData, '\n'), 0o600); err != nil {
		return err
	}
	// A published site contains the selected release only; source distributions
	// retain their complete standalone bundles and prior release generations.
	return fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			return nil
		}

		if !keep[name] {
			return root.Remove(filepath.FromSlash(path.Clean(name)))
		}

		return nil
	})
}

func readHostedRelease(root *os.Root) (hostedPointer, hostedRelease, error) {
	pointer := hostedPointer{}
	release := hostedRelease{}

	pointerData, err := root.ReadFile("karty-latest.json")
	if err != nil {
		return pointer, release, err
	}

	if err = json.Unmarshal(pointerData, &pointer); err != nil {
		return pointer, release, err
	}

	if pointer.Version != 1 || pointer.Manifest != "karty-releases/"+pointer.Release+".json" ||
		!hostedRuntimeHash.MatchString(pointer.Release) {
		return pointer, release, fmt.Errorf("invalid source release pointer: %w", os.ErrInvalid)
	}

	releaseData, err := root.ReadFile(pointer.Manifest)
	if err != nil {
		return pointer, release, err
	}

	if hashBytes(releaseData) != pointer.Release {
		return pointer, release, fmt.Errorf("source release checksum mismatch: %w", os.ErrInvalid)
	}

	if err = json.Unmarshal(releaseData, &release); err != nil {
		return pointer, release, err
	}

	if release.Version != 1 {
		return pointer, release, fmt.Errorf("unsupported source release: %w", os.ErrInvalid)
	}

	return pointer, release, nil
}

func relocateHostedFiles(
	source []hostedReleaseFile,
	index []byte,
	shared string,
	inventory hostedRuntimeInventory,
) ([]hostedReleaseFile, map[string]bool, error) {
	files := make([]hostedReleaseFile, 0, len(source))
	keep := map[string]bool{
		"karty-latest.json": true, "karty-sw.js": true, "karty-web-status.js": true,
		"RUNTIME_LICENSE.md": true, "SDK_LICENSE.md": true,
	}
	seen := map[string]bool{}
	sharedCount := 0

	for _, file := range source {
		if seen[file.URL] {
			return nil, nil, fmt.Errorf("duplicate source release URL: %w", os.ErrInvalid)
		}

		seen[file.URL] = true
		if file.URL == "runtime.bin" {
			continue
		}

		if entry, ok := inventory.Files[file.URL]; ok {
			if file.SHA256 != entry.SHA256 || file.Size != entry.Size || file.Role != "startup" {
				return nil, nil, fmt.Errorf("source runtime release pin mismatch: %w", os.ErrInvalid)
			}

			file.URL = shared + file.URL
			sharedCount++
		} else {
			if !fs.ValidPath(file.URL) || strings.Contains(file.URL, "\\") {
				return nil, nil, fmt.Errorf("source content escapes sample: %w", os.ErrInvalid)
			}

			if file.URL == "index.html" {
				file.SHA256 = hashBytes(index)
				file.Size = int64(len(index))
			}

			keep[file.URL] = true
		}

		files = append(files, file)
	}

	if sharedCount != len(inventory.Files) {
		return nil, nil, fmt.Errorf("source release omits runtime files: %w", os.ErrInvalid)
	}

	return files, keep, nil
}
