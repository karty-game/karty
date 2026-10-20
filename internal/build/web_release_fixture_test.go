package build

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// TestWebDeliveryFixtures emits small real staged web shells for the maintained
// browser delivery check. It uses only synthetic public fixture bytes; ordinary
// unit test runs do not create output outside t.TempDir.
func TestWebDeliveryFixtures(t *testing.T) {
	t.Parallel()

	destination := os.Getenv("KARTY_WEB_DELIVERY_FIXTURES")

	if destination == "" {
		t.Skip("KARTY_WEB_DELIVERY_FIXTURES is not set")
	}

	for _, version := range []string{"a", "b"} {
		root := filepath.Join(destination, version)
		webReleaseTestWrite(t, root, "game.kart", webReleaseFixtureWASM("cartridge-"+version, 64))
		webReleaseTestWrite(t, root, "karty-host.wasm", webReleaseFixtureWASM("shared-host", 64))
		webReleaseTestContent(t, root, ".kld", webReleaseFixtureWASM("shared-level", 65537))

		size := 131099

		if version == "b" {
			size = 131113
		}

		webReleaseTestContent(t, root, ".kld", webReleaseFixtureWASM("level-"+version, size))

		runtime := filepath.Join(t.TempDir(), "wasm_exec.js")

		if err := os.WriteFile(runtime, []byte("globalThis.kartyDeliveryFixtureRuntime = true;\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		if err := stageWebShell(t.Context(), root, runtime, "Delivery fixture "+version, false, nil); err != nil {
			t.Fatal(err)
		}

		if err := stageWebRelease(t.Context(), root, nil); err != nil {
			t.Fatal(err)
		}
	}
}

func webReleaseFixtureWASM(label string, size int) []byte {
	// An unknown custom section makes each fixture a valid WASM module while
	// varying bytes across SHA256 blocks and releases without executable code.
	section := make([]byte, 1, 1+len(label)+size)
	section[0] = byte(len(label))
	section = append(section, label...)
	section = append(section, make([]byte, size)...)

	for index := len(label) + 1; index < len(section); index++ {
		section[index] = byte(index*31 + len(label))
	}

	contents := []byte{'\x00', 'a', 's', 'm', 1, 0, 0, 0, 0}

	contents = binary.AppendUvarint(contents, uint64(len(section)))

	return append(contents, section...)
}
