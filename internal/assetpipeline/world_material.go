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
	worldMaterialProcessor = "world-material-atlas@4"
	materialPixelBytes     = 4
	materialOpaque         = 255
	materialMidpoint       = 127
	materialNeutral        = 128
	materialHalfTexel      = 0.5
	materialPayloadHeader  = 12
	materialAtlasCount     = 2
	materialMergeCount     = 3
)

// Keep the complete processor below the shared cache's two-minute stale-lock
// window. Cancellation kills the native child; no partial result is published.
const worldMaterialTimeout = 90 * time.Second

type materializeExecutor func(context.Context, sdk.Manifest, string, []string) error

func materializeArguments(inputPath, directory string) []string {
	return []string{inputPath, "--output", directory, "--only", "height,normal,ao",
		"--normal-format", "opengl", "--format", "png", "--no-seamless"}
}

// WorldMaterialIDs uses the public compiled world's stable first-surface order.
func WorldMaterialIDs(document world.Document) []uint32 {
	return world.MaterialIDs(&document)
}

// packWorldAlbedo preserves authored pixel edges with nearest sampling and
// extrudes each border before the atlas is handed to Materialize.
func packWorldAlbedo(ctx context.Context, layout worldmaterial.Layout, textures map[uint32][]byte) (*image.NRGBA, error) {
	if err := layout.Validate(); err != nil {
		return nil, err
	}

	atlasBytes := uint64(layout.Width) * uint64(layout.Height) * materialPixelBytes
	// Merge holds albedo, data and one decoded map simultaneously.
	if atlasBytes > asset.MaxDecodedTextures/materialMergeCount {
		return nil, ErrAssetResourceLimits
	}

	total := atlasBytes * materialAtlasCount
	width, height := layout.MipDimensions()

	tailBytes := uint64(width) * uint64(height) * materialPixelBytes
	if tailBytes > asset.MaxDecodedTextures-total {
		return nil, ErrAssetResourceLimits
	}

	total += tailBytes

	for _, encoded := range textures {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		metadata, err := qoi.Inspect(encoded)
		if err != nil {
			return nil, err
		}

		if metadata.DecodedBytes > asset.MaxDecodedTextures-total {
			return nil, ErrAssetResourceLimits
		}

		total += metadata.DecodedBytes
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
// atomic cross-process publication. A cold entry invokes only the managed,
// checksum-pinned native executable, once on the completed extruded atlas.
func ProcessWorldMaterials(ctx context.Context, projectRoot string, manifest sdk.Manifest,
	document world.Document, textures map[uint32][]byte) (worldmaterial.Pair, Artifact, error) {
	return processWorldMaterials(ctx, projectRoot, manifest, document, textures, runManagedMaterialize)
}

func processWorldMaterials(ctx context.Context, projectRoot string, manifest sdk.Manifest,
	document world.Document, textures map[uint32][]byte, execute materializeExecutor) (worldmaterial.Pair, Artifact, error) {
	if execute == nil || (!slices.Contains(manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialAtlasV1) &&
		!slices.Contains(manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialAtlasV2)) {
		return worldmaterial.Pair{}, Artifact{}, fmt.Errorf(
			"SDK %s does not enable %s: %w", manifest.Version, worldmaterial.Feature, ErrCacheRecipe)
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

	if err != nil {
		return worldmaterial.Pair{}, Artifact{}, err
	}

	layout, err = worldmaterial.NewMipLayout(layout)
	if err != nil {
		return worldmaterial.Pair{}, Artifact{}, err
	}

	albedo, err := packWorldAlbedo(ctx, layout, textures)
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

	generation, err := materialGenerationInput(ctx, layout, albedo)
	if err != nil {
		return worldmaterial.Pair{}, Artifact{}, err
	}

	secondaryOnly := secondaryOnlyMaterials(document)
	neutralizeSecondaryTiles(layout, generation, secondaryOnly, color.NRGBA{128, 128, 128, 255})

	if err := png.Encode(writer, generation); err != nil {
		return worldmaterial.Pair{}, Artifact{}, err
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
		materializeArguments("atlas.png", "."),
		secondaryOnly,
	)
	if err != nil {
		return worldmaterial.Pair{}, Artifact{}, err
	}

	artifact, err := NewProjectCache(projectRoot).Resolve(ctx, snapshot, recipe,
		func(ctx context.Context, source SourceSnapshot, _ Recipe) (Processed, error) {
			return generateWorldMaterials(ctx, manifest, source, layout, albedo, secondaryOnly, execute)
		})
	if err != nil {
		return worldmaterial.Pair{}, Artifact{}, err
	}

	pair, err := loadWorldMaterialPair(ctx, artifact, layout)

	return pair, artifact, err
}

func generateWorldMaterials(ctx context.Context, manifest sdk.Manifest, source SourceSnapshot,
	layout worldmaterial.Layout, albedo *image.NRGBA, secondaryOnly map[uint32]bool, execute materializeExecutor) (Processed, error) {
	ctx, cancel := context.WithTimeout(ctx, worldMaterialTimeout)
	defer cancel()

	directory, err := os.MkdirTemp("", "karty-material-atlas-")
	if err != nil {
		return Processed{}, err
	}
	defer os.RemoveAll(directory)

	inputPath := filepath.Join(directory, "atlas.png")
	if err := os.WriteFile(inputPath, source.Bytes(), 0o600); err != nil {
		return Processed{}, err
	}

	if err := execute(ctx, manifest, directory, materializeArguments(inputPath, directory)); err != nil {
		return Processed{}, err
	}

	data := image.NewNRGBA(albedo.Bounds())
	for _, name := range []string{"normal", "height", "ao"} {
		if err := mergeMaterialMap(ctx, directory, name, data); err != nil {
			return Processed{}, err
		}
	}

	neutralizeSecondaryTiles(layout, data, secondaryOnly, color.NRGBA{128, 128, 0, 255})

	for _, rect := range layout.Materials {
		if rect.Coverage != worldmaterial.CoverageMasked {
			continue
		}

		for y := range rect.Height {
			for x := range rect.Width {
				if albedo.NRGBAAt(rect.X+x, rect.Y+y).A == 0 {
					data.SetNRGBA(rect.X+x, rect.Y+y, color.NRGBA{128, 128, 0, 255})
				}
			}
		}
	}

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

	return Processed{Encoding: worldMaterialProcessor, Payload: payload, Metadata: layout}, nil
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
// Layout remains canonical JSON metadata, and the public level format is unchanged.
func encodeMaterialPayload(pair worldmaterial.Pair) ([]byte, error) {
	if len(pair.Albedo) == 0 || len(pair.Data) == 0 {
		return nil, ErrCacheOutput
	}

	total := materialPayloadHeader
	for _, encoded := range [][]byte{pair.Albedo, pair.Data, pair.MipTail} {
		if len(encoded) > MaxCachePayloadBytes-total {
			return nil, ErrCacheOutput
		}

		total += len(encoded)
	}

	payload := make([]byte, materialPayloadHeader, total)
	for index, encoded := range [][]byte{pair.Albedo, pair.Data, pair.MipTail} {
		binary.BigEndian.PutUint32(payload[index*4:(index+1)*4], uint32(len(encoded)))
		payload = append(payload, encoded...)
	}

	return payload, nil
}

func loadWorldMaterialPair(ctx context.Context, artifact Artifact, layout worldmaterial.Layout) (worldmaterial.Pair, error) {
	if err := ctx.Err(); err != nil {
		return worldmaterial.Pair{}, err
	}

	payload, err := readBoundedMaterialFile(artifact.PayloadPath, MaxCachePayloadBytes)
	if err != nil {
		return worldmaterial.Pair{}, err
	}

	digest := sha256.Sum256(payload)
	if hex.EncodeToString(digest[:]) != artifact.OutputDigest || int64(len(payload)) != artifact.OutputBytes ||
		artifact.Encoding != worldMaterialProcessor || len(payload) <= materialPayloadHeader {
		return worldmaterial.Pair{}, ErrCacheOutput
	}

	albedoSize := uint64(binary.BigEndian.Uint32(payload[:4]))
	dataSize := uint64(binary.BigEndian.Uint32(payload[4:8]))

	tailSize := uint64(binary.BigEndian.Uint32(payload[8:materialPayloadHeader]))

	if albedoSize == 0 || dataSize == 0 || albedoSize+dataSize+tailSize != uint64(len(payload)-materialPayloadHeader) {
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

	split := materialPayloadHeader + int(albedoSize)
	end := split + int(dataSize)
	pair := worldmaterial.Pair{
		Layout:  cachedLayout,
		Albedo:  payload[materialPayloadHeader:split:split],
		Data:    payload[split:end:end],
		MipTail: payload[end:],
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
		{Name: "packer", Version: "5"}, {Name: "merge", Version: "1"},
		{Name: "material-mips", Version: "2"}, {Name: "coverage-extension", Version: "1"},
		{Name: "materialize", Version: manifest.Tools.MaterializeRevision},
		{Name: "tool-version", Version: manifest.Tools.Materialize},
		{Name: "archive", Version: checksum}, {Name: "qoi", Version: "1"},
	}, struct {
		Layout    string   `json:"layout"`
		Sources   string   `json:"sources"`
		Platform  string   `json:"platform"`
		Arguments []string `json:"arguments"`
		ColorOnly []uint32 `json:"colorOnly,omitempty"`
	}{hex.EncodeToString(layoutDigest[:]), hex.EncodeToString(sources.Sum(nil)), platform, arguments, roles})
}

func readBoundedMaterialFile(path string, maximum int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maximum {
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

	encoded, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		return nil, err
	}

	if int64(len(encoded)) != info.Size() || int64(len(encoded)) > maximum {
		return nil, ErrCacheOutput
	}

	return encoded, nil
}

func readMaterialMap(directory, name string, bounds image.Rectangle) (image.Image, error) {
	encoded, err := readBoundedMaterialFile(filepath.Join(directory, "atlas_"+name+".png"), MaxCachePayloadBytes)
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
