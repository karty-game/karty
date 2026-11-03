package levelbuild

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash"
	"math"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

// GPU cells ship final GPU payloads, not the legacy semantic mesh wire format.
// CompileReady hashes an opaque source identity. Validate the full source mesh
// and hash explicit fields without allocating an unused legacy payload or
// applying that payload's smaller delivery bound to whole rooms.
func gpuSourceMeshIdentity(mesh *worldchunk.Mesh, document *world.Document) ([]byte, error) {
	if err := worldchunk.ValidateMesh(mesh, document); err != nil {
		return nil, err
	}

	writer := gpuIdentityWriter{digest: sha256.New()}
	writer.text("karty.gpu-source-identity@1")
	writer.text(mesh.Schema)
	writer.text(mesh.GeometrySHA256)
	writer.text(mesh.SurfaceSHA256)
	writer.integers(mesh.Sectors)
	writer.integers(mesh.Solids)
	writer.f64(mesh.Bounds[:])
	writer.selection(mesh.Selection)
	writer.number(uint64(len(mesh.Vertices)))

	for _, vertex := range mesh.Vertices {
		writer.f32(vertex.Position[:])

		for _, uv := range vertex.UV {
			writer.f32(uv[:])
		}

		writer.f32(vertex.BandUV[:])

		for _, uv := range vertex.SecondaryUV {
			writer.f32(uv[:])
		}

		writer.f32(vertex.LightmapUV[:])
		writer.f32(vertex.WaveUV[:])
	}

	writer.number(uint64(len(mesh.Indices)))

	for _, index := range mesh.Indices {
		writer.number(uint64(index))
	}

	writer.number(uint64(len(mesh.Triangles)))

	for _, triangle := range mesh.Triangles {
		writer.triangle(triangle)
	}

	if writer.bytes > gpuMaxSourceMeshBytes {
		return nil, fmt.Errorf("GPU source identity exceeds %d bytes: %w", gpuMaxSourceMeshBytes, worldchunk.ErrChunk)
	}

	return writer.digest.Sum(nil), nil
}

type gpuIdentityWriter struct {
	digest  hash.Hash
	bytes   int
	scratch [8]byte
}

func (writer *gpuIdentityWriter) number(value uint64) {
	binary.LittleEndian.PutUint64(writer.scratch[:], value)
	_, _ = writer.digest.Write(writer.scratch[:])
	writer.bytes += len(writer.scratch)
}

func (writer *gpuIdentityWriter) integer(value int) {
	// #nosec G115 -- the identity preserves the signed integer's two's-complement bits.
	writer.number(uint64(value))
}

func (writer *gpuIdentityWriter) text(value string) {
	writer.number(uint64(len(value)))
	_, _ = writer.digest.Write([]byte(value))
	writer.bytes += len(value)
}

func (writer *gpuIdentityWriter) integers(values []int) {
	writer.number(uint64(len(values)))

	for _, value := range values {
		writer.integer(value)
	}
}

func (writer *gpuIdentityWriter) f32(values []float32) {
	for _, value := range values {
		writer.number(uint64(math.Float32bits(value)))
	}
}

func (writer *gpuIdentityWriter) f64(values []float64) {
	for _, value := range values {
		writer.number(math.Float64bits(value))
	}
}

func (writer *gpuIdentityWriter) boolean(value bool) {
	if value {
		writer.number(1)
	} else {
		writer.number(0)
	}
}

func (writer *gpuIdentityWriter) basis(value worldchunk.Basis) {
	writer.f32(value.Tangent[:])
	writer.f32(value.Bitangent[:])
}

func (writer *gpuIdentityWriter) triangle(value worldchunk.Triangle) {
	writer.integer(value.Sector)
	writer.integer(value.Solid)
	writer.text(value.Surface)
	writer.integer(value.SurfaceEdge)
	writer.f32(value.Normal[:])
	writer.f32(value.Tangent[:])
	writer.f32(value.Bitangent[:])
	writer.number(uint64(value.Material))
	writer.number(uint64(value.MappingCount))
	writer.f32(value.Weights[:])

	for _, basis := range value.ProjectionBases {
		writer.basis(basis)
	}

	writer.number(uint64(value.Band.Material))
	writer.number(uint64(value.Band.Coverage))
	writer.boolean(value.Band.RepeatU)
	writer.boolean(value.Band.RepeatV)
	writer.basis(value.Band.Basis)
	writer.number(uint64(value.Secondary.Material))
	writer.f32([]float32{value.Secondary.Strength})
	writer.number(uint64(value.Secondary.MappingCount))
	writer.f32(value.Secondary.Weights[:])
	writer.boolean(value.Sky)
	writer.boolean(value.Outdoor)
	writer.integer(value.Chart)
}

func (writer *gpuIdentityWriter) selection(value *worldlightmap.SurfaceSelection) {
	writer.boolean(value != nil)

	if value == nil {
		return
	}

	writer.f64([]float64{value.LiquidEdgeStep})
	writer.integer(value.LiquidMaxSubdivisions)
	writer.number(uint64(len(value.LiquidTiles)))

	for _, tile := range value.LiquidTiles {
		writer.f64(tile.Clip[:])
		writer.integer(tile.LiquidMaxSubdivisions)
	}

	writer.number(uint64(len(value.Bindings)))

	for _, binding := range value.Bindings {
		writer.text(binding.Kind)
		writer.integer(binding.Index)
		writer.integer(binding.Edge)
		writer.integer(binding.Chart)
	}

	writer.boolean(value.Clip != nil)

	if value.Clip != nil {
		writer.f64(value.Clip[:])
	}
}
