package assetpipeline

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/toolchain"
)

const (
	worldMaterialProcessor = "world-material-atlas@7"
	materialPixelBytes     = 4
	materialOpaque         = 255
	materialMidpoint       = 127
	materialNeutral        = 128
	materialHalfTexel      = 0.5
	materialPayloadHeader  = 20
	materialAtlasCount     = 2
	materialMergeCount     = 3
)

// Keep the complete processor below the shared cache's two-minute stale-lock
// window. Cancellation kills the native child; no partial result is published.
const worldMaterialTimeout = 90 * time.Second

type materializeExecutor func(context.Context, sdk.Manifest, string, []string) error

// WorldMaterialIDs uses the public compiled world's stable first-surface order.
func WorldMaterialIDs(document world.Document) []uint32 {
	return world.MaterialIDs(&document)
}

// packWorldAlbedo preserves authored pixel edges with nearest sampling and
// extrudes each border for runtime atlas sampling.
func packWorldAlbedo(
	ctx context.Context,
	layout worldmaterial.Layout,
	textures map[uint32][]byte,
	reflectance ...bool,
) (*image.NRGBA, error) {
	if err := layout.Validate(); err != nil {
		return nil, err
	}

	if err := validateMaterialMemory(ctx, layout, textures, reflectance); err != nil {
		return nil, err
	}

	atlas := image.NewNRGBA(image.Rect(0, 0, layout.Width, layout.Height))
	for offset := materialPixelBytes - 1; offset < len(atlas.Pix); offset += materialPixelBytes {
		atlas.Pix[offset] = materialOpaque
	}

	for _, rect := range layout.Materials {
		var source *image.NRGBA

		if rect.MaterialID != 0 {
			decoded, err := decodeMaterialLayer(textures[rect.MaterialID], rect.Coverage)
			if err != nil {
				return nil, fmt.Errorf("material %d: %w", rect.MaterialID, err)
			}

			source = decoded
		}

		for row := -rect.Gutter; row < rect.Height+rect.Gutter; row++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}

			vertical := min(max(row, 0), rect.Height-1)
			for column := -rect.Gutter; column < rect.Width+rect.Gutter; column++ {
				horizontal := min(max(column, 0), rect.Width-1)
				pixel := color.NRGBA{192, 192, 192, 255}

				if source != nil {
					pixel = sampleRepeatingMaterial(source, horizontal, vertical, rect.Width, rect.Height)
				}

				atlas.SetNRGBA(rect.X+column, rect.Y+row, pixel)
			}
		}
	}

	return atlas, nil
}

// Preserve the authored palette and pixel edges when enlarging retro textures.
func sampleRepeatingMaterial(source *image.NRGBA, column, row, width, height int) color.NRGBA {
	return source.NRGBAAt(column*source.Bounds().Dx()/width, row*source.Bounds().Dy()/height)
}

func decodeMaterialLayer(encoded []byte, coverage string) (*image.NRGBA, error) {
	_, decoded, err := qoi.Decode(encoded)
	if err != nil {
		return nil, err
	}

	for offset := materialPixelBytes - 1; offset < len(decoded.Pix); offset += materialPixelBytes {
		if coverage != worldmaterial.CoverageMasked && decoded.Pix[offset] != materialOpaque {
			return nil, worldmaterial.ErrAtlas
		}
	}

	return decoded, nil
}

// ProcessWorldMaterials shares the build/dev content-addressed cache and its
// atomic cross-process publication. Each normalized material is generated and
// cached independently before the matching atlas layers are assembled.
func ProcessWorldMaterials(ctx context.Context, projectRoot string, manifest sdk.Manifest,
	document world.Document, textures map[uint32][]byte, options ...map[uint32]MaterializeOptions) (worldmaterial.Pair, Artifact, error) {
	return processWorldMaterials(ctx, projectRoot, manifest, document, textures, runManagedMaterialize, options...)
}

func processWorldMaterials(
	ctx context.Context,
	projectRoot string,
	manifest sdk.Manifest,
	document world.Document,
	textures map[uint32][]byte,
	execute materializeExecutor,
	options ...map[uint32]MaterializeOptions,
) (worldmaterial.Pair, Artifact, error) {
	if execute == nil || (!slices.Contains(manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialAtlasV1) &&
		!slices.Contains(manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialAtlasV2)) {
		return worldmaterial.Pair{}, Artifact{}, fmt.Errorf(
			"SDK %s does not enable %s: %w", manifest.Version, worldmaterial.Feature, ErrCacheRecipe)
	}

	if len(options) > 1 {
		return worldmaterial.Pair{}, Artifact{}, ErrCacheRecipe
	}

	var materialOptions map[uint32]MaterializeOptions
	if len(options) == 1 {
		materialOptions = options[0]
	}

	for id, value := range materialOptions {
		if err := value.Validate(); err != nil {
			return worldmaterial.Pair{}, Artifact{}, fmt.Errorf("material %d: %w", id, err)
		}
	}

	if err := ctx.Err(); err != nil {
		return worldmaterial.Pair{}, Artifact{}, err
	}

	spec, err := toolchain.MaterializeSpec(manifest)
	if err != nil {
		return worldmaterial.Pair{}, Artifact{}, err
	}

	platform := runtime.GOOS + "-" + runtime.GOARCH
	pin, ok := spec.Artifacts[platform]

	if !ok {
		return worldmaterial.Pair{}, Artifact{}, fmt.Errorf("materialize unavailable for %s: %w", platform, ErrCacheRecipe)
	}

	layout, err := materialWorldLayout(document)
	if err != nil {
		return worldmaterial.Pair{}, Artifact{}, err
	}

	layout, err = worldmaterial.NewMipLayout(layout)
	if err != nil {
		return worldmaterial.Pair{}, Artifact{}, err
	}

	albedo, err := packWorldAlbedo(ctx, layout, textures, materialReflectanceEnabled(manifest))
	if err != nil {
		return worldmaterial.Pair{}, Artifact{}, err
	}

	var input bytes.Buffer

	writer := materialPNGWriter(func(encoded []byte) (int, error) {
		if err := ctx.Err(); err != nil {
			return 0, err
		}

		if len(encoded) > MaxCachePayloadBytes-input.Len() {
			return 0, ErrAssetResourceLimits
		}

		return input.Write(encoded)
	})

	secondaryOnly := secondaryOnlyMaterials(document)

	if err := png.Encode(writer, albedo); err != nil {
		return worldmaterial.Pair{}, Artifact{}, err
	}

	// Include every referenced generation recipe in stable layout order.
	arguments := materializeManifestArguments(manifest, "atlas.png", ".", MaterializeOptions{})
	for _, rect := range layout.Materials {
		digest := sha256.Sum256(
			[]byte(strings.Join(materializeManifestArguments(manifest, "atlas.png", ".", materialOptions[rect.MaterialID]), "\x00")),
		)
		arguments = append(arguments, hex.EncodeToString(digest[:]))
	}

	snapshot, err := SnapshotBytes(input.Bytes())
	if err != nil {
		return worldmaterial.Pair{}, Artifact{}, err
	}

	recipe, err := worldMaterialRecipe(
		manifest,
		layout,
		textures,
		platform,
		pin.SHA256,
		arguments,
		secondaryOnly,
	)
	if err != nil {
		return worldmaterial.Pair{}, Artifact{}, err
	}

	artifact, err := NewProjectCache(projectRoot).Resolve(ctx, snapshot, recipe,
		func(ctx context.Context, source SourceSnapshot, _ Recipe) (Processed, error) {
			return generateWorldMaterials(
				ctx,
				projectRoot,
				manifest,
				layout,
				albedo,
				textures,
				secondaryOnly,
				materialOptions,
				platform,
				pin.SHA256,
				execute,
			)
		})
	if err != nil {
		return worldmaterial.Pair{}, Artifact{}, err
	}

	pair, err := loadWorldMaterialPair(ctx, artifact, layout)

	return pair, artifact, err
}

func generateWorldMaterials(ctx context.Context, projectRoot string, manifest sdk.Manifest,
	layout worldmaterial.Layout, albedo *image.NRGBA, textures map[uint32][]byte, secondaryOnly map[uint32]bool,
	options map[uint32]MaterializeOptions, platform, checksum string, execute materializeExecutor) (Processed, error) {
	ctx, cancel := context.WithTimeout(ctx, worldMaterialTimeout)
	defer cancel()

	data := image.NewNRGBA(albedo.Bounds())

	var reflectance *image.NRGBA
	if materialReflectanceEnabled(manifest) {
		reflectance = newNeutralReflectance(albedo.Bounds())
	}

	for _, rect := range layout.Materials {
		if err := ctx.Err(); err != nil {
			return Processed{}, err
		}

		if secondaryOnly[rect.MaterialID] {
			continue
		}

		tile, err := cachedMaterialTile(ctx, projectRoot, manifest, albedo, rect,
			textures[rect.MaterialID], options[rect.MaterialID], platform, checksum, execute)
		if err != nil {
			return Processed{}, fmt.Errorf("material %d: %w", rect.MaterialID, err)
		}

		copyMaterialTile(data, reflectance, tile, rect)
	}

	neutralizeSecondaryTiles(layout, data, secondaryOnly, color.NRGBA{128, 128, 0, 255})

	neutralizeMaskedMaterialPixels(layout, albedo, data, reflectance)

	if err := extrudeMaterialData(ctx, layout, data); err != nil {
		return Processed{}, err
	}

	albedoQOI, _, err := qoi.Encode(albedo, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceSRGB})
	if err != nil {
		return Processed{}, err
	}

	if err := ctx.Err(); err != nil {
		return Processed{}, err
	}

	dataQOI, _, err := qoi.Encode(data, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceLinear})
	if err != nil {
		return Processed{}, err
	}

	tail, err := buildWorldMaterialMips(ctx, layout, albedo, data)
	if err != nil {
		return Processed{}, err
	}

	tailQOI, _, err := qoi.Encode(tail, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceLinear})
	if err != nil {
		return Processed{}, err
	}

	pair := worldmaterial.Pair{Layout: layout, Albedo: albedoQOI, Data: dataQOI, MipTail: tailQOI}
	if err := encodeWorldReflectance(ctx, &pair, reflectance); err != nil {
		return Processed{}, err
	}

	if err := pair.Validate(); err != nil {
		return Processed{}, err
	}

	payload, err := encodeMaterialPayload(pair)
	if err != nil {
		return Processed{}, err
	}

	if err := ctx.Err(); err != nil {
		return Processed{}, err
	}

	metadata, err := worldmaterial.EncodeLayout(layout)
	if err != nil {
		return Processed{}, err
	}

	return Processed{Encoding: worldMaterialProcessor, Payload: payload, Metadata: metadata}, nil
}

func extrudeMaterialData(ctx context.Context, layout worldmaterial.Layout, data *image.NRGBA) error {
	// Atlas-wide filters can cross tile borders; runtime bilinear sampling must not.
	for _, rect := range layout.Materials {
		for row := -rect.Gutter; row < rect.Height+rect.Gutter; row++ {
			if err := ctx.Err(); err != nil {
				return err
			}

			for column := -rect.Gutter; column < rect.Width+rect.Gutter; column++ {
				if column < 0 || row < 0 || column >= rect.Width || row >= rect.Height {
					data.SetNRGBA(rect.X+column, rect.Y+row,
						data.NRGBAAt(rect.X+min(max(column, 0), rect.Width-1), rect.Y+min(max(row, 0), rect.Height-1)))
				}
			}
		}
	}

	return nil
}

// Private cache framing avoids JSON/base64 expansion of already encoded QOI.
// Layout remains bounded binary metadata; optional companions are framed separately.
func encodeMaterialPayload(pair worldmaterial.Pair) ([]byte, error) {
	if len(pair.Albedo) == 0 || len(pair.Data) == 0 {
		return nil, ErrCacheOutput
	}

	total := materialPayloadHeader
	for _, encoded := range [][]byte{pair.Albedo, pair.Data, pair.MipTail, pair.Reflectance, pair.ReflectanceMipTail} {
		if len(encoded) > MaxCachePayloadBytes-total {
			return nil, ErrCacheOutput
		}

		total += len(encoded)
	}

	payload := make([]byte, materialPayloadHeader, total)
	for index, encoded := range [][]byte{pair.Albedo, pair.Data, pair.MipTail, pair.Reflectance, pair.ReflectanceMipTail} {
		binary.BigEndian.PutUint32(payload[index*4:(index+1)*4], uint32(len(encoded)))
		payload = append(payload, encoded...)
	}

	return payload, nil
}

func loadWorldMaterialPair(ctx context.Context, artifact Artifact, layout worldmaterial.Layout) (worldmaterial.Pair, error) {
	if err := ctx.Err(); err != nil {
		return worldmaterial.Pair{}, err
	}

	payload, err := readBoundedMaterialFile(artifact.PayloadPath)
	if err != nil {
		return worldmaterial.Pair{}, err
	}

	digest := sha256.Sum256(payload)
	if hex.EncodeToString(digest[:]) != artifact.OutputDigest || int64(len(payload)) != artifact.OutputBytes ||
		artifact.Encoding != worldMaterialProcessor || len(payload) <= materialPayloadHeader {
		return worldmaterial.Pair{}, ErrCacheOutput
	}

	var (
		sizes [5]uint64
		total uint64
	)

	for index := range sizes {
		sizes[index] = uint64(binary.BigEndian.Uint32(payload[index*4 : (index+1)*4]))
		total += sizes[index]
	}

	if sizes[0] == 0 || sizes[1] == 0 || total != uint64(len(payload)-materialPayloadHeader) {
		return worldmaterial.Pair{}, ErrCacheOutput
	}

	cachedLayout, err := worldmaterial.DecodeLayout(artifact.Metadata)
	if err != nil {
		return worldmaterial.Pair{}, ErrCacheOutput
	}

	wantLayout, err := worldmaterial.EncodeLayout(layout)
	if err != nil || !bytes.Equal(wantLayout, artifact.Metadata) {
		return worldmaterial.Pair{}, ErrCacheOutput
	}

	var layers [5][]byte

	position := materialPayloadHeader
	for index, size := range sizes {
		end := position + int(size)
		layers[index] = payload[position:end:end]
		position = end
	}

	pair := worldmaterial.Pair{
		Layout: cachedLayout, Albedo: layers[0], Data: layers[1], MipTail: layers[2],
		Reflectance: layers[3], ReflectanceMipTail: layers[4],
	}

	if err := pair.Validate(); err != nil {
		return worldmaterial.Pair{}, ErrCacheOutput
	}

	if err := ctx.Err(); err != nil {
		return worldmaterial.Pair{}, err
	}

	return pair, nil
}

func worldMaterialRecipe(manifest sdk.Manifest, layout worldmaterial.Layout, textures map[uint32][]byte,
	platform, checksum string, arguments []string, colorOnly ...map[uint32]bool) (Recipe, error) {
	// Hash the complete referenced QOI, not only sampled atlas pixels. Changing
	// an unsampled source texel still invalidates the asset recipe.
	sources := sha256.New()

	for _, rect := range layout.Materials {
		if rect.MaterialID != 0 {
			digest := sha256.Sum256(textures[rect.MaterialID])
			_, _ = sources.Write(digest[:])
		}
	}

	// Artistic controls affect packaged metadata only, never generated bytes.
	layout.Materials = append([]worldmaterial.Rect(nil), layout.Materials...)
	for index := range layout.Materials {
		layout.Materials[index].Strengths = nil
	}

	layoutJSON, err := worldmaterial.EncodeLayout(layout)
	if err != nil {
		return Recipe{}, err
	}

	var roles []uint32

	if len(colorOnly) > 0 {
		for _, rect := range layout.Materials {
			if colorOnly[0][rect.MaterialID] {
				roles = append(roles, rect.MaterialID)
			}
		}
	}

	layoutDigest := sha256.Sum256(layoutJSON)

	return NewRecipe("world-material", worldMaterialProcessor, []Revision{
		{Name: "packer", Version: "6"}, {Name: "merge", Version: "1"},
		{Name: "material-mips", Version: "2"}, {Name: "coverage-extension", Version: "1"},
		{Name: "materialize", Version: manifest.Tools.MaterializeRevision},
		{Name: "tool-version", Version: manifest.Tools.Materialize},
		{Name: "archive", Version: checksum}, {Name: "qoi", Version: "1"},
		{Name: "reflectance", Version: strconv.FormatBool(materialReflectanceEnabled(manifest))},
	}, struct {
		Layout    string   `toml:"layout"`
		Sources   string   `toml:"sources"`
		Platform  string   `toml:"platform"`
		Arguments []string `toml:"arguments"`
		ColorOnly []uint32 `toml:"colorOnly,omitempty"`
	}{hex.EncodeToString(layoutDigest[:]), hex.EncodeToString(sources.Sum(nil)), platform, arguments, roles})
}

func readBoundedMaterialFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > MaxCachePayloadBytes {
		return nil, fmt.Errorf("material output missing or invalid %s: %w", path, ErrCacheOutput)
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, ErrCacheOutput
	}

	encoded, err := io.ReadAll(io.LimitReader(file, MaxCachePayloadBytes+1))
	if err != nil {
		return nil, err
	}

	if int64(len(encoded)) != info.Size() || int64(len(encoded)) > MaxCachePayloadBytes {
		return nil, ErrCacheOutput
	}

	return encoded, nil
}

func readMaterialMap(directory, name string, bounds image.Rectangle) (image.Image, error) {
	encoded, err := readBoundedMaterialFile(filepath.Join(directory, "atlas_"+name+".png"))
	if err != nil {
		return nil, fmt.Errorf("materialize %s: %w", name, err)
	}

	config, err := png.DecodeConfig(bytes.NewReader(encoded))
	if err != nil || bounds.Min != (image.Point{}) || config.Width != bounds.Dx() || config.Height != bounds.Dy() ||
		config.Width < 1 || config.Height < 1 || config.Width > asset.MaxTextureDimension || config.Height > asset.MaxTextureDimension ||
		uint64(config.Width)*uint64(config.Height) > asset.MaxTexturePixels {
		return nil, fmt.Errorf("materialize %s dimensions differ from bounded atlas: %w", name, ErrCacheOutput)
	}

	if config.ColorModel == color.RGBA64Model || config.ColorModel == color.NRGBA64Model || config.ColorModel == color.Gray16Model {
		return nil, fmt.Errorf("materialize %s must use 8-bit channels: %w", name, ErrCacheOutput)
	}

	return png.Decode(bytes.NewReader(encoded))
}

func mergeMaterialMap(ctx context.Context, directory, name string, data *image.NRGBA) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	generated, err := readMaterialMap(directory, name, data.Bounds())
	if err != nil {
		return err
	}

	for row := range data.Bounds().Dy() {
		if err := ctx.Err(); err != nil {
			return err
		}

		for column := range data.Bounds().Dx() {
			pixel, valid := color.NRGBAModel.Convert(generated.At(column, row)).(color.NRGBA)
			if !valid || pixel.A != materialOpaque {
				return fmt.Errorf("materialize %s contains transparent pixels: %w", name, ErrCacheOutput)
			}

			offset := data.PixOffset(column, row)

			switch name {
			case "normal":
				// Materialize's standard midpoint 127/128 becomes exact zero.
				if pixel.R == materialMidpoint {
					pixel.R = materialNeutral
				}

				if pixel.G == materialMidpoint {
					pixel.G = materialNeutral
				}

				data.Pix[offset], data.Pix[offset+1] = pixel.R, pixel.G
			case "height":
				data.Pix[offset+2] = pixel.R
			case "ao":
				data.Pix[offset+3] = pixel.R
			case "metallic":
				data.Pix[offset] = pixel.R
			case "smoothness":
				data.Pix[offset+1] = pixel.R
			default:
				return ErrCacheOutput
			}
		}
	}

	return nil
}

// Only this production executor resolves a binary. Fixtures inject an executor
// directly, without a PATH lookup, global override or fake production fallback.
func runManagedMaterialize(ctx context.Context, manifest sdk.Manifest, directory string, arguments []string) error {
	executable, err := toolchain.EnsureMaterialize(ctx, manifest, "")
	if err != nil {
		return err
	}

	executable, err = filepath.Abs(executable)
	if err != nil {
		return err
	}

	return materializeWithFallback(ctx, runtime.GOOS, os.Environ(), func(environment []string) (string, error) {
		// #nosec G204 -- Only the checksum-verified SDK-managed binary is executed.
		command := exec.CommandContext(ctx, executable, arguments...)
		command.Dir, command.Env = directory, environment
		command.WaitDelay = time.Second

		var output materializeLog

		command.Stdout, command.Stderr = &output, &output
		err := command.Run()

		return output.buffer.String(), err
	})
}

// Retry only adapter discovery failures, never shader/output failures. Keep
// explicit user backend choices and the process environment unchanged.
func materializeWithFallback(ctx context.Context, system string, environment []string,
	run func([]string) (string, error)) error {
	output, err := run(environment)
	if err == nil {
		return nil
	}

	forced := false

	for _, entry := range environment {
		if strings.HasPrefix(entry, "MATERIALIZE_GPU_BACKEND=") && strings.TrimPrefix(entry, "MATERIALIZE_GPU_BACKEND=") != "" {
			forced = true
		}
	}

	if ctx.Err() == nil && system == "linux" && !forced && strings.Contains(output, "No GPU adapter available") {
		fallback := make([]string, 0, len(environment)+3)
		for _, entry := range environment {
			if !strings.HasPrefix(entry, "MATERIALIZE_GPU_BACKEND=") &&
				!strings.HasPrefix(entry, "LIBGL_ALWAYS_SOFTWARE=") && !strings.HasPrefix(entry, "EGL_PLATFORM=") {
				fallback = append(fallback, entry)
			}
		}

		fallback = append(fallback, "MATERIALIZE_GPU_BACKEND=gl", "LIBGL_ALWAYS_SOFTWARE=1", "EGL_PLATFORM=surfaceless")

		var retryOutput string

		retryOutput, err = run(fallback)
		if err == nil {
			return nil
		}

		output += "\nSoftware OpenGL fallback (requires Mesa EGL/DRI):\n" + retryOutput
	}

	if cancellation := ctx.Err(); cancellation != nil {
		err = cancellation
	}

	return fmt.Errorf("materialize atlas: %w\n%s", err, output)
}

type materializeLog struct{ buffer bytes.Buffer }

func (log *materializeLog) Write(p []byte) (int, error) {
	const maximum = 64 * 1024

	count := len(p)
	if remaining := maximum - log.buffer.Len(); remaining > 0 {
		_, _ = log.buffer.Write(p[:min(remaining, count)])
	}

	return count, nil
}

type materialPNGWriter func([]byte) (int, error)

func (writer materialPNGWriter) Write(encoded []byte) (int, error) {
	return writer(encoded)
}

func validateMaterialMemory(ctx context.Context, layout worldmaterial.Layout, textures map[uint32][]byte, reflectance []bool) error {
	atlasBytes := uint64(layout.Width) * uint64(layout.Height) * materialPixelBytes
	// Bound the full assembly plus one decoded generation map before processing.
	mergeCount, atlasCount, tailCount := uint64(materialMergeCount), uint64(materialAtlasCount), uint64(1)
	if len(reflectance) > 0 && reflectance[0] {
		mergeCount, atlasCount, tailCount = mergeCount+1, atlasCount+1, 2
	}

	if atlasBytes > asset.MaxDecodedTextures/mergeCount {
		return ErrAssetResourceLimits
	}

	total := atlasBytes * atlasCount
	width, height := layout.MipDimensions()

	tailBytes := uint64(width) * uint64(height) * materialPixelBytes * tailCount
	if tailBytes > asset.MaxDecodedTextures-total {
		return ErrAssetResourceLimits
	}

	total += tailBytes

	for _, encoded := range textures {
		if err := ctx.Err(); err != nil {
			return err
		}

		metadata, err := qoi.Inspect(encoded)
		if err != nil {
			return err
		}

		if metadata.DecodedBytes > asset.MaxDecodedTextures-total {
			return ErrAssetResourceLimits
		}

		total += metadata.DecodedBytes
	}

	return nil
}

func materialWorldLayout(document world.Document) (worldmaterial.Layout, error) {
	layout, err := worldmaterial.NewLayout(WorldMaterialIDs(document))
	if document.MaterialLayers != nil {
		layout, err = worldmaterial.NewLayoutV2(WorldMaterialIDs(document))
		if err == nil {
			for index := range layout.Materials {
				if isMaskedFrameMaterial(document, layout.Materials[index].MaterialID) {
					layout.Materials[index].Coverage = worldmaterial.CoverageMasked
				}
			}
		}
	}

	return layout, err
}

func copyMaterialTile(data, reflectance *image.NRGBA, tile materialTile, rect worldmaterial.Rect) {
	for row := range rect.Height {
		copy(data.Pix[data.PixOffset(rect.X, rect.Y+row):][:rect.Width*materialPixelBytes],
			tile.Data.Pix[tile.Data.PixOffset(0, row):][:rect.Width*materialPixelBytes])

		if reflectance != nil {
			copy(reflectance.Pix[reflectance.PixOffset(rect.X, rect.Y+row):][:rect.Width*materialPixelBytes],
				tile.Reflectance.Pix[tile.Reflectance.PixOffset(0, row):][:rect.Width*materialPixelBytes])
		}
	}
}

func neutralizeMaskedMaterialPixels(layout worldmaterial.Layout, albedo, data, reflectance *image.NRGBA) {
	for _, rect := range layout.Materials {
		if rect.Coverage != worldmaterial.CoverageMasked {
			continue
		}

		for row := range rect.Height {
			for column := range rect.Width {
				if albedo.NRGBAAt(rect.X+column, rect.Y+row).A == 0 {
					data.SetNRGBA(rect.X+column, rect.Y+row, color.NRGBA{128, 128, 0, 255})

					if reflectance != nil {
						reflectance.SetNRGBA(rect.X+column, rect.Y+row, color.NRGBA{A: materialOpaque})
					}
				}
			}
		}
	}
}

func encodeWorldReflectance(ctx context.Context, pair *worldmaterial.Pair, reflectance *image.NRGBA) error {
	if reflectance == nil {
		return nil
	}

	if err := extrudeMaterialData(ctx, pair.Layout, reflectance); err != nil {
		return err
	}

	var err error

	pair.Reflectance, _, err = qoi.Encode(reflectance, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceLinear})
	if err != nil {
		return err
	}

	reflectanceTail, err := buildWorldReflectanceMips(ctx, pair.Layout, reflectance)
	if err != nil {
		return err
	}

	pair.ReflectanceMipTail, _, err = qoi.Encode(
		reflectanceTail,
		qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceLinear},
	)
	if err != nil {
		return err
	}

	return nil
}
