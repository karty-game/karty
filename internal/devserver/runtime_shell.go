package devserver

import (
	"context"
	"fmt"
	"slices"

	"github.com/karty-game/karty-sdk/format/worldmaterialgpu"
)

func textureCodecRuntimeFiles() []string {
	return []string{"texture-codec.mjs", "texture-worker.mjs", "transcoder.wasm",
		"licenses/Basis-LICENSE.txt", "licenses/Zstd-LICENSE.txt"}
}

func (snapshot *Snapshot) requiredRuntimeFiles() []string {
	names := []string{"index.html", "wasm_exec.js", "game.kart"}
	if len(snapshot.files["bootstrap.mjs"]) != 0 {
		names = append(names, "bootstrap.mjs", "runtime.mjs", "client.mjs", "karty-host.wasm", "runtime.bin")

		codec := textureCodecRuntimeFiles()
		for _, name := range codec[:3] {
			if _, present := snapshot.files[name]; present {
				names = append(names, codec...)

				break
			}
		}

		return names
	}

	return append(names, "karty.js")
}

func (snapshot *Snapshot) validatePreparedMaterialRuntime(features []string) error {
	if !slices.Contains(features, worldmaterialgpu.Feature) {
		return nil
	}

	for _, name := range textureCodecRuntimeFiles() {
		if len(snapshot.files[name]) == 0 {
			return fmt.Errorf("missing %s: %w", name, ErrSnapshot)
		}
	}

	return nil
}

func (snapshot *Snapshot) runtimeShellFiles(ctx context.Context, wasm []byte) (map[string][]byte, error) {
	files := map[string][]byte{"game.kart": wasm, "index.html": snapshot.files["index.html"]}
	if len(snapshot.files["bootstrap.mjs"]) != 0 {
		return files, nil
	}

	clientHash, err := digestContext(ctx, snapshot.files["game.kart"])
	if err != nil {
		return nil, err
	}

	launcherHash, err := digestContext(ctx, snapshot.files["karty.js"])
	if err != nil {
		return nil, err
	}

	files["karty.js"], err = normalizeShellHash(ctx, snapshot.files["karty.js"], "game.kart", clientHash)
	if err != nil {
		return nil, err
	}

	files["index.html"], err = normalizeShellHash(ctx, snapshot.files["index.html"], "karty.js", launcherHash)

	return files, err
}
