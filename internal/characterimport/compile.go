//nolint:gocognit,gocyclo,maintidx,mnd,nestif,varnamelen // The bounded hierarchy, palette and primitive compiler follows the glTF component layout directly.
package characterimport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"

	"github.com/karty-game/karty-sdk/format/character"
	"github.com/karty-game/karty/internal/characterimport/meshopt"
)

// Input contains a model and a separate animation GLB. All bytes must already
// have been read through the owning project's confined filesystem capability.
type Input struct {
	ID                string
	Model, Animations []byte
}

// Compile normalizes source-only GLBs; the runtime never loads GLB or Meshopt.
func Compile(ctx context.Context, inputs []Input) (*character.Document, error) {
	if len(inputs) < 1 || len(inputs) > character.MaxModels {
		return nil, invalid("model count")
	}

	decoder, err := meshopt.New(ctx)
	if err != nil {
		return nil, err
	}
	defer decoder.Close(context.WithoutCancel(ctx))

	document := &character.Document{}
	banks := map[[32]byte]int{}

	for _, input := range inputs {
		if err = ctx.Err(); err != nil {
			return nil, err
		}

		if len(input.Model) > MaxSourceBytes || len(input.Animations) > MaxSourceBytes {
			return nil, invalid("source byte budget")
		}

		hash := sha256.Sum256(input.Animations)

		bankIndex, exists := banks[hash]
		if !exists {
			g, parseErr := parse(input.Animations, decoder)
			if parseErr != nil {
				return nil, fmt.Errorf("%s animations: %w", input.ID, parseErr)
			}

			bank, bankErr := g.bank(ctx, hex.EncodeToString(hash[:]))
			if bankErr != nil {
				return nil, fmt.Errorf("%s animations: %w", input.ID, bankErr)
			}

			bankIndex = len(document.Banks)
			banks[hash] = bankIndex

			document.Banks = append(document.Banks, bank)
		}

		g, parseErr := parse(input.Model, decoder)
		if parseErr != nil {
			return nil, fmt.Errorf("%s model: %w", input.ID, parseErr)
		}

		model, modelErr := g.model(ctx, input.ID, bankIndex)
		if modelErr != nil {
			return nil, fmt.Errorf("%s model: %w", input.ID, modelErr)
		}

		document.Models = append(document.Models, model)
	}

	if err = document.Validate(); err != nil {
		return nil, fmt.Errorf("compiled characters: %w", err)
	}

	return document, nil
}

func (g *gltf) bank(ctx context.Context, id string) (character.Bank, error) {
	bank := character.Bank{ID: id}
	names := map[string]bool{}

	for _, n := range g.Nodes {
		if n.Name != "" {
			if names[n.Name] {
				return bank, invalid("ambiguous animation node name")
			}

			names[n.Name] = true
		}
	}

	if len(g.Animations) < 1 || len(g.Animations) > character.MaxClips {
		return bank, invalid("animation count")
	}

	keys := 0

	for _, source := range g.Animations {
		clip := character.Clip{Name: source.Name}
		if len(source.Channels) > character.MaxTracks {
			return bank, invalid("animation track count")
		}

		for _, channel := range source.Channels {
			if channel.Sampler < 0 || channel.Sampler >= len(source.Samplers) || channel.Target.Node == nil || *channel.Target.Node < 0 ||
				*channel.Target.Node >= len(g.Nodes) {
				return bank, invalid("animation target")
			}

			sampler := source.Samplers[channel.Sampler]
			track := character.Track{Node: g.Nodes[*channel.Target.Node].Name}

			switch sampler.Interpolation {
			case "", "LINEAR":
			case "STEP":
				track.Step = true
			default:
				return bank, invalid("only STEP and LINEAR animation are supported")
			}

			dimension := 3

			switch channel.Target.Path {
			case "translation":
				track.Path = character.Translation
			case "rotation":
				track.Path = character.Rotation
				dimension = 4
			case "scale":
				track.Path = character.Scale
			default:
				return bank, invalid("unsupported animation path")
			}

			var err error

			track.Times, err = g.values(ctx, sampler.Input, 1, character.MaxKeys-keys)
			if err != nil {
				return bank, err
			}

			keys += len(track.Times)

			track.Values, err = g.values(ctx, sampler.Output, dimension, len(track.Times))
			if err != nil {
				return bank, err
			}

			if len(track.Values) != len(track.Times)*dimension {
				return bank, invalid("animation sample counts")
			}

			clip.Duration = max(clip.Duration, track.Times[len(track.Times)-1])
			clip.Tracks = append(clip.Tracks, track)
		}

		bank.Clips = append(bank.Clips, clip)
	}

	return bank, nil
}

func (g *gltf) model(ctx context.Context, id string, bank int) (character.Model, error) {
	m := character.Model{ID: id, Bank: bank, MeshNode: -1}
	if len(g.Nodes) < 1 || len(g.Meshes) != 1 || len(g.Skins) != 1 {
		return m, invalid("exactly one skinned mesh and skin are required")
	}

	for i, n := range g.Nodes {
		if len(n.Matrix) != 0 {
			return m, invalid("matrix nodes must be exported as TRS")
		}

		if len(n.Weights) != 0 {
			return m, invalid("morph weights are unsupported")
		}

		out := character.Node{Name: n.Name, Parent: -1, Rotation: [4]float32{0, 0, 0, 1}, Scale: [3]float32{1, 1, 1}}
		if out.Name == "" {
			out.Name = fmt.Sprintf("@node/%d", i)
		}

		if len(n.Translation) != 0 {
			if len(n.Translation) != 3 {
				return m, invalid("translation shape")
			}

			copy(out.Translation[:], n.Translation)
		}

		if len(n.Rotation) != 0 {
			if len(n.Rotation) != 4 {
				return m, invalid("rotation shape")
			}

			copy(out.Rotation[:], n.Rotation)
		}

		if len(n.Scale) != 0 {
			if len(n.Scale) != 3 {
				return m, invalid("scale shape")
			}

			copy(out.Scale[:], n.Scale)
		}

		m.Nodes = append(m.Nodes, out)
		if n.Mesh != nil {
			if m.MeshNode != -1 || *n.Mesh != 0 || n.Skin == nil || *n.Skin != 0 {
				return m, invalid("unsupported mesh instance")
			}

			m.MeshNode = i
		}
	}

	for i, n := range g.Nodes {
		for _, child := range n.Children {
			if child < 0 || child >= len(m.Nodes) || m.Nodes[child].Parent != -1 {
				return m, invalid("node hierarchy")
			}

			m.Nodes[child].Parent = i
		}
	}

	if m.MeshNode < 0 {
		return m, invalid("missing mesh instance")
	}

	skin := g.Skins[0]
	if len(skin.Joints) < 1 || len(skin.Joints) > character.MaxNodes {
		return m, invalid("skin joint count")
	}

	inverse := make([]float32, len(skin.Joints)*16)
	if skin.InverseBindMatrices != nil {
		var err error

		inverse, err = g.values(ctx, *skin.InverseBindMatrices, 16, len(skin.Joints))
		if err != nil {
			return m, err
		}

		if len(inverse) != len(skin.Joints)*16 {
			return m, invalid("inverse bind count")
		}
	} else {
		for j := range skin.Joints {
			for k := range 4 {
				inverse[j*16+k*5] = 1
			}
		}
	}

	palette := map[int]int{}

	var regions []atlasRegion

	if len(g.Meshes[0].Weights) != 0 || len(g.Meshes[0].Primitives) > 128 {
		return m, invalid("morphs or excessive primitives")
	}

	for _, p := range g.Meshes[0].Primitives {
		if len(p.Targets) != 0 || len(p.Extensions) != 0 || p.Mode != nil && *p.Mode != 4 {
			return m, invalid("only unextended triangle primitives are supported")
		}

		attributes := map[string]int{"POSITION": 3, "NORMAL": 3, "TEXCOORD_0": 2, "JOINTS_0": 4, "WEIGHTS_0": 4}
		values := map[string][]float32{}

		for name, index := range p.Attributes {
			dimension, ok := attributes[name]
			if !ok {
				return m, invalid("unsupported vertex attribute " + name)
			}

			if index < 0 || index >= len(g.Accessors) {
				return m, invalid("vertex accessor index")
			}

			a := g.Accessors[index]
			if name == "JOINTS_0" && (a.ComponentType != 5121 && a.ComponentType != 5123 || a.Normalized) ||
				name == "WEIGHTS_0" && (a.ComponentType != 5126 && (a.ComponentType != 5121 && a.ComponentType != 5123 || !a.Normalized)) {
				return m, invalid("skin accessor component type")
			}

			v, err := g.values(ctx, index, dimension, character.MaxVertices)
			if err != nil {
				return m, err
			}

			values[name] = v
		}

		position := values["POSITION"]
		count := len(position) / 3

		base := len(m.Vertices) / character.VertexStride
		if count == 0 || base > character.MaxVertices-count || len(values["NORMAL"]) != count*3 || len(values["JOINTS_0"]) != count*4 ||
			len(values["WEIGHTS_0"]) != count*4 ||
			values["TEXCOORD_0"] != nil && len(values["TEXCOORD_0"]) != count*2 {
			return m, invalid("vertex attribute counts")
		}

		region, err := g.region(ctx, p.Material, values["TEXCOORD_0"], count)
		if err != nil {
			return m, err
		}

		region.vertexStart = base
		regions = append(regions, region)

		for v := range count {
			vertex := make([]float32, character.VertexStride)
			copy(vertex[:3], position[v*3:])
			copy(vertex[3:6], values["NORMAL"][v*3:])
			copy(vertex[6:8], region.uv[v*2:])

			weightSum := float32(0)

			for slot := range 4 {
				weight := values["WEIGHTS_0"][v*4+slot]

				joint := values["JOINTS_0"][v*4+slot]
				if weight < 0 || joint != float32(int(joint)) || joint < 0 || int(joint) >= len(skin.Joints) {
					return m, invalid("skin influence")
				}

				if weight > 0 {
					source := int(joint)

					target, exists := palette[source]
					if !exists {
						target = len(palette)
						if target >= character.MaxJoints {
							return m, invalid("referenced joint palette exceeds 64")
						}

						if skin.Joints[source] < 0 || skin.Joints[source] >= len(m.Nodes) {
							return m, invalid("skin node index")
						}

						palette[source] = target
						m.Joints = append(m.Joints, uint16(skin.Joints[source]))
						m.InverseBind = append(m.InverseBind, inverse[source*16:(source+1)*16]...)
					}

					vertex[8+slot] = float32(target)
				}

				vertex[12+slot] = weight
				weightSum += weight
			}

			if weightSum <= 0 {
				return m, invalid("unweighted vertex")
			}

			for slot := range 4 {
				vertex[12+slot] /= weightSum
			}

			normalLength := math.Sqrt(float64(vertex[3]*vertex[3] + vertex[4]*vertex[4] + vertex[5]*vertex[5]))
			if normalLength < 1e-6 {
				return m, invalid("zero normal")
			}

			for slot := 3; slot < 6; slot++ {
				vertex[slot] /= float32(normalLength)
			}

			m.Vertices = append(m.Vertices, vertex...)
		}

		var indices []float32

		if p.Indices != nil {
			if *p.Indices < 0 || *p.Indices >= len(g.Accessors) {
				return m, invalid("index accessor")
			}

			a := g.Accessors[*p.Indices]
			if a.Normalized || a.ComponentType != 5121 && a.ComponentType != 5123 && a.ComponentType != 5125 {
				return m, invalid("unsigned index accessor required")
			}

			indices, err = g.values(ctx, *p.Indices, 1, character.MaxIndices)
			if err != nil {
				return m, err
			}
		} else {
			indices = make([]float32, count)
			for i := range count {
				indices[i] = float32(i)
			}
		}

		if len(indices)%3 != 0 || len(m.Indices) > character.MaxIndices-len(indices) {
			return m, invalid("triangle index count")
		}

		for _, index := range indices {
			if index < 0 || index >= float32(count) || index != float32(int(index)) {
				return m, invalid("triangle index")
			}

			m.Indices = append(m.Indices, uint32(base+int(index)))
		}
	}

	if err := packAtlas(&m, regions); err != nil {
		return m, err
	}

	return m, nil
}
