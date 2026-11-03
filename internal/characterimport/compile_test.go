//nolint:varnamelen // Fixtures use short GLB records and accessor symbols for byte-layout assertions.
package characterimport

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math"
	"reflect"
	"testing"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty/internal/characterimport/meshopt"
)

type sourceBuilder struct {
	bin              []byte
	views, accessors []map[string]any
}

func (b *sourceBuilder) floats(values []float32, kind string, dimension int) int {
	start := len(b.bin)
	for _, v := range values {
		b.bin = binary.LittleEndian.AppendUint32(b.bin, math.Float32bits(v))
	}

	b.views = append(b.views, map[string]any{"buffer": 0, "byteOffset": start, "byteLength": len(b.bin) - start})
	b.accessors = append(
		b.accessors,
		map[string]any{"bufferView": len(b.views) - 1, "componentType": 5126, "count": len(values) / dimension, "type": kind},
	)

	return len(b.accessors) - 1
}
func (b *sourceBuilder) finish(t *testing.T, document map[string]any) []byte {
	t.Helper()

	document["asset"] = map[string]any{"version": "2.0"}
	document["buffers"] = []any{map[string]any{"byteLength": len(b.bin)}}
	document["bufferViews"] = b.views
	document["accessors"] = b.accessors

	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}

	for len(encoded)%4 != 0 {
		encoded = append(encoded, ' ')
	}

	for len(b.bin)%4 != 0 {
		b.bin = append(b.bin, 0)
	}

	out := []byte("glTF")
	out = binary.LittleEndian.AppendUint32(out, 2)
	out = binary.LittleEndian.AppendUint32(out, uint32(12+8+len(encoded)+8+len(b.bin)))
	out = binary.LittleEndian.AppendUint32(out, uint32(len(encoded)))
	out = binary.LittleEndian.AppendUint32(out, 0x4e4f534a)
	out = append(out, encoded...)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(b.bin)))
	out = binary.LittleEndian.AppendUint32(out, 0x004e4942)

	return append(out, b.bin...)
}
func fixture(t *testing.T) (model, bank []byte) {
	t.Helper()

	b := sourceBuilder{}
	position := b.floats([]float32{0, 0, 0, 1, 0, 0, 0, 1, 0}, "VEC3", 3)
	normal := b.floats([]float32{0, 0, 1, 0, 0, 1, 0, 0, 1}, "VEC3", 3)
	joints := b.floats(make([]float32, 12), "VEC4", 4)
	b.accessors[joints]["componentType"] = 5123
	b.views[joints]["byteLength"] = 24
	weights := b.floats([]float32{1, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0}, "VEC4", 4)
	nodes := make([]any, 1, 67)
	nodes[0] = map[string]any{"mesh": 0, "skin": 0}
	skinJoints := make([]int, 0, 66)

	for i := range 66 {
		nodes = append(nodes, map[string]any{"name": string(rune('A' + i))})
		skinJoints = append(skinJoints, i+1)
	}

	document := map[string]any{
		"nodes": nodes,
		"skins": []any{map[string]any{"joints": skinJoints}},
		"meshes": []any{
			map[string]any{
				"primitives": []any{
					map[string]any{
						"attributes": map[string]int{"POSITION": position, "NORMAL": normal, "JOINTS_0": joints, "WEIGHTS_0": weights},
						"material":   0,
					},
				},
			},
		},
		"materials": []any{map[string]any{"pbrMetallicRoughness": map[string]any{"baseColorFactor": []float32{.25, .25, .25, 1}}}},
	}
	model = b.finish(t, document)
	b = sourceBuilder{}
	times := b.floats([]float32{0, 1}, "SCALAR", 1)
	values := b.floats([]float32{0, 0, 0, 1, 0, 0}, "VEC3", 3)
	bank = b.finish(
		t,
		map[string]any{
			"nodes": []any{map[string]any{"name": "A"}},
			"animations": []any{
				map[string]any{
					"name":     "Walk",
					"samplers": []any{map[string]any{"input": times, "output": values, "interpolation": "STEP"}},
					"channels": []any{map[string]any{"sampler": 0, "target": map[string]any{"node": 0, "path": "translation"}}},
				},
			},
		},
	)

	return model, bank
}

func TestCompileSharesBankAndPrunesUnreferencedJoints(t *testing.T) {
	t.Parallel()
	model, bank := fixture(t)
	inputs := []Input{{ID: "guard", Model: model, Animations: bank}, {ID: "scientist", Model: model, Animations: bank}}

	document, err := Compile(context.Background(), inputs)
	if err != nil {
		t.Fatal(err)
	}

	if len(document.Banks) != 1 || len(document.Models) != 2 || len(document.Models[0].Joints) != 1 || document.Models[0].Joints[0] != 1 ||
		!document.Banks[0].Clips[0].Tracks[0].Step {
		t.Fatalf("incorrect compiled structure: %+v", document)
	}

	again, err := Compile(context.Background(), inputs)
	if err != nil || !reflect.DeepEqual(document, again) {
		t.Fatalf("import is not deterministic: %v", err)
	}

	_, image, err := qoi.Decode(document.Models[0].AlbedoQOI)
	if err != nil {
		t.Fatal(err)
	}

	red := image.NRGBAAt(2, 2).R
	if red < 136 || red > 138 {
		t.Fatalf("linear base color factor encoded as %d, want sRGB 137", red)
	}
}

func TestRejectsMalformedSourceAndUnsupportedAnimation(t *testing.T) {
	t.Parallel()
	model, bank := fixture(t)
	overflowingChunk := append([]byte(nil), model...)
	binary.LittleEndian.PutUint32(overflowingChunk[12:], 0xfffffffc)

	for _, broken := range [][]byte{nil, model[:len(model)-1], append([]byte("nope"), model[4:]...), overflowingChunk} {
		if _, err := Compile(context.Background(), []Input{{ID: "bad", Model: broken, Animations: bank}}); err == nil {
			t.Fatal("accepted malformed GLB")
		}
	}

	var b sourceBuilder

	times := b.floats([]float32{0, 1}, "SCALAR", 1)
	values := b.floats(make([]float32, 18), "VEC3", 3)

	cubic := b.finish(
		t,
		map[string]any{
			"nodes": []any{map[string]any{"name": "A"}},
			"animations": []any{
				map[string]any{
					"name":     "Cubic",
					"samplers": []any{map[string]any{"input": times, "output": values, "interpolation": "CUBICSPLINE"}},
					"channels": []any{map[string]any{"sampler": 0, "target": map[string]any{"node": 0, "path": "translation"}}},
				},
			},
		},
	)
	if _, err := Compile(context.Background(), []Input{{ID: "bad", Model: model, Animations: cubic}}); err == nil {
		t.Fatal("accepted unsupported cubic interpolation")
	}
}

func TestClampSamplerRetainsUVInterpolationOutsideImage(t *testing.T) {
	t.Parallel()

	source := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	source.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	source.SetNRGBA(1, 0, color.NRGBA{B: 255, A: 255})

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatal(err)
	}

	builder := sourceBuilder{bin: encoded.Bytes(), views: []map[string]any{{"buffer": 0, "byteLength": encoded.Len()}}}
	glbBytes := builder.finish(t, map[string]any{
		"images":    []any{map[string]any{"bufferView": 0, "mimeType": "image/png"}},
		"textures":  []any{map[string]any{"source": 0, "sampler": 0}},
		"samplers":  []any{map[string]any{"wrapS": 33071, "wrapT": 33071}},
		"materials": []any{map[string]any{"pbrMetallicRoughness": map[string]any{"baseColorTexture": map[string]any{"index": 0}}}},
	})

	document, err := parse(glbBytes, nil)
	if err != nil {
		t.Fatal(err)
	}

	index := 0

	region, err := document.region(t.Context(), &index, []float32{-1, 0, 1, 0, 0, 1}, 3)
	if err != nil {
		t.Fatal(err)
	}

	if region.uv[2]-region.uv[0] != 4 {
		t.Fatalf("clamped vertex coordinates changed interpolation: %v", region.uv)
	}

	if region.pixels.NRGBAAt(gutter+1, gutter).R != 255 || region.pixels.NRGBAAt(gutter+3, gutter).B != 255 {
		t.Fatal("clamped edge pixels were not baked across the original UV range")
	}
}

func TestAccessorNormalizedSignedValuesAndBounds(t *testing.T) {
	t.Parallel()

	index := 0
	g := gltf{
		bin:         []byte{128, 0, 127, 0},
		BufferViews: []view{{ByteLength: 4}},
		Accessors:   []accessor{{BufferView: &index, ComponentType: 5120, Count: 3, Type: "SCALAR", Normalized: true}},
		decoded:     map[int][]byte{},
	}

	values, err := g.values(context.Background(), 0, 1, 3)
	if err != nil || !reflect.DeepEqual(values, []float32{-1, 0, 1}) {
		t.Fatalf("normalized values: %v %v", values, err)
	}

	g.Accessors[0].ByteOffset = 3
	if _, err = g.values(context.Background(), 0, 1, 3); err == nil {
		t.Fatal("accepted accessor outside view")
	}
}

func TestPrimitiveMergeOffsetsIndicesAndRetainsSeparateMaterials(t *testing.T) {
	t.Parallel()
	model, _ := fixture(t)

	g, err := parse(model, nil)
	if err != nil {
		t.Fatal(err)
	}

	p := g.Meshes[0].Primitives[0]
	second := g.Materials[0]
	second.PBRMetallicRoughness.BaseColorFactor = []float32{1, 0, 0, 1}
	g.Materials = append(g.Materials, second)
	index := 1
	p.Material = &index
	g.Meshes[0].Primitives = append(g.Meshes[0].Primitives, p)

	compiled, err := g.model(t.Context(), "guard", 0)
	if err != nil {
		t.Fatal(err)
	}

	if len(compiled.Vertices) != 6*16 || !reflect.DeepEqual(compiled.Indices, []uint32{0, 1, 2, 3, 4, 5}) {
		t.Fatalf("incorrect merged primitive offsets: %v", compiled.Indices)
	}

	if compiled.Vertices[6] == compiled.Vertices[3*16+6] && compiled.Vertices[7] == compiled.Vertices[3*16+7] {
		t.Fatal("separate primitive materials share an atlas coordinate")
	}
}

func TestWrapAndLinearColor(t *testing.T) {
	t.Parallel()

	if wrapped(-1, 4, 10497) != 3 || wrapped(4, 4, 33648) != 3 || wrapped(-1, 4, 33071) != 0 {
		t.Fatal("incorrect texture wrap")
	}

	if modulate(255, .25) != 137 {
		t.Fatal("base color factor must multiply linear RGB")
	}
}

func TestMeshoptViewUsesDecodedStrideAndRejectsBudgetMismatch(t *testing.T) {
	t.Parallel()

	decoder, err := meshopt.New(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer decoder.Close(context.Background())

	source := make([]byte, 37)
	source[0] = 0xa0
	copy(source[33:], []byte{1, 2, 3, 4})

	raw, err := json.Marshal(
		map[string]any{"buffer": 0, "byteOffset": 0, "byteLength": len(source), "byteStride": 4, "count": 3, "mode": "ATTRIBUTES"},
	)
	if err != nil {
		t.Fatal(err)
	}

	index := 0
	g := gltf{
		bin:     source,
		decoder: decoder,
		decoded: map[int][]byte{},
		BufferViews: []view{
			{Buffer: 1, ByteLength: 12, ByteStride: 4, Extensions: map[string]json.RawMessage{"EXT_meshopt_compression": raw}},
		},
		Accessors: []accessor{{BufferView: &index, ComponentType: 5121, Count: 3, Type: "VEC4"}},
	}

	values, err := g.values(t.Context(), 0, 4, 3)
	if err != nil || !reflect.DeepEqual(values, []float32{1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4}) {
		t.Fatalf("decoded accessor: %v %v", values, err)
	}

	g.decoded = map[int][]byte{}

	g.BufferViews[0].ByteLength = 13
	if _, err = g.values(t.Context(), 0, 4, 3); err == nil {
		t.Fatal("accepted inconsistent decompressed view length")
	}
}
