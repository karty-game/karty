package assetpipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"

	"github.com/disintegration/imaging"
	sdkqoi "github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/asset"
	"golang.org/x/image/webp"
)

// ImageSourceFormat identifies the source codec from its contents, not its file name.
type ImageSourceFormat string

const (
	ImageSourcePNG            ImageSourceFormat = "png"
	ImageSourceJPEG           ImageSourceFormat = "jpeg"
	ImageSourceWebP           ImageSourceFormat = "webp"
	ImageEncodingQOI                            = "qoi"
	imageBitDepth                               = 8
	imageBytesPerPixel                          = 4
	pngSignatureLength                          = 8
	chunkHeaderLength                           = 8
	pngChunkEnvelopeLength                      = 12
	webPContainerHeaderLength                   = 12
	webPExtendedHeaderLength                    = 10
	riffPaddingAlignment                        = 2
)

var (
	ErrImageSourceTooLarge = errors.New("image source exceeds the byte limit")
	ErrImageFormat         = errors.New("unsupported image format")
	ErrAnimatedImage       = errors.New("animated images are not supported")
	ErrImageBounds         = errors.New("image dimensions exceed resource limits")
	ErrImageOutput         = errors.New("encoded QOI output failed validation")
	errTruncatedChunk      = errors.New("truncated image chunk")
	errChunkLength         = errors.New("image chunk length exceeds source")
	errPNGHeader           = errors.New("PNG IHDR must be the first chunk")
	errMissingPNGHeader    = errors.New("PNG is missing IHDR")
	errWebPContainerLength = errors.New("WebP RIFF length does not match source")
	errWebPExtendedLength  = errors.New("invalid WebP VP8X length")
)

// ImageMetadata describes the normalized QOI output and the source properties
// relevant to build reports. QOI always contains straight-alpha, 8-bit pixels.
type ImageMetadata struct {
	SourceFormat     ImageSourceFormat `json:"sourceFormat"`
	SourceWidth      uint32            `json:"sourceWidth"`
	SourceHeight     uint32            `json:"sourceHeight"`
	SourceBitDepth   uint8             `json:"sourceBitDepth"`
	Width            uint32            `json:"width"`
	Height           uint32            `json:"height"`
	BitDepth         uint8             `json:"bitDepth"`
	PrecisionReduced bool              `json:"precisionReduced"`
	DecodedBytes     uint64            `json:"decodedBytes"`
}

// ProcessedImage is an owned, deterministic processor result. EncodedQOI does
// not alias the caller's source bytes. Digests are raw SHA-256 values so the
// cache layer can choose its own textual representation.
type ProcessedImage struct {
	Encoding     string
	EncodedQOI   []byte
	SourceBytes  uint64
	OutputBytes  uint64
	SourceDigest [sha256.Size]byte
	OutputDigest [sha256.Size]byte
	RecipeDigest [sha256.Size]byte
	Metadata     ImageMetadata
}

// ProcessImage snapshots bounded source bytes, decodes PNG, JPEG, or WebP by
// content, applies the canonical qoi@1 recipe, and validates its QOI output.
func ProcessImage(source []byte, recipe asset.ImageRecipe) (ProcessedImage, error) {
	if len(source) > asset.MaxSourceAssetBytes {
		return ProcessedImage{}, ErrImageSourceTooLarge
	}

	if err := recipe.Validate(); err != nil {
		return ProcessedImage{}, fmt.Errorf("validate image recipe: %w", err)
	}

	snapshot := bytes.Clone(source)

	format, bitDepth, err := inspectImageContainer(snapshot)
	if err != nil {
		return ProcessedImage{}, err
	}

	config, err := decodeImageConfig(snapshot, format)
	if err != nil {
		return ProcessedImage{}, fmt.Errorf("inspect %s image: %w", format, err)
	}

	if err := validateSourceDimensions(config.Width, config.Height); err != nil {
		return ProcessedImage{}, err
	}

	targetWidth, targetHeight := fittedDimensions(uint32(config.Width), uint32(config.Height), recipe)
	if err := validateOutputDimensions(targetWidth, targetHeight); err != nil {
		return ProcessedImage{}, err
	}

	decoded, err := decodeImage(snapshot, format)
	if err != nil {
		return ProcessedImage{}, fmt.Errorf("decode %s image: %w", format, err)
	}

	if decoded.Bounds().Dx() != config.Width || decoded.Bounds().Dy() != config.Height {
		return ProcessedImage{}, fmt.Errorf("decoded dimensions differ from inspected header: %w", ErrImageBounds)
	}

	normalized := imaging.Clone(decoded)

	if targetWidth != uint32(config.Width) || targetHeight != uint32(config.Height) {
		filter := imaging.NearestNeighbor
		if recipe.Filter == asset.ImageFilterSmooth {
			filter = imaging.Lanczos
		}

		normalized = imaging.Resize(normalized, int(targetWidth), int(targetHeight), filter)
	}

	output, _, err := sdkqoi.Encode(normalized, sdkqoi.Options{
		Channels: sdkqoi.ChannelsRGBA, Colorspace: sdkqoi.ColorspaceSRGB,
	})
	if err != nil {
		return ProcessedImage{}, fmt.Errorf("encode QOI: %w", err)
	}

	if err := validateQOIOutput(output, normalized, targetWidth, targetHeight); err != nil {
		return ProcessedImage{}, err
	}

	return ProcessedImage{
		Encoding:     ImageEncodingQOI,
		EncodedQOI:   output,
		SourceBytes:  uint64(len(snapshot)),
		OutputBytes:  uint64(len(output)),
		SourceDigest: sha256.Sum256(snapshot),
		OutputDigest: sha256.Sum256(output),
		RecipeDigest: digestImageRecipe(recipe),
		Metadata: ImageMetadata{
			SourceFormat:     format,
			SourceWidth:      uint32(config.Width),
			SourceHeight:     uint32(config.Height),
			SourceBitDepth:   bitDepth,
			Width:            targetWidth,
			Height:           targetHeight,
			BitDepth:         imageBitDepth,
			PrecisionReduced: bitDepth > imageBitDepth,
			DecodedBytes:     uint64(targetWidth) * uint64(targetHeight) * imageBytesPerPixel,
		},
	}, nil
}

func inspectImageContainer(source []byte) (ImageSourceFormat, uint8, error) {
	if bytes.HasPrefix(source, []byte("\x89PNG\r\n\x1a\n")) {
		bitDepth, animated, err := inspectPNG(source)
		if err != nil {
			return "", 0, fmt.Errorf("%w: inspect PNG chunks: %w", ErrImageFormat, err)
		}

		if animated {
			return "", 0, fmt.Errorf("APNG: %w", ErrAnimatedImage)
		}

		return ImageSourcePNG, bitDepth, nil
	}

	if len(source) >= 3 && source[0] == 0xff && source[1] == 0xd8 && source[2] == 0xff {
		return ImageSourceJPEG, imageBitDepth, nil
	}

	if len(source) >= 12 && string(source[:4]) == "RIFF" && string(source[8:12]) == "WEBP" {
		animated, err := inspectWebP(source)
		if err != nil {
			return "", 0, fmt.Errorf("%w: inspect WebP chunks: %w", ErrImageFormat, err)
		}

		if animated {
			return "", 0, fmt.Errorf("animated WebP: %w", ErrAnimatedImage)
		}

		return ImageSourceWebP, imageBitDepth, nil
	}

	return "", 0, ErrImageFormat
}

func inspectPNG(source []byte) (uint8, bool, error) {
	position := pngSignatureLength

	var bitDepth uint8

	seenIHDR := false

	for position < len(source) {
		if len(source)-position < pngChunkEnvelopeLength {
			return 0, false, errTruncatedChunk
		}

		length := uint64(binary.BigEndian.Uint32(source[position : position+4]))
		chunkType := string(source[position+4 : position+8])

		end := uint64(position) + pngChunkEnvelopeLength + length
		if end > uint64(len(source)) {
			return 0, false, errChunkLength
		}

		if !seenIHDR {
			if chunkType != "IHDR" || length != 13 {
				return 0, false, errPNGHeader
			}

			bitDepth = source[position+16]
			seenIHDR = true
		}

		if chunkType == "acTL" {
			return bitDepth, true, nil
		}

		position = int(end)

		if chunkType == "IEND" {
			break
		}
	}

	if !seenIHDR {
		return 0, false, errMissingPNGHeader
	}

	return bitDepth, false, nil
}

func inspectWebP(source []byte) (bool, error) {
	declared := uint64(binary.LittleEndian.Uint32(source[4:8])) + chunkHeaderLength
	if declared != uint64(len(source)) {
		return false, errWebPContainerLength
	}

	position := webPContainerHeaderLength
	for position < len(source) {
		if len(source)-position < chunkHeaderLength {
			return false, errTruncatedChunk
		}

		chunkType := string(source[position : position+4])
		length := uint64(binary.LittleEndian.Uint32(source[position+4 : position+8]))
		paddedLength := length + length%riffPaddingAlignment

		end := uint64(position) + chunkHeaderLength + paddedLength
		if end > uint64(len(source)) {
			return false, errChunkLength
		}

		if chunkType == "ANIM" || chunkType == "ANMF" {
			return true, nil
		}

		if chunkType == "VP8X" {
			if length != webPExtendedHeaderLength {
				return false, errWebPExtendedLength
			}

			if source[position+8]&(1<<1) != 0 {
				return true, nil
			}
		}

		position = int(end)
	}

	return false, nil
}

func decodeImageConfig(source []byte, format ImageSourceFormat) (image.Config, error) {
	switch format {
	case ImageSourcePNG:
		return png.DecodeConfig(bytes.NewReader(source))
	case ImageSourceJPEG:
		return jpeg.DecodeConfig(bytes.NewReader(source))
	case ImageSourceWebP:
		return webp.DecodeConfig(bytes.NewReader(source))
	default:
		return image.Config{}, ErrImageFormat
	}
}

func decodeImage(source []byte, format ImageSourceFormat) (image.Image, error) {
	switch format {
	case ImageSourcePNG:
		return png.Decode(bytes.NewReader(source))
	case ImageSourceJPEG:
		return jpeg.Decode(bytes.NewReader(source))
	case ImageSourceWebP:
		return webp.Decode(bytes.NewReader(source))
	default:
		return nil, ErrImageFormat
	}
}

func validateSourceDimensions(width, height int) error {
	if width < 1 || height < 1 || width > asset.MaxSourceImageDimension || height > asset.MaxSourceImageDimension ||
		uint64(width)*uint64(height) > asset.MaxSourceImagePixels {
		return ErrImageBounds
	}

	return nil
}

func validateOutputDimensions(width, height uint32) error {
	if width < 1 || height < 1 || width > asset.MaxTextureDimension || height > asset.MaxTextureDimension ||
		uint64(width)*uint64(height) > asset.MaxTexturePixels ||
		uint64(width)*uint64(height)*imageBytesPerPixel > asset.MaxDecodedTextureBytes {
		return ErrImageBounds
	}

	return nil
}

func fittedDimensions(width, height uint32, recipe asset.ImageRecipe) (uint32, uint32) {
	if (recipe.MaxWidth == 0 || width <= recipe.MaxWidth) && (recipe.MaxHeight == 0 || height <= recipe.MaxHeight) {
		return width, height
	}

	if recipe.MaxHeight == 0 || (recipe.MaxWidth != 0 && uint64(recipe.MaxWidth)*uint64(height) <= uint64(recipe.MaxHeight)*uint64(width)) {
		return recipe.MaxWidth, max(1, uint32(uint64(height)*uint64(recipe.MaxWidth)/uint64(width)))
	}

	return max(1, uint32(uint64(width)*uint64(recipe.MaxHeight)/uint64(height))), recipe.MaxHeight
}

func digestImageRecipe(recipe asset.ImageRecipe) [sha256.Size]byte {
	var canonical bytes.Buffer
	canonical.WriteString(string(asset.ProcessorQOIv1))
	canonical.WriteByte(0)
	_ = binary.Write(&canonical, binary.BigEndian, recipe.MaxWidth)
	_ = binary.Write(&canonical, binary.BigEndian, recipe.MaxHeight)
	canonical.WriteString(string(recipe.Filter))
	canonical.WriteByte(0)
	canonical.WriteByte(recipe.BitDepth)

	return sha256.Sum256(canonical.Bytes())
}

func validateQOIOutput(output []byte, expected *image.NRGBA, width, height uint32) error {
	metadata, decoded, err := sdkqoi.Decode(output)
	if err != nil || metadata.Width != width || metadata.Height != height ||
		metadata.Channels != sdkqoi.ChannelsRGBA || metadata.Colorspace != sdkqoi.ColorspaceSRGB {
		return fmt.Errorf("invalid QOI header: %w", ErrImageOutput)
	}

	if !bytes.Equal(decoded.Pix, expected.Pix) || nrgbAHasUnexpectedLayout(decoded, int(width), int(height)) {
		return fmt.Errorf("QOI pixels differ from normalized source: %w", ErrImageOutput)
	}

	return nil
}

func nrgbAHasUnexpectedLayout(img *image.NRGBA, width, height int) bool {
	return img.Rect != image.Rect(0, 0, width, height) || img.Stride != width*4 || len(img.Pix) != width*height*4
}
