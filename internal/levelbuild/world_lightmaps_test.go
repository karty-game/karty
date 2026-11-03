package levelbuild

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty-sdk/format/level"
)

func enableFixtureLightmap(t *testing.T, root, settings string) {
	t.Helper()

	path := filepath.Join(root, "levels", "lighting", "level.toml")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// #nosec G703 -- root is an isolated test fixture from t.TempDir.
	if err := os.WriteFile(path, append(data, []byte("\n[lightmap]\n"+settings)...), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Cache lifecycle assertions inspect the SDK-validated canonical passive module.
// Dedicated packaging tests execute the root and chunk modules in real WASM.
func decodePassiveEnvelope(tb testing.TB, module []byte) level.Envelope {
	tb.Helper()

	envelope, err := level.DecodePassiveModule(module)
	if err != nil {
		tb.Fatal(err)
	}

	return envelope
}

func executeLightmapEnvelope(tb testing.TB, module []byte) level.Envelope {
	tb.Helper()

	node, err := exec.LookPath("node")
	if err != nil {
		tb.Skip("actual WASM execution needs pinned Node")
	}

	path := filepath.Join(tb.TempDir(), "lightmap.kld")
	// #nosec G703 -- path is a fixed filename beneath tb.TempDir.
	if err := os.WriteFile(path, module, 0o600); err != nil {
		tb.Fatal(err)
	}

	const script = `import fs from 'node:fs'; const {instance}=await WebAssembly.instantiate(fs.readFileSync(process.argv[1])); const e=instance.exports; if(e.karty_level_abi_version()!==2)throw new Error('ABI');process.stdout.write(Buffer.from(e.memory.buffer,e.karty_level_data_pointer(),e.karty_level_data_length()));`

	raw, err := exec.CommandContext(tb.Context(), node, "--input-type=module", "-e", script, path).Output()
	if err != nil {
		tb.Fatal(err)
	}

	envelope, err := level.Decode(raw)
	if err != nil {
		tb.Fatal(err)
	}

	return envelope
}
