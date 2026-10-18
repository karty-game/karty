package devserver

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/cartridge"
	"github.com/karty-game/karty-ui/schema"
)

func uiFixture(t *testing.T, templates, assets []cartridge.Asset) *Snapshot {
	t.Helper()
	root := t.TempDir()
	fixtureBuild(t, root, []byte("level"), []byte("code"), nil)

	wasm, err := os.ReadFile(filepath.Join(root, "game.kart"))
	if err != nil {
		t.Fatal(err)
	}

	for _, section := range []struct {
		name    string
		entries []cartridge.Asset
	}{{ui.SectionName, templates}, {cartridge.AssetSectionName, assets}} {
		if section.entries == nil {
			continue
		}

		payload, err := cartridge.EncodeAssets(section.entries)
		if err != nil {
			t.Fatal(err)
		}

		wasm, err = cartridge.EmbedSection(wasm, section.name, payload)
		if err != nil {
			t.Fatal(err)
		}
	}

	launcher := []byte(`fetch("game.kart?v=` + digest(wasm)[:shellHashLength] + `")`)

	index := []byte(`<html><head></head><script src="karty.js?v=` + digest(launcher)[:shellHashLength] + `"></script></html>`)
	for name, data := range map[string][]byte{"game.kart": wasm, "karty.js": launcher, "index.html": index} {
		if err := os.WriteFile(filepath.Join(root, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	return captureFixture(t, root, 1)
}

func TestUIAndFontsChangeResourcesWhileExecutableIdentityStays(t *testing.T) {
	t.Parallel()

	texture := cartridge.Asset{Name: "texture", Bytes: []byte("texture data")}
	first := uiFixture(t, []cartridge.Asset{{Name: "hud", Bytes: []byte("old UI")}},
		[]cartridge.Asset{texture, {Name: ui.FontAssetPrefix + "body", Bytes: []byte("old font")}})

	second := uiFixture(t, []cartridge.Asset{{Name: "hud", Bytes: []byte("edited UI")}},
		[]cartridge.Asset{texture, {Name: ui.FontAssetPrefix + "body", Bytes: []byte("new font")}})
	if first.Generation.RuntimeIdentity != second.Generation.RuntimeIdentity ||
		first.Generation.UI.ContentSHA256 == second.Generation.UI.ContentSHA256 || first.Generation.ID == second.Generation.ID {
		t.Fatal("presentation update must preserve runtime and change resource identity")
	}

	data := second.files[second.Generation.UI.Artifact]

	templates, fonts := decodeUIFixture(t, data)
	if len(templates) != 1 || len(fonts) != 1 || fonts[0].Name != ui.FontAssetPrefix+"body" ||
		!bytes.Equal(templates[0].Bytes, []byte("edited UI")) || !bytes.Equal(fonts[0].Bytes, []byte("new font")) {
		t.Fatal("UI transport omitted or changed resource bytes")
	}

	if uint64(len(data)) != second.Generation.UI.Size || digest(data) != second.Generation.UI.ContentSHA256 {
		t.Fatal("UI resource descriptor does not verify payload")
	}

	changedAssets := []cartridge.Asset{
		{Name: "texture", Bytes: []byte("changed texture")},
		{Name: ui.FontAssetPrefix + "body", Bytes: []byte("old font")},
	}

	changedTexture := uiFixture(t, []cartridge.Asset{{Name: "hud", Bytes: []byte("old UI")}}, changedAssets)
	if changedTexture.Generation.RuntimeIdentity == first.Generation.RuntimeIdentity {
		t.Fatal("nonfont asset changes must restart the runtime")
	}
}

func TestEmptyPresentationAndFontAddRemoveShareRuntimeIdentity(t *testing.T) {
	t.Parallel()
	absent := uiFixture(t, nil, nil)
	empty := uiFixture(t, []cartridge.Asset{}, []cartridge.Asset{})

	font := uiFixture(t, []cartridge.Asset{}, []cartridge.Asset{{Name: ui.FontAssetPrefix + "body", Bytes: []byte("font")}})
	for _, other := range []*Snapshot{empty, font} {
		if absent.Generation.RuntimeIdentity != other.Generation.RuntimeIdentity {
			t.Fatal("adding/removing presentation-only sections changed executable identity")
		}
	}

	data := absent.files[absent.Generation.UI.Artifact]

	templates, fonts := decodeUIFixture(t, data)
	if len(data) != 44 || len(templates) != 0 || len(fonts) != 0 {
		t.Fatal("empty UI must include two empty SDK asset bundles")
	}

	if !strings.HasPrefix(font.Generation.UI.Artifact, "__karty_dev/ui/") {
		t.Fatal("UI artifact is not generation-owned")
	}
}

func TestUIPayloadOnlyAllowsReservedFontRoles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	fixtureBuild(t, root, []byte("level"), []byte("code"), nil)

	wasm, err := os.ReadFile(filepath.Join(root, "game.kart"))
	if err != nil {
		t.Fatal(err)
	}

	payload, err := cartridge.EncodeAssets([]cartridge.Asset{{Name: ui.FontAssetPrefix + "unknown", Bytes: []byte("font")}})
	if err != nil {
		t.Fatal(err)
	}

	wasm, err = cartridge.EmbedSection(wasm, cartridge.AssetSectionName, payload)
	if err != nil {
		t.Fatal(err)
	}
	//nolint:gosec // root is a fresh t.TempDir and the basename is fixed.
	if err := os.WriteFile(filepath.Join(root, "game.kart"), wasm, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Capture(t.Context(), root, 1); err == nil {
		t.Fatal("unsupported font role accepted for live UI")
	}
}

func decodeUIFixture(t *testing.T, data []byte) ([]cartridge.Asset, []cartridge.Asset) {
	t.Helper()

	if len(data) < 12 || string(data[:4]) != "KDU1" {
		t.Fatal("UI resource header is invalid")
	}

	templatesLength := int(binary.LittleEndian.Uint32(data[4:8]))

	fontsLength := int(binary.LittleEndian.Uint32(data[8:12]))
	if len(data) != 12+templatesLength+fontsLength {
		t.Fatal("UI resource lengths are invalid")
	}

	templates, err := cartridge.DecodeAssets(data[12 : 12+templatesLength])
	if err != nil {
		t.Fatal(err)
	}

	fonts, err := cartridge.DecodeAssets(data[12+templatesLength:])
	if err != nil {
		t.Fatal(err)
	}

	return templates, fonts
}
