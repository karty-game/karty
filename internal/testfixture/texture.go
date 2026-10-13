package testfixture

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
)

// PlayerTexture supplies the sprite used by asset and runtime probes independently
// of the published starter template's vector-only game.
func PlayerTexture(directory string) error {
	texture := image.NewRGBA(image.Rect(0, 0, 32, 32))
	draw.Draw(texture, texture.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, texture); err != nil {
		return err
	}

	textures := filepath.Join(directory, "assets", "textures")
	if err := os.MkdirAll(textures, 0o750); err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(textures, "player.png"), encoded.Bytes(), 0o600)
}
