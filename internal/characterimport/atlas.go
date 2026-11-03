//nolint:gocognit,gocyclo,maintidx,mnd,nestif,varnamelen // Material conversion keeps glTF UV arithmetic, sampler enums and sRGB coefficients together.
package characterimport

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"sort"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/character"
	_ "golang.org/x/image/webp"
)

const gutter = 2

type atlasRegion struct {
	pixels      *image.NRGBA
	uv          []float32
	vertexStart int
	x, y        int
}

func (g *gltf) region(ctx context.Context, materialIndex *int, uv []float32, count int) (atlasRegion, error) {
	region := atlasRegion{uv: make([]float32, count*2)}
	factor := [4]float64{1, 1, 1, 1}

	var info *textureInfo

	if materialIndex != nil {
		if *materialIndex < 0 || *materialIndex >= len(g.Materials) {
			return region, invalid("material index")
		}

		m := g.Materials[*materialIndex]
		if m.AlphaMode != "" && m.AlphaMode != "OPAQUE" || len(m.Extensions) != 0 || len(m.NormalTexture) != 0 ||
			len(m.OcclusionTexture) != 0 ||
			len(m.EmissiveTexture) != 0 ||
			len(m.PBRMetallicRoughness.MetallicRoughnessTexture) != 0 {
			return region, invalid("unsupported material textures, extensions or transparency")
		}

		for _, value := range m.EmissiveFactor {
			if value != 0 {
				return region, invalid("emissive materials are unsupported")
			}
		}

		if len(m.PBRMetallicRoughness.BaseColorFactor) != 0 {
			if len(m.PBRMetallicRoughness.BaseColorFactor) != 4 {
				return region, invalid("base color factor")
			}

			for i, v := range m.PBRMetallicRoughness.BaseColorFactor {
				if v < 0 || v > 1 {
					return region, invalid("base color factor range")
				}

				factor[i] = float64(v)
			}
		}

		info = m.PBRMetallicRoughness.BaseColorTexture
	}

	var source image.Image = image.NewUniform(color.White)

	width, height := 1, 1
	wrapS, wrapT := 10497, 10497
	offset := [2]float64{}
	scale := [2]float64{1, 1}
	rotation := float64(0)

	if info != nil {
		if len(uv) != count*2 || info.Index < 0 || info.Index >= len(g.Textures) || info.TexCoord != 0 {
			return region, invalid("base color texture coordinates")
		}

		for extension, raw := range info.Extensions {
			if extension != "KHR_texture_transform" {
				return region, invalid("unsupported texture extension")
			}

			var transform textureTransform
			if json.Unmarshal(raw, &transform) != nil || transform.TexCoord != 0 {
				return region, invalid("texture transform")
			}

			if len(transform.Offset) != 0 {
				if len(transform.Offset) != 2 {
					return region, invalid("texture offset")
				}

				copy(offset[:], transform.Offset)
			}

			if len(transform.Scale) != 0 {
				if len(transform.Scale) != 2 {
					return region, invalid("texture scale")
				}

				copy(scale[:], transform.Scale)
			}

			rotation = transform.Rotation
		}

		texture := g.Textures[info.Index]

		imageIndex := texture.Source
		for extension, raw := range texture.Extensions {
			if extension != "EXT_texture_webp" {
				return region, invalid("unsupported texture extension")
			}

			var webp struct {
				Source *int `json:"source"`
			}
			if json.Unmarshal(raw, &webp) != nil {
				return region, invalid("WebP extension")
			}

			imageIndex = webp.Source
		}

		if imageIndex == nil || *imageIndex < 0 || *imageIndex >= len(g.Images) {
			return region, invalid("image index")
		}

		if texture.Sampler != nil {
			if *texture.Sampler < 0 || *texture.Sampler >= len(g.Samplers) {
				return region, invalid("sampler index")
			}

			sampler := g.Samplers[*texture.Sampler]
			if sampler.WrapS != 0 {
				wrapS = sampler.WrapS
			}

			if sampler.WrapT != 0 {
				wrapT = sampler.WrapT
			}
		}

		if !validWrap(wrapS) || !validWrap(wrapT) {
			return region, invalid("texture wrap mode")
		}

		img := g.Images[*imageIndex]
		if img.URI != "" || img.BufferView == nil ||
			img.MimeType != "image/png" && img.MimeType != "image/jpeg" && img.MimeType != "image/webp" {
			return region, invalid("only embedded PNG, JPEG and WebP images are supported")
		}

		data, err := g.bytes(ctx, *img.BufferView)
		if err != nil {
			return region, err
		}

		config, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || config.Width < 1 || config.Height < 1 || config.Width > character.MaxTextureSize ||
			config.Height > character.MaxTextureSize {
			return region, invalid("texture dimensions")
		}

		source, _, err = image.Decode(bytes.NewReader(data))
		if err != nil {
			return region, err
		}

		width, height = config.Width, config.Height
	}

	minU, minV, maxU, maxV := float64(0), float64(0), float64(1), float64(1)
	if info != nil {
		minU, minV, maxU, maxV = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)

		for i := range count {
			u, v := float64(uv[i*2])*scale[0], float64(uv[i*2+1])*scale[1]

			u, v = offset[0]+math.Cos(rotation)*u-math.Sin(rotation)*v, offset[1]+math.Sin(rotation)*u+math.Cos(rotation)*v
			if math.IsNaN(u) || math.IsInf(u, 0) || math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(u) > 1024 || math.Abs(v) > 1024 {
				return region, invalid("texture coordinate range")
			}

			region.uv[i*2] = float32(u)
			region.uv[i*2+1] = float32(v)
			minU = min(minU, u)
			minV = min(minV, v)
			maxU = max(maxU, u)
			maxV = max(maxV, v)
		}

		minU, minV = math.Floor(minU), math.Floor(minV)
		maxU, maxV = max(minU+1, math.Ceil(maxU)), max(minV+1, math.Ceil(maxV))
	}

	regionWidth, regionHeight := int(maxU-minU)*width, int(maxV-minV)*height
	if regionWidth > character.MaxTextureSize-2*gutter || regionHeight > character.MaxTextureSize-2*gutter {
		return region, invalid("tiled texture exceeds atlas bounds")
	}

	pixels := (regionWidth + 2*gutter) * (regionHeight + 2*gutter)
	if pixels > character.MaxTextureSize*character.MaxTextureSize-g.atlasPixels {
		return region, invalid("atlas pixel budget")
	}

	g.atlasPixels += pixels
	region.pixels = image.NewNRGBA(image.Rect(0, 0, regionWidth+2*gutter, regionHeight+2*gutter))

	var colors [3][256]uint8

	for channel := range 3 {
		for value := range 256 {
			colors[channel][value] = modulate(uint8(value), factor[channel])
		}
	}

	for y := -gutter; y < regionHeight+gutter; y++ {
		if err := ctx.Err(); err != nil {
			return region, err
		}

		for x := -gutter; x < regionWidth+gutter; x++ {
			sx, sy := wrapped(x+int(minU)*width, width, wrapS), wrapped(y+int(minV)*height, height, wrapT)

			pixel, valid := color.NRGBAModel.Convert(source.At(sx+source.Bounds().Min.X, sy+source.Bounds().Min.Y)).(color.NRGBA)
			if !valid {
				return region, invalid("image color conversion")
			}

			pixel.R = colors[0][pixel.R]
			pixel.G = colors[1][pixel.G]
			pixel.B = colors[2][pixel.B]
			pixel.A = 255
			region.pixels.SetNRGBA(x+gutter, y+gutter, pixel)
		}
	}

	for i := range count {
		if info != nil {
			region.uv[i*2] = float32((float64(region.uv[i*2])-minU)*float64(width) + gutter)
			region.uv[i*2+1] = float32((float64(region.uv[i*2+1])-minV)*float64(height) + gutter)
		} else {
			region.uv[i*2] = gutter + .5
			region.uv[i*2+1] = gutter + .5
		}
	}

	return region, nil
}

func validWrap(mode int) bool { return mode == 10497 || mode == 33071 || mode == 33648 }
func wrapped(value, size, mode int) int {
	if mode == 33071 {
		return max(0, min(size-1, value))
	}

	period := size
	if mode == 33648 {
		period *= 2
	}

	value = ((value % period) + period) % period
	if value >= size {
		return period - 1 - value
	}

	return value
}

// glTF baseColorFactor multiplies linear RGB, while image channels are sRGB.
func modulate(value uint8, factor float64) uint8 {
	v := float64(value) / 255
	if v <= .04045 {
		v /= 12.92
	} else {
		v = math.Pow((v+.055)/1.055, 2.4)
	}

	v *= factor
	if v <= .0031308 {
		v *= 12.92
	} else {
		v = 1.055*math.Pow(v, 1/2.4) - .055
	}

	return uint8(math.Round(max(0, min(1, v)) * 255))
}

func packAtlas(model *character.Model, regions []atlasRegion) error {
	order := make([]int, len(regions))
	area := 0

	for i, r := range regions {
		order[i] = i

		area += r.pixels.Bounds().Dx() * r.pixels.Bounds().Dy()
		if area > character.MaxTextureSize*character.MaxTextureSize {
			return invalid("atlas area exceeds bound")
		}
	}

	sort.SliceStable(order, func(i, j int) bool {
		return regions[order[i]].pixels.Bounds().Dy() > regions[order[j]].pixels.Bounds().Dy()
	})

	width, height := 0, 0

	for candidate := 32; candidate <= character.MaxTextureSize; candidate *= 2 {
		x, y, row := 0, 0, 0
		ok := true

		for _, index := range order {
			r := &regions[index]

			w, h := r.pixels.Bounds().Dx(), r.pixels.Bounds().Dy()
			if w > candidate {
				ok = false

				break
			}

			if x+w > candidate {
				x = 0
				y += row
				row = 0
			}

			r.x, r.y = x, y
			x += w
			row = max(row, h)
		}

		if ok && y+row <= character.MaxTextureSize {
			width, height = candidate, y+row

			break
		}
	}

	if width == 0 || height == 0 {
		return invalid("cannot pack bounded character atlas")
	}

	atlas := image.NewNRGBA(image.Rect(0, 0, width, height))

	for _, r := range regions {
		for y := range r.pixels.Bounds().Dy() {
			copy(atlas.Pix[(r.y+y)*atlas.Stride+r.x*4:], r.pixels.Pix[y*r.pixels.Stride:(y+1)*r.pixels.Stride])
		}

		for i := range len(r.uv) / 2 {
			base := (r.vertexStart + i) * character.VertexStride
			model.Vertices[base+6] = (r.uv[i*2] + float32(r.x)) / float32(width)
			model.Vertices[base+7] = (r.uv[i*2+1] + float32(r.y)) / float32(height)
		}
	}

	encoded, _, err := qoi.Encode(atlas, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceSRGB})
	if err != nil {
		return err
	}

	model.Width, model.Height, model.AlbedoQOI = width, height, encoded

	return nil
}
