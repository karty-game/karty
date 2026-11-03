// Package meshopt runs the pinned upstream decoder during asset compilation.
//
//nolint:gocognit,gocyclo,mnd,varnamelen // This bounded adapter mirrors the pinned upstream decoder ABI and four-vertex alignment.
package meshopt

import (
	"context"
	_ "embed"
	"errors"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

//go:generate go run gen.go

//go:embed decoder.wasm
var decoderWASM []byte

var ErrInvalid = errors.New("invalid bounded Meshopt buffer")

const MaxBytes = 64 << 20

type Decoder struct {
	runtime wazero.Runtime
	module  api.Module
}

func New(ctx context.Context) (*Decoder, error) {
	runtime := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithMemoryLimitPages(1024).WithCloseOnContextDone(true))

	module, err := runtime.InstantiateWithConfig(ctx, decoderWASM, wazero.NewModuleConfig().WithStartFunctions())
	if err != nil {
		_ = runtime.Close(ctx)

		return nil, err
	}

	if _, err = module.ExportedFunction("__wasm_call_ctors").Call(ctx); err != nil {
		_ = runtime.Close(ctx)

		return nil, err
	}

	return &Decoder{runtime: runtime, module: module}, nil
}

func (decoder *Decoder) Close(ctx context.Context) error { return decoder.runtime.Close(ctx) }

func (decoder *Decoder) Decode(ctx context.Context, source []byte, count, stride int, mode, filter string) ([]byte, error) {
	if count <= 0 || stride <= 0 || stride > 256 || count > MaxBytes/stride || len(source) == 0 || len(source) > MaxBytes/2 {
		return nil, ErrInvalid
	}

	var name string

	switch mode {
	case "ATTRIBUTES":
		if stride%4 != 0 {
			return nil, ErrInvalid
		}

		name = "meshopt_decodeVertexBuffer"
	case "TRIANGLES":
		if count%3 != 0 {
			return nil, ErrInvalid
		}

		fallthrough
	case "INDICES":
		if stride != 2 && stride != 4 {
			return nil, ErrInvalid
		}

		name = "meshopt_decodeIndexSequence"
		if mode == "TRIANGLES" {
			name = "meshopt_decodeIndexBuffer"
		}
	default:
		return nil, ErrInvalid
	}

	filters := map[string]string{
		"":            "",
		"NONE":        "",
		"OCTAHEDRAL":  "meshopt_decodeFilterOct",
		"QUATERNION":  "meshopt_decodeFilterQuat",
		"EXPONENTIAL": "meshopt_decodeFilterExp",
	}

	filterFunction, ok := filters[filter]
	if !ok || filterFunction != "" && mode != "ATTRIBUTES" || filter == "OCTAHEDRAL" && stride != 4 && stride != 8 ||
		filter == "QUATERNION" && stride != 8 {
		return nil, ErrInvalid
	}

	allocate := decoder.module.ExportedFunction("sbrk")

	target, err := allocate.Call(ctx, uint64((count+3)&^3)*uint64(stride))
	if err != nil || len(target) != 1 || target[0] > MaxBytes ||
		target[0]+uint64((count+3)&^3)*uint64(stride) > uint64(decoder.module.Memory().Size()) {
		return nil, ErrInvalid
	}
	defer func() {
		if current, callErr := allocate.Call(ctx, 0); callErr == nil {
			_, _ = allocate.Call(ctx, uint64(uint32(target[0]-current[0])))
		}
	}()

	input, err := allocate.Call(ctx, uint64(len(source)))
	if err != nil || len(input) != 1 || input[0] > MaxBytes || !decoder.module.Memory().Write(uint32(input[0]), source) {
		return nil, ErrInvalid
	}

	result, err := decoder.module.ExportedFunction(name).Call(ctx, target[0], uint64(count), uint64(stride), input[0], uint64(len(source)))
	if err != nil || len(result) != 1 || result[0] != 0 {
		return nil, ErrInvalid
	}

	if filterFunction != "" {
		if _, err = decoder.module.ExportedFunction(filterFunction).Call(ctx, target[0], uint64((count+3)&^3), uint64(stride)); err != nil {
			return nil, ErrInvalid
		}
	}

	output, ok := decoder.module.Memory().Read(uint32(target[0]), uint32(count*stride))
	if !ok {
		return nil, ErrInvalid
	}

	return append([]byte(nil), output...), nil
}
