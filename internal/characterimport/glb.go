// Package characterimport compiles bounded, embedded glTF characters offline.
//
//nolint:gocognit,gocyclo,mnd,nestif,varnamelen // Bounded glTF byte views and numeric component layouts are audited together.
package characterimport

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/karty-game/karty/internal/characterimport/meshopt"
)

const MaxSourceBytes = 64 << 20

var ErrInvalid = errors.New("invalid character GLB")

func invalid(reason string) error { return fmt.Errorf("%w: %s", ErrInvalid, reason) }
func parse(data []byte, decoder *meshopt.Decoder) (*gltf, error) {
	if len(data) < 20 || len(data) > MaxSourceBytes || string(data[:4]) != "glTF" || binary.LittleEndian.Uint32(data[4:]) != 2 ||
		uint64(binary.LittleEndian.Uint32(data[8:])) != uint64(len(data)) {
		return nil, invalid("invalid bounded GLB container")
	}

	var (
		document  gltf
		jsonBytes []byte
	)

	for offset := 12; offset < len(data); {
		if len(data)-offset < 8 {
			return nil, invalid("truncated chunk")
		}

		rawSize := binary.LittleEndian.Uint32(data[offset:])
		kind := binary.LittleEndian.Uint32(data[offset+4:])

		offset += 8
		if rawSize%4 != 0 || uint64(rawSize) > uint64(len(data)-offset) {
			return nil, invalid("invalid chunk bounds")
		}

		size := int(rawSize)

		switch kind {
		case 0x4e4f534a:
			if jsonBytes != nil || offset != 20 || size > 8<<20 {
				return nil, invalid("invalid JSON chunk")
			}

			jsonBytes = data[offset : offset+size]
		case 0x004e4942:
			if jsonBytes == nil || document.bin != nil {
				return nil, invalid("invalid BIN chunk")
			}

			document.bin = data[offset : offset+size]
		default:
			return nil, invalid("unsupported GLB chunk")
		}

		offset += size
	}

	if len(jsonBytes) == 0 || json.Unmarshal(jsonBytes, &document) != nil || document.Asset.Version != "2.0" ||
		len(document.Buffers) == 0 ||
		len(document.Nodes) > 512 ||
		len(document.BufferViews) > 4096 ||
		len(document.Accessors) > 8192 {
		return nil, invalid("invalid or oversized glTF document")
	}

	if len(document.Buffers) > 2 || len(document.Images) > 128 || len(document.Materials) > 128 || len(document.Textures) > 128 ||
		len(document.Samplers) > 128 ||
		len(document.Meshes) > 1 ||
		len(document.Skins) > 1 ||
		len(document.Animations) > 32 {
		return nil, invalid("glTF object count bounds")
	}

	for _, extension := range document.ExtensionsRequired {
		switch extension {
		case "EXT_meshopt_compression", "KHR_mesh_quantization", "EXT_texture_webp", "KHR_texture_transform":
		default:
			return nil, invalid("unsupported required extension " + extension)
		}
	}

	for i, buffer := range document.Buffers {
		if buffer.URI != "" || buffer.ByteLength < 0 || buffer.ByteLength > MaxSourceBytes ||
			i == 0 && (buffer.ByteLength > len(document.bin) || len(document.bin)-buffer.ByteLength > 3) {
			return nil, invalid("external or invalid buffer")
		}
	}

	document.decoder = decoder
	document.decoded = map[int][]byte{}

	return &document, nil
}

func (g *gltf) bytes(ctx context.Context, index int) ([]byte, error) {
	if index < 0 || index >= len(g.BufferViews) {
		return nil, invalid("buffer view index")
	}

	if data, ok := g.decoded[index]; ok {
		return data, nil
	}

	v := g.BufferViews[index]
	if v.ByteLength <= 0 || v.ByteLength > MaxSourceBytes || v.ByteOffset < 0 || v.ByteStride < 0 || v.ByteStride > 256 {
		return nil, invalid("buffer view bounds")
	}

	var out []byte

	if raw, ok := v.Extensions["EXT_meshopt_compression"]; ok {
		var compressed compressedView
		if json.Unmarshal(raw, &compressed) != nil || compressed.Buffer != 0 || compressed.ByteOffset < 0 || compressed.ByteLength <= 0 ||
			compressed.ByteOffset > len(g.bin)-compressed.ByteLength ||
			compressed.ByteStride <= 0 ||
			compressed.Count <= 0 ||
			compressed.Count > MaxSourceBytes/compressed.ByteStride ||
			compressed.Count*compressed.ByteStride != v.ByteLength ||
			v.ByteStride != 0 && v.ByteStride != compressed.ByteStride {
			return nil, invalid("Meshopt view bounds")
		}

		if g.decodedBytes > MaxSourceBytes-v.ByteLength {
			return nil, invalid("decoded buffer budget")
		}

		var err error

		out, err = g.decoder.Decode(
			ctx,
			g.bin[compressed.ByteOffset:compressed.ByteOffset+compressed.ByteLength],
			compressed.Count,
			compressed.ByteStride,
			compressed.Mode,
			compressed.Filter,
		)
		if err != nil {
			return nil, fmt.Errorf("decode Meshopt view %d: %w", index, err)
		}

		g.decodedBytes += len(out)
	} else {
		if v.Buffer != 0 || v.ByteOffset > len(g.bin)-v.ByteLength {
			return nil, invalid("unavailable fallback buffer")
		}

		out = g.bin[v.ByteOffset : v.ByteOffset+v.ByteLength]
	}

	g.decoded[index] = out

	return out, nil
}

func (g *gltf) values(ctx context.Context, index, dimension, maximum int) ([]float32, error) {
	if index < 0 || index >= len(g.Accessors) {
		return nil, invalid("accessor index")
	}

	a := g.Accessors[index]
	dims := map[string]int{"SCALAR": 1, "VEC2": 2, "VEC3": 3, "VEC4": 4, "MAT4": 16}

	width := map[int]int{5120: 1, 5121: 1, 5122: 2, 5123: 2, 5125: 4, 5126: 4}[a.ComponentType]
	if a.BufferView == nil || len(a.Sparse) != 0 || width == 0 || dims[a.Type] != dimension || a.Count < 1 || a.Count > maximum ||
		a.ByteOffset < 0 ||
		a.ByteOffset%width != 0 ||
		a.Normalized && (a.ComponentType == 5125 || a.ComponentType == 5126) {
		return nil, invalid("unsupported or oversized accessor")
	}

	data, err := g.bytes(ctx, *a.BufferView)
	if err != nil {
		return nil, err
	}

	stride := g.BufferViews[*a.BufferView].ByteStride
	if stride == 0 {
		stride = width * dimension
	}

	if stride < width*dimension || stride%width != 0 ||
		uint64(a.ByteOffset)+uint64(a.Count-1)*uint64(stride)+uint64(width*dimension) > uint64(len(data)) {
		return nil, invalid("accessor bounds")
	}

	out := make([]float32, a.Count*dimension)
	for i := range a.Count {
		for c := range dimension {
			p := data[a.ByteOffset+i*stride+c*width:]

			var v float32

			switch a.ComponentType {
			case 5120:
				v = float32(int8(p[0]))
				if a.Normalized {
					v = max(-1, v/127)
				}
			case 5121:
				v = float32(p[0])
				if a.Normalized {
					v /= 255
				}
			case 5122:
				v = float32(int16(binary.LittleEndian.Uint16(p)))
				if a.Normalized {
					v = max(-1, v/32767)
				}
			case 5123:
				v = float32(binary.LittleEndian.Uint16(p))
				if a.Normalized {
					v /= 65535
				}
			case 5125:
				u := binary.LittleEndian.Uint32(p)
				if u > 1<<24 {
					return nil, invalid("integer accessor exceeds exact range")
				}

				v = float32(u)
			case 5126:
				v = math.Float32frombits(binary.LittleEndian.Uint32(p))
			}

			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				return nil, invalid("nonfinite accessor")
			}

			out[i*dimension+c] = v
		}
	}

	return out, nil
}
