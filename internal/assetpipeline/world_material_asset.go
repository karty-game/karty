package assetpipeline

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty/internal/sdk"
)

const (
	worldMaterialAssetProcessor = "world-material-asset@4"
	materialTileLengthBytes     = 4
	materialTileHeaderBytes     = 2 * materialTileLengthBytes
)

// WorldMaterialColorOnly reports assets whose generated material data is unused.
func WorldMaterialColorOnly(document world.Document, id uint32) bool {
	return secondaryOnlyMaterials(document)[id]
}

// MaterializeOptions are generation overrides, separate from runtime strengths.
// Omitted numeric fields retain the preset value; explicit values must be finite
// and within [0,16]. Empty Preset is equivalent to "default".
//
//nolint:tagliatelle // Recipe keys preserve the documented Materialize authoring fields.
type MaterializeOptions struct {
	Preset         string   `json:"preset,omitempty"          toml:"preset"`
	NormalStrength *float64 `json:"normal_strength,omitempty" toml:"normal_strength"`
	HeightContrast *float64 `json:"height_contrast,omitempty" toml:"height_contrast"`
	MetallicScale  *float64 `json:"metallic_scale,omitempty"  toml:"metallic_scale"`
	AODepthScale   *float64 `json:"ao_depth_scale,omitempty"  toml:"ao_depth_scale"`
}

func (options MaterializeOptions) Validate() error {
	if !slices.Contains([]string{"", "default", "auto", "skin", "floor", "metal", "fabric", "wood",
		"stone", "concrete", "leather", "marble", "sand", "foliage", "plaster", "asphalt", "brick",
		"ice", "snow", "lava", "water"}, options.Preset) {
		return fmt.Errorf("unknown Materialize preset %q: %w", options.Preset, ErrCacheRecipe)
	}

	for _, item := range options.overrides() {
		if item.value != nil && (math.IsNaN(*item.value) || math.IsInf(*item.value, 0) || *item.value < 0 || *item.value > 16) {
			return fmt.Errorf("materialize %s must be finite within [0,16]: %w", item.flag, ErrCacheRecipe)
		}
	}

	return nil
}

type materializeOverride struct {
	flag  string
	value *float64
}

func (options MaterializeOptions) overrides() []materializeOverride {
	return []materializeOverride{
		{"--normal-strength", options.NormalStrength}, {"--height-contrast", options.HeightContrast},
		{"--metallic-scale", options.MetallicScale}, {"--ao-depth-scale", options.AODepthScale},
	}
}

func materializeArguments(inputPath, directory string, values ...MaterializeOptions) []string {
	var options MaterializeOptions
	if len(values) > 0 {
		options = values[0]
	}

	preset := options.Preset
	if preset == "" {
		preset = "default"
	}

	arguments := []string{inputPath, "--output", directory, "--only", "height,normal,ao",
		"--normal-format", "opengl", "--format", "png", "--preset", preset}

	for _, item := range options.overrides() {
		if item.value != nil {
			arguments = append(arguments, item.flag, strconv.FormatFloat(*item.value, 'g', -1, 64))
		}
	}

	return arguments
}

// This capability is a generation policy: older SDKs retain their three-map recipe.
func materialReflectanceEnabled(manifest sdk.Manifest) bool {
	return slices.Contains(manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialReflectanceV1)
}

func materializeManifestArguments(manifest sdk.Manifest, inputPath, directory string, options MaterializeOptions) []string {
	arguments := materializeArguments(inputPath, directory, options)
	if materialReflectanceEnabled(manifest) {
		arguments[4] = "height,normal,ao,metallic,smoothness"
	}

	return arguments
}

type materialTile struct {
	Data        *image.NRGBA
	Reflectance *image.NRGBA
}

func cachedMaterialTile(ctx context.Context, projectRoot string, manifest sdk.Manifest,
	albedo *image.NRGBA, rect worldmaterial.Rect, raw []byte, options MaterializeOptions,
	platform, checksum string, execute materializeExecutor) (materialTile, error) {
	// Crop only the normalized interior: atlas position, gutters and IDs do not
	// enter this cache's identity or the native tool's neighborhood analysis.
	tile := image.NewNRGBA(image.Rect(0, 0, rect.Width, rect.Height))
	for y := range rect.Height {
		copy(tile.Pix[tile.PixOffset(0, y):][:rect.Width*materialPixelBytes],
			albedo.Pix[albedo.PixOffset(rect.X, rect.Y+y):][:rect.Width*materialPixelBytes])
	}

	tileRect := rect
	tileRect.MaterialID, tileRect.X, tileRect.Y, tileRect.Gutter = 0, 0, 0, 0
	tileRect.Strengths = nil

	generation, err := materialGenerationInput(ctx, worldmaterial.Layout{Materials: []worldmaterial.Rect{tileRect}}, tile)
	if err != nil {
		return materialTile{}, err
	}

	var input bytes.Buffer
	if err := png.Encode(&input, generation); err != nil {
		return materialTile{}, err
	}

	source, err := SnapshotBytes(input.Bytes())
	if err != nil {
		return materialTile{}, err
	}

	geometry, err := materialGeometrySource(ctx, tile, tileRect)
	if err != nil {
		return materialTile{}, err
	}

	digest := sha256.Sum256(raw)

	recipe, err := NewRecipe("world-material-asset", worldMaterialAssetProcessor, []Revision{
		{Name: "normalizer", Version: "2"}, {Name: "merge", Version: "1"},
		{Name: "geometry-luminance", Version: "1"}, {Name: "generation-passes", Version: "2"},
		{Name: "coverage-extension", Version: "1"}, {Name: "qoi", Version: "1"},
		{Name: "materialize", Version: manifest.Tools.MaterializeRevision},
		{Name: "tool-version", Version: manifest.Tools.Materialize}, {Name: "archive", Version: checksum},
	}, struct {
		Source    string   `json:"source"`
		Platform  string   `json:"platform"`
		Arguments []string `json:"arguments"`
	}{hex.EncodeToString(digest[:]), platform, materializeManifestArguments(manifest, "atlas.png", ".", options)})
	if err != nil {
		return materialTile{}, err
	}

	artifact, err := NewProjectCache(projectRoot).Resolve(ctx, source, recipe,
		func(ctx context.Context, source SourceSnapshot, _ Recipe) (Processed, error) {
			return generateMaterialTile(ctx, manifest, source, geometry, tile.Bounds(), options, execute)
		})
	if err != nil {
		return materialTile{}, err
	}

	return loadMaterialTile(ctx, artifact, tile.Bounds(), materialReflectanceEnabled(manifest))
}

func generateMaterialTile(ctx context.Context, manifest sdk.Manifest, source, geometry SourceSnapshot,
	bounds image.Rectangle, options MaterializeOptions, execute materializeExecutor) (Processed, error) {
	directory, err := os.MkdirTemp("", "karty-material-asset-")
	if err != nil {
		return Processed{}, err
	}
	defer os.RemoveAll(directory)

	inputPath := filepath.Join(directory, "atlas.png")
	if err := os.WriteFile(inputPath, geometry.Bytes(), 0o600); err != nil {
		return Processed{}, err
	}

	if err := execute(ctx, manifest, directory, materializeArguments(inputPath, directory, options)); err != nil {
		return Processed{}, err
	}

	data := image.NewNRGBA(bounds)
	for _, name := range []string{"normal", "height", "ao"} {
		if err := mergeMaterialMap(ctx, directory, name, data); err != nil {
			return Processed{}, err
		}
	}

	var reflectance *image.NRGBA
	if materialReflectanceEnabled(manifest) {
		reflectance, err = generateMaterialReflectance(ctx, manifest, directory, source, bounds, options, execute)
		if err != nil {
			return Processed{}, err
		}
	}

	payload, err := encodeMaterialTile(materialTile{Data: data, Reflectance: reflectance})
	if err != nil {
		return Processed{}, err
	}

	return Processed{Encoding: worldMaterialAssetProcessor, Payload: payload}, nil
}

// Two length-prefixed linear QOI payloads retain all generated channels per asset.
// The second length is zero for SDKs without the reflectance capability.
func encodeMaterialTile(tile materialTile) ([]byte, error) {
	var header [materialTileHeaderBytes]byte

	payload := header[:]

	for index, pixels := range []*image.NRGBA{tile.Data, tile.Reflectance} {
		if pixels == nil {
			continue
		}

		encoded, _, err := qoi.Encode(pixels, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceLinear})
		if err != nil {
			return nil, err
		}

		if len(encoded) > MaxCachePayloadBytes-len(payload) {
			return nil, ErrCacheOutput
		}

		binary.BigEndian.PutUint32(payload[index*materialTileLengthBytes:(index+1)*materialTileLengthBytes], uint32(len(encoded)))
		payload = append(payload, encoded...)
	}

	return payload, nil
}

func loadMaterialTile(ctx context.Context, artifact Artifact, bounds image.Rectangle, reflectance bool) (materialTile, error) {
	if err := ctx.Err(); err != nil {
		return materialTile{}, err
	}

	payload, err := readBoundedMaterialFile(artifact.PayloadPath)
	if err != nil {
		return materialTile{}, err
	}

	digest := sha256.Sum256(payload)
	if artifact.Encoding != worldMaterialAssetProcessor || artifact.OutputDigest != hex.EncodeToString(digest[:]) ||
		artifact.OutputBytes != int64(len(payload)) || len(payload) < materialTileHeaderBytes {
		return materialTile{}, ErrCacheOutput
	}

	dataSize, reflectanceSize := uint64(
		binary.BigEndian.Uint32(payload[:materialTileLengthBytes]),
	), uint64(
		binary.BigEndian.Uint32(payload[materialTileLengthBytes:materialTileHeaderBytes]),
	)
	if dataSize == 0 || (reflectanceSize != 0) != reflectance || dataSize+reflectanceSize != uint64(len(payload)-materialTileHeaderBytes) {
		return materialTile{}, ErrCacheOutput
	}

	split := materialTileHeaderBytes + int(dataSize)

	data, err := decodeCachedMaterialTile(payload[materialTileHeaderBytes:split], bounds, false)
	if err != nil {
		return materialTile{}, err
	}

	tile := materialTile{Data: data}
	if reflectance {
		tile.Reflectance, err = decodeCachedMaterialTile(payload[split:], bounds, true)
	}

	return tile, err
}

func decodeCachedMaterialTile(payload []byte, bounds image.Rectangle, reflectance bool) (*image.NRGBA, error) {
	metadata, err := qoi.Inspect(payload)
	if err != nil || int(metadata.Width) != bounds.Dx() || int(metadata.Height) != bounds.Dy() ||
		metadata.Channels != qoi.ChannelsRGBA || metadata.Colorspace != qoi.ColorspaceLinear {
		return nil, ErrCacheOutput
	}

	_, decoded, err := qoi.Decode(payload)
	if err != nil {
		return nil, ErrCacheOutput
	}

	if reflectance {
		for offset := 0; offset < len(decoded.Pix); offset += materialPixelBytes {
			if decoded.Pix[offset+2] != 0 || decoded.Pix[offset+3] != materialOpaque {
				return nil, ErrCacheOutput
			}
		}
	}

	return decoded, nil
}
