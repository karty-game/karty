package main

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"time"

	"github.com/karty-game/karty/internal/guide"
	"github.com/pelletier/go-toml/v2"
)

const noticeTimeout = 60 * time.Second

func samplePriority(name string) int {
	sampleOrder := [...]string{"hangar", "crate-room", "environment", "world-camera", "ui-demo", "media-lab", "particle-lab"}
	if index := slices.Index(sampleOrder[:], name); index >= 0 {
		return index
	}

	return len(sampleOrder)
}

func sampleNames(root string) ([]string, error) {
	directory := filepath.Join(root, "samples")

	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}

	var names []string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		config, err := os.Lstat(filepath.Join(directory, entry.Name(), "karty.toml"))
		if os.IsNotExist(err) {
			continue
		}

		if err != nil {
			return nil, err
		}

		if !config.Mode().IsRegular() || !samplePathPart.MatchString(entry.Name()) {
			return nil, fmt.Errorf("invalid sample project: %s: %w", entry.Name(), os.ErrInvalid)
		}

		names = append(names, entry.Name())
	}

	if len(names) == 0 {
		return nil, fmt.Errorf("no sample projects found: %w", os.ErrInvalid)
	}

	slices.SortFunc(names, func(a, b string) int {
		if priority := cmp.Compare(samplePriority(a), samplePriority(b)); priority != 0 {
			return priority
		}

		return cmp.Compare(a, b)
	})

	return names, nil
}

func buildSamples(root string, cdn bool) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}

	names, err := sampleNames(root)
	if err != nil {
		return err
	}

	cards, err := sampleCards(root, names)
	if err != nil {
		return err
	}

	site := filepath.Join(root, "dist", "samples-site")
	if err = os.RemoveAll(site); err != nil {
		return err
	}

	if err = os.MkdirAll(site, 0o755); err != nil {
		return err
	}

	cli := filepath.Join(root, "dist", "karty")
	if err = buildPublishedSamples(root, site, cli, names, cdn); err != nil {
		return err
	}

	if err = guide.Generate(filepath.Join(site, "guide")); err != nil {
		return err
	}

	if err = writeGallery(site, cards); err != nil {
		return err
	}

	return validateSamples(os.DirFS(site))
}

func buildPublishedSamples(root, site, cli string, names []string, cdn bool) error {
	installed := map[string]bool{}
	hostURLs := map[string]string{}

	for _, name := range names {
		sample := filepath.Join(root, "samples", name)

		version, err := sampleVersion(sample)
		if err != nil {
			return err
		}

		if !installed[version] {
			if err = runCLI(root, cli, "sdk", "install", version); err != nil {
				return err
			}

			installed[version] = true
		}

		if cdn && hostURLs[version] == "" {
			hostURLs[version], err = mirrorSampleHost(site, version)
			if err != nil {
				return err
			}
		}

		if err = buildSample(sample, cli, hostURLs[version]); err != nil {
			return err
		}

		target := filepath.Join(site, name)
		if err = os.CopyFS(target, os.DirFS(filepath.Join(sample, "dist", "web"))); err != nil {
			return err
		}

		if err = copySamplePreview(sample, target); err != nil {
			return err
		}

		for _, notice := range []string{"RUNTIME_LICENSE.md", "SDK_LICENSE.md"} {
			if err = downloadNotice(version, notice, filepath.Join(target, notice)); err != nil {
				return err
			}
		}
	}

	return nil
}

// buildSample regenerates configured offline lighting before packaging it. The
// project's public SDK is installed by buildSamples before invoking either CLI command.
func buildSample(directory, cli, cdnURL string) error {
	return buildSampleWithRunner(directory, cli, cdnURL, runCLI)
}

func buildSampleWithRunner(directory, cli, cdnURL string, run func(string, string, ...string) error) error {
	// Source masks are shared; regenerate each level's confined copies when
	// packaging a fresh checkout instead of committing those duplicate files.
	if name := filepath.Base(directory); name == "particle-lab" || name == "hangar" {
		task := "generate-" + name
		if name == "hangar" {
			task += "-effects"
		}

		root := filepath.Dir(filepath.Dir(directory))
		if err := run(root, "mise", "run", task); err != nil {
			return err
		}
	}

	levels, err := sampleBakeLevels(directory)
	if err != nil {
		return err
	}

	for _, name := range levels {
		if err = run(directory, cli, "bake", "--level", name); err != nil {
			return err
		}
	}

	args := []string{"build", "--target", "web"}
	if cdnURL != "" {
		args = append(args, "--cdn", cdnURL)
	}

	return run(directory, cli, args...)
}

func sampleBakeLevels(directory string) ([]string, error) {
	levelsDirectory := filepath.Join(directory, "levels")

	info, err := os.Lstat(levelsDirectory)
	if os.IsNotExist(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("invalid sample levels directory: %s: %w", levelsDirectory, os.ErrInvalid)
	}

	entries, err := os.ReadDir(levelsDirectory)
	if err != nil {
		return nil, err
	}

	var names []string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		path := filepath.Join(directory, "levels", entry.Name(), "level.toml")

		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}

		if err != nil {
			return nil, err
		}

		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("invalid sample level manifest: %s: %w", path, os.ErrInvalid)
		}

		contents, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		var config struct {
			Lightmap struct{ Enabled, Offline bool } `toml:"lightmap"`
			Chunks   struct{ Enabled bool }          `toml:"chunks"`
		}
		if err = toml.Unmarshal(contents, &config); err != nil {
			return nil, fmt.Errorf("parse sample level %s: %w", path, err)
		}

		if config.Lightmap.Enabled && config.Lightmap.Offline && !config.Chunks.Enabled {
			names = append(names, entry.Name())
		}
	}

	return names, nil
}

func sampleVersion(directory string) (string, error) {
	contents, err := os.ReadFile(filepath.Join(directory, "karty.toml"))
	if err != nil {
		return "", err
	}

	var config struct {
		SDK struct{ Version string } `toml:"sdk"`
	}
	if err = toml.Unmarshal(contents, &config); err != nil {
		return "", err
	}

	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(config.SDK.Version) {
		return "", fmt.Errorf("invalid sample SDK version: %w", os.ErrInvalid)
	}

	return config.SDK.Version, nil
}

func runCLI(directory, cli string, args ...string) error {
	command := exec.CommandContext(context.Background(), cli, args...)
	command.Dir = directory
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	return command.Run()
}

func downloadNotice(version, name, output string) error {
	client := &http.Client{Timeout: noticeTimeout}
	url := "https://github.com/karty-game/karty-sdk/releases/download/sdk-v" + version + "/" + name

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("license download %s: HTTP %d: %w", name, response.StatusCode, os.ErrInvalid)
	}

	file, err := os.Create(output)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(file, response.Body)
	closeErr := file.Close()

	if copyErr != nil {
		return copyErr
	}

	return closeErr
}
