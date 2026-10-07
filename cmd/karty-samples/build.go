package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const noticeTimeout = 60 * time.Second

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

	return names, nil
}

func buildSamples(root string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}

	names, err := sampleNames(root)
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
	installed := map[string]bool{}

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

		if err = runCLI(sample, cli, "build", "--target", "web"); err != nil {
			return err
		}

		target := filepath.Join(site, name)
		if err = os.CopyFS(target, os.DirFS(filepath.Join(sample, "dist", "web"))); err != nil {
			return err
		}

		for _, notice := range []string{"RUNTIME_LICENSE.md", "SDK_LICENSE.md"} {
			if err = downloadNotice(version, notice, filepath.Join(target, notice)); err != nil {
				return err
			}
		}
	}

	var links strings.Builder

	for _, name := range names {
		fmt.Fprintf(&links, "<li><a href=\"%s/\">%s</a></li>", name, name)
	}

	return writeIndex(site, "Games and interfaces built with Karty.", links.String())
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
