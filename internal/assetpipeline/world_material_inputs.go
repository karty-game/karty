package assetpipeline

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty/internal/sdk"
)

// Normalize each source independently before extending coverage. Hidden RGB
// must not influence the luminance statistics or create geometry at mask edges.
func materialGeometrySource(ctx context.Context, tile *image.NRGBA, rect worldmaterial.Rect) (SourceSnapshot, error) {
	geometry, err := normalizeMaterialGeometryInput(ctx, tile)
	if err != nil {
		return SourceSnapshot{}, err
	}

	geometry, err = materialGenerationInput(ctx, worldmaterial.Layout{Materials: []worldmaterial.Rect{rect}}, geometry)
	if err != nil {
		return SourceSnapshot{}, err
	}

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, geometry); err != nil {
		return SourceSnapshot{}, err
	}

	return SnapshotBytes(encoded.Bytes())
}

// Reflectance detection retains the original colors and brightness. Geometry
// normalization is not evidence that a painted surface is metallic or smooth.
func generateMaterialReflectance(ctx context.Context, manifest sdk.Manifest, directory string,
	source SourceSnapshot, bounds image.Rectangle, options MaterializeOptions, execute materializeExecutor) (*image.NRGBA, error) {
	directory = filepath.Join(directory, "reflectance")
	if err := os.Mkdir(directory, 0o700); err != nil {
		return nil, err
	}

	inputPath := filepath.Join(directory, "atlas.png")
	if err := os.WriteFile(inputPath, source.Bytes(), 0o600); err != nil {
		return nil, err
	}

	arguments := materializeArguments(inputPath, directory, options)
	arguments[4] = "metallic,smoothness"

	if err := execute(ctx, manifest, directory, arguments); err != nil {
		return nil, err
	}

	reflectance := newNeutralReflectance(bounds)
	for _, name := range []string{"metallic", "smoothness"} {
		if err := mergeMaterialMap(ctx, directory, name, reflectance); err != nil {
			return nil, err
		}
	}

	// The pinned tool adds specular evidence after multiplying its detector by
	// metallic_scale. Honor explicit nonmetal classification; keep smoothness.
	if options.MetallicScale != nil && *options.MetallicScale == 0 {
		for row := range bounds.Dy() {
			if err := ctx.Err(); err != nil {
				return nil, err
			}

			for column := range bounds.Dx() {
				reflectance.Pix[reflectance.PixOffset(column, row)] = 0
			}
		}
	}

	return reflectance, nil
}
