package main

import (
	"crypto/sha256"
	"fmt"
	"image"
	"io/fs"
	"path"

	"github.com/karty-game/tiled"
	"github.com/karty-game/tiled/assets"
)

// The shared importer resolves every source rectangle. This project callback
// retains prepared-image aliases and the existing stable atlas-crop filenames.
func importTiledImages(fsys fs.FS, level *tiled.Map, emit func(string, *image.NRGBA)) (map[string]string, error) {
	return assets.Import(fsys, level, func(input assets.Input) (string, error) {
		if input.Rectangle == input.SourceBounds {
			candidate := path.Base(input.Source)
			candidate = candidate[:len(candidate)-len(path.Ext(candidate))]
			if input.Source == target+"/"+candidate+".png" && textures[candidate] != nil {
				return candidate, nil
			}
		}
		name := fmt.Sprintf("tiled-%x", sha256.Sum256([]byte(input.Key)))[:18]
		emit(name, input.Pixels)
		return name, nil
	})
}
