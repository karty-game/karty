package build

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/karty-game/karty/internal/brand"
	"github.com/karty-game/karty/internal/contextio"
	"github.com/karty-game/karty/internal/levelbuild"
	"github.com/karty-game/karty/internal/toolchain"
	"github.com/pelletier/go-toml/v2"
)

func stageHostBundle(ctx context.Context, bundle *hostBundle, distribution, raw, artifact string,
	levels []levelbuild.Artifact, target, platform, title string,
) error {
	if target == "" {
		target = "native"
	}

	directory := filepath.Join(distribution, target)
	if platform != "" {
		directory = filepath.Join(directory, platform)
	}

	if err := os.MkdirAll(directory, 0o750); err != nil {
		return err
	}

	if err := stageTargetAssets(ctx, directory, artifact); err != nil {
		return err
	}

	if err := stageLevels(ctx, directory, levels, raw); err != nil {
		return err
	}

	if err := stageVideoFiles(ctx, directory, artifact, raw); err != nil {
		return err
	}

	if err := stageAudioStreamFiles(ctx, directory, artifact, raw); err != nil {
		return err
	}

	if err := stageBundleFiles(ctx, bundle, directory, platform); err != nil {
		return err
	}

	if target == "web" {
		return stageWebGPUHostShell(ctx, directory, title, bundle)
	}

	return nil
}

func stageBundleDescriptor(ctx context.Context, bundle *hostBundle, directory, platform string) error {
	staged := *bundle

	staged.Files = make(map[string]hostBundleFile, len(bundle.Files))
	maps.Copy(staged.Files, bundle.Files)

	if bundle.Target == "web" {
		delete(staged.Files, "runtime.bin")
	}

	if bundle.Target != "web" {
		staged.Entrypoint = toolchain.NativeHostName(platform)
		delete(staged.Files, bundle.Entrypoint)
		staged.Files[staged.Entrypoint] = bundle.Files[bundle.Entrypoint]
	}

	contents, err := toml.Marshal(staged)
	if err != nil {
		return err
	}

	return copyToFile(ctx, bytes.NewReader(contents), filepath.Join(directory, hostBundleName), 0o600)
}

func stageBundleFiles(ctx context.Context, bundle *hostBundle, directory, platform string) error {
	root, err := os.OpenRoot(bundle.Directory)
	if err != nil {
		return err
	}
	defer root.Close()

	names := make([]string, 0, len(bundle.Files))
	for name := range bundle.Files {
		if bundle.Target == "web" && name == "runtime.bin" {
			continue
		}

		names = append(names, name)
	}

	slices.Sort(names)

	for _, name := range names {
		if err := verifyBundleFile(ctx, root, name, bundle.Files[name]); err != nil {
			return err
		}

		destination := name
		mode := os.FileMode(0o600)

		if bundle.Target != "web" && name == bundle.Entrypoint {
			destination = toolchain.NativeHostName(platform)
			mode = 0o755
		}

		if err := ensureBundleOutput(directory, destination); err != nil {
			return err
		}

		if err := copyBundleFile(
			ctx,
			root,
			name,
			bundle.Files[name],
			filepath.Join(directory, filepath.FromSlash(destination)),
			mode,
		); err != nil {
			return err
		}
	}

	if bundle.Target == "web" {
		if err := os.Remove(filepath.Join(directory, "runtime.bin")); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	return stageBundleDescriptor(ctx, bundle, directory, platform)
}

func copyBundleFile(ctx context.Context, root *os.Root, name string, entry hostBundleFile, destination string, mode os.FileMode) error {
	input, err := root.Open(name)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.CreateTemp(filepath.Dir(destination), ".host-bundle-*")
	if err != nil {
		return err
	}
	defer os.Remove(output.Name())

	digest := sha256.New()
	size, copyErr := contextio.Copy(ctx, io.MultiWriter(output, digest), io.LimitReader(input, entry.Size+1))
	modeErr := output.Chmod(mode)
	closeErr := output.Close()

	if copyErr != nil {
		return copyErr
	}

	if modeErr != nil {
		return modeErr
	}

	if closeErr != nil {
		return closeErr
	}

	if size != entry.Size || hex.EncodeToString(digest.Sum(nil)) != entry.SHA256 {
		return fmt.Errorf("host bundle file changed during staging: %w", os.ErrInvalid)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	return os.Rename(output.Name(), destination)
}

func ensureBundleOutput(directory, name string) error {
	current := directory

	for _, part := range append([]string{""}, strings.Split(filepath.ToSlash(filepath.Dir(name)), "/")...) {
		if part == "." {
			continue
		}

		current = filepath.Join(current, part)
		if err := os.Mkdir(current, 0o750); err != nil && !os.IsExist(err) {
			return err
		}

		info, err := os.Lstat(current)
		if err != nil {
			return err
		}

		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("host bundle output must use real directories: %w", os.ErrInvalid)
		}
	}

	return nil
}

func stageWebGPUHostShell(ctx context.Context, directory, title string, bundle *hostBundle) error {
	for _, name := range []string{"karty.js", "renderer-debug.js", "catalog.json"} {
		if err := os.Remove(filepath.Join(directory, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	bootstrap, status, err := stageWebDelivery(ctx, directory)
	if err != nil {
		return err
	}

	index := strings.NewReplacer(
		"@@GAME_TITLE@@", html.EscapeString(title), "@@FAVICON_URL@@", string(brand.FaviconURL()),
		"@@WASM_SHA256@@", bundle.Files[bundle.Entrypoint].SHA256, "@@WEB_BOOTSTRAP_HASH@@", bootstrap, "@@WEB_STATUS_HASH@@", status,
	).Replace(webGPUIndexTemplate)
	if err := contextio.WriteFile(ctx, filepath.Join(directory, "index.html"), []byte(index), 0o600); err != nil {
		return fmt.Errorf("write WebGPU shell: %w", err)
	}

	startup := []string{"index.html", "game.kart", "karty-web.js"}

	for name := range bundle.Files {
		if name != bundle.Entrypoint && name != "runtime.bin" {
			startup = append(startup, name)
		}
	}

	slices.Sort(startup)

	return stageWebRelease(ctx, directory, startup...)
}
