// karty-materialize-check prepares the released tool and graphics driver before
// the timed native atlas regression. Its temporary material cache is discarded.
package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"os"
	"time"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

const (
	materialID       = 9
	sourceRed        = 200
	opaqueAlpha      = 255
	normalStrength   = 0.8
	ambientOcclusion = 0.5
)

func main() {
	if err := prepare(); err != nil {
		fmt.Fprintln(os.Stderr, "prepare Materialize graphics:", err)
		os.Exit(1)
	}
}

func prepare() error {
	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		return err
	}

	root, err := os.MkdirTemp("", "karty-materialize-prepare-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)

	texture := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for y := range texture.Bounds().Dy() {
		for x := range texture.Bounds().Dx() {
			texture.SetNRGBA(x, y, color.NRGBA{R: sourceRed, A: opaqueAlpha})
		}
	}

	encoded, _, err := qoi.Encode(texture, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceSRGB})
	if err != nil {
		return err
	}

	document := world.Document{Sectors: []world.Sector{{FloorMaterial: materialID, CeilingMaterial: materialID}}}
	normal, height, metallic, ao := normalStrength, 1.0, 1.0, ambientOcclusion
	options := map[uint32]assetpipeline.MaterializeOptions{
		materialID: {Preset: "metal", NormalStrength: &normal, HeightContrast: &height, MetallicScale: &metallic, AODepthScale: &ao},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	_, _, err = assetpipeline.ProcessWorldMaterials(ctx, root, manifest, document, map[uint32][]byte{materialID: encoded}, options)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, "Prepared SDK-pinned Materialize and native graphics; discarded preparation atlas cache.")

	return nil
}
