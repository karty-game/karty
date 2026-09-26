package levelbuild_test

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-ui/schema"
	"github.com/karty-game/karty/internal/levelbuild"
)

func TestBuildAllProducesDeterministicSortedCartridges(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	writeLevel(t, directory, "z", "levels.z", "z-data")
	writeLevel(t, directory, "a", "levels.a", "a-data")

	first, err := levelbuild.BuildAll(directory)
	if err != nil {
		t.Fatal(err)
	}

	second, err := levelbuild.BuildAll(directory)
	if err != nil {
		t.Fatal(err)
	}

	if len(first) != 2 || first[0].Name != "levels.a" || first[1].Name != "levels.z" {
		t.Fatalf("BuildAll() = %+v", first)
	}

	for index := range first {
		if first[index].ContentSHA256 != second[index].ContentSHA256 || string(first[index].Bytes) != string(second[index].Bytes) {
			t.Fatal("BuildAll() output is not deterministic")
		}
	}
}

func TestBuildAllEmbedsDeterministicLevelTextures(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()

	levelDirectory := filepath.Join(directory, "levels", "textured")
	if err := os.MkdirAll(filepath.Join(levelDirectory, "assets"), 0o750); err != nil {
		t.Fatal(err)
	}

	png, err := base64.StdEncoding.DecodeString(
		"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=",
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(levelDirectory, "assets", "pixel.png"), png, 0o600); err != nil {
		t.Fatal(err)
	}

	manifest := "[level]\nname='levels.textured'\nkind='level'\n[[textures]]\nname='pixel'\nsource='assets/pixel.png'\n"
	if err := os.WriteFile(filepath.Join(levelDirectory, "level.toml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	artifacts, err := levelbuild.BuildAll(directory)
	if err != nil {
		t.Fatal(err)
	}

	envelope := unwrapLevelModule(t, artifacts[0].Bytes)

	decoded, err := level.Decode(envelope)
	if err != nil {
		t.Fatal(err)
	}

	texture, _, found := decoded.Read(level.TextureEntryName(1), 0, level.MaxEntrySize)
	if !found || string(texture) != string(png) {
		t.Fatal("embedded texture does not match its source")
	}

	var metadata struct {
		Level struct {
			Name            string `json:"name"`
			Kind            string `json:"kind"`
			EnvelopeVersion uint16 `json:"envelopeVersion"`
		} `json:"kartyLevel"`
		Textures []struct {
			ID   uint32 `json:"id"`
			Name string `json:"name"`
		} `json:"kartyTextures"`
	}
	if err := json.Unmarshal(decoded.Metadata, &metadata); err != nil || metadata.Level.Name != "levels.textured" ||
		metadata.Level.Kind != "level" || metadata.Level.EnvelopeVersion != level.EnvelopeVersion || len(metadata.Textures) != 1 ||
		metadata.Textures[0].ID != 1 || metadata.Textures[0].Name != "pixel" {
		t.Fatalf("metadata = %s, error = %v", decoded.Metadata, err)
	}
}

func TestBuildAllResolvesLevelThemeImage(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	directory := filepath.Join(root, "levels", "themed")
	if err := os.MkdirAll(directory, 0o750); err != nil {
		t.Fatal(err)
	}

	var texture bytes.Buffer
	if err := png.Encode(&texture, image.NewNRGBA(image.Rect(0, 0, 3, 3))); err != nil {
		t.Fatal(err)
	}

	for name, contents := range map[string][]byte{
		"panel.png": texture.Bytes(),
		"theme.toml": []byte(`extends = "dark"
[images.panel]
asset = "panel"
slice = [1, 1, 1, 1]
`),
		"hud.ui": []byte(`kartui Hud() { <panel class="frame"><label>HUD</label></panel> }
style { .frame { background-image: theme.images.panel; } }`),
		"level.toml": []byte(`[level]
name = "levels.themed"
kind = "level"
[theme]
source = "theme.toml"
[[textures]]
name = "panel"
source = "panel.png"
[[ui]]
name = "ui.hud"
source = "hud.ui"
`),
	} {
		if err := os.WriteFile(filepath.Join(directory, name), contents, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	artifacts, err := levelbuild.BuildAllWithTheme(root, 4, "")
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := level.Decode(unwrapLevelModule(t, artifacts[0].Bytes))
	if err != nil {
		t.Fatal(err)
	}

	encoded, _, found := decoded.Read(ui.AssetPrefix+"ui.hud", 0, level.MaxEntrySize)
	if !found {
		t.Fatal("compiled UI asset is missing")
	}

	template, err := ui.DecodeComposition(encoded)
	if err != nil {
		t.Fatal(err)
	}

	image := template.Panel.BackgroundImage
	if template.Version != 4 || image.Scope != ui.ImageScopeLevel || image.AssetID != 1 || image.Name != "" {
		t.Fatalf("level image = %+v", image)
	}
}

func unwrapLevelModule(t *testing.T, module []byte) []byte {
	t.Helper()

	offset := bytes.Index(module, []byte("KTYL"))
	if offset < 0 || len(module)-offset < level.HeaderSize {
		t.Fatal("level envelope is missing")
	}

	length := int(binary.LittleEndian.Uint32(module[offset+24 : offset+28]))
	if length < level.HeaderSize || length > len(module)-offset {
		t.Fatal("level envelope length is invalid")
	}

	return module[offset : offset+length]
}

func TestBuildAllRejectsEscapingDataSource(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "outside"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	levelDirectory := filepath.Join(directory, "levels", "bad")
	if err := os.MkdirAll(levelDirectory, 0o750); err != nil {
		t.Fatal(err)
	}

	manifest := "[level]\nname='levels.bad'\nkind='level'\n[[data]]\nname='map'\nsource='../../outside'\n"
	if err := os.WriteFile(filepath.Join(levelDirectory, "level.toml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := levelbuild.BuildAll(directory); err == nil {
		t.Fatal("BuildAll() accepted an escaping source")
	}
}

func writeLevel(t *testing.T, root, directoryName, name, data string) {
	t.Helper()

	directory := filepath.Join(root, "levels", directoryName)
	if err := os.MkdirAll(filepath.Join(directory, "data"), 0o750); err != nil {
		t.Fatal(err)
	}

	manifest := "[level]\nname='" + name + "'\nkind='level'\n[[data]]\nname='map'\nsource='data/map.bin'\n"
	if err := os.WriteFile(filepath.Join(directory, "level.toml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(directory, "data", "map.bin"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}
