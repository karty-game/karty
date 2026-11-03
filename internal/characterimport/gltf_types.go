package characterimport

import (
	"encoding/json"

	"github.com/karty-game/karty/internal/characterimport/meshopt"
)

type gltf struct {
	Asset struct {
		Version string `json:"version"`
	} `json:"asset"`
	ExtensionsRequired []string `json:"extensionsRequired"`
	Buffers            []struct {
		URI        string `json:"uri"`
		ByteLength int    `json:"byteLength"`
	} `json:"buffers"`
	BufferViews []view     `json:"bufferViews"`
	Accessors   []accessor `json:"accessors"`
	Nodes       []node     `json:"nodes"`
	Meshes      []struct {
		Primitives []primitive `json:"primitives"`
		Weights    []float32   `json:"weights"`
	} `json:"meshes"`
	Skins []struct {
		Joints              []int `json:"joints"`
		InverseBindMatrices *int  `json:"inverseBindMatrices"`
	} `json:"skins"`
	Images []struct {
		URI        string `json:"uri"`
		MimeType   string `json:"mimeType"`
		BufferView *int   `json:"bufferView"`
	} `json:"images"`
	Textures []struct {
		Source     *int                       `json:"source"`
		Sampler    *int                       `json:"sampler"`
		Extensions map[string]json.RawMessage `json:"extensions"`
	} `json:"textures"`
	Samplers []struct {
		WrapS int `json:"wrapS"`
		WrapT int `json:"wrapT"`
	} `json:"samplers"`
	Materials    []material  `json:"materials"`
	Animations   []animation `json:"animations"`
	bin          []byte
	decoded      map[int][]byte
	decodedBytes int
	atlasPixels  int
	decoder      *meshopt.Decoder
}
type view struct {
	Buffer     int                        `json:"buffer"`
	ByteOffset int                        `json:"byteOffset"`
	ByteLength int                        `json:"byteLength"`
	ByteStride int                        `json:"byteStride"`
	Extensions map[string]json.RawMessage `json:"extensions"`
}
type accessor struct {
	BufferView    *int            `json:"bufferView"`
	ByteOffset    int             `json:"byteOffset"`
	ComponentType int             `json:"componentType"`
	Count         int             `json:"count"`
	Type          string          `json:"type"`
	Normalized    bool            `json:"normalized"`
	Sparse        json.RawMessage `json:"sparse"`
}
type node struct {
	Name        string    `json:"name"`
	Children    []int     `json:"children"`
	Mesh        *int      `json:"mesh"`
	Skin        *int      `json:"skin"`
	Translation []float32 `json:"translation"`
	Rotation    []float32 `json:"rotation"`
	Scale       []float32 `json:"scale"`
	Matrix      []float32 `json:"matrix"`
	Weights     []float32 `json:"weights"`
}
type primitive struct {
	Attributes map[string]int             `json:"attributes"`
	Indices    *int                       `json:"indices"`
	Material   *int                       `json:"material"`
	Mode       *int                       `json:"mode"`
	Targets    []json.RawMessage          `json:"targets"`
	Extensions map[string]json.RawMessage `json:"extensions"`
}
type textureInfo struct {
	Index      int                        `json:"index"`
	TexCoord   int                        `json:"texCoord"`
	Extensions map[string]json.RawMessage `json:"extensions"`
}
type material struct {
	AlphaMode            string                     `json:"alphaMode"`
	DoubleSided          bool                       `json:"doubleSided"`
	NormalTexture        json.RawMessage            `json:"normalTexture"`
	OcclusionTexture     json.RawMessage            `json:"occlusionTexture"`
	EmissiveTexture      json.RawMessage            `json:"emissiveTexture"`
	EmissiveFactor       []float32                  `json:"emissiveFactor"`
	Extensions           map[string]json.RawMessage `json:"extensions"`
	PBRMetallicRoughness struct {
		BaseColorFactor          []float32       `json:"baseColorFactor"`
		BaseColorTexture         *textureInfo    `json:"baseColorTexture"`
		MetallicRoughnessTexture json.RawMessage `json:"metallicRoughnessTexture"`
	} `json:"pbrMetallicRoughness"`
}
type animation struct {
	Name     string `json:"name"`
	Samplers []struct {
		Input         int    `json:"input"`
		Output        int    `json:"output"`
		Interpolation string `json:"interpolation"`
	} `json:"samplers"`
	Channels []struct {
		Sampler int `json:"sampler"`
		Target  struct {
			Node *int   `json:"node"`
			Path string `json:"path"`
		} `json:"target"`
	} `json:"channels"`
}

type compressedView struct {
	Buffer     int    `json:"buffer"`
	ByteOffset int    `json:"byteOffset"`
	ByteLength int    `json:"byteLength"`
	ByteStride int    `json:"byteStride"`
	Count      int    `json:"count"`
	Mode       string `json:"mode"`
	Filter     string `json:"filter"`
}
type textureTransform struct {
	Offset   []float64 `json:"offset"`
	Scale    []float64 `json:"scale"`
	Rotation float64   `json:"rotation"`
	TexCoord int       `json:"texCoord"`
}
