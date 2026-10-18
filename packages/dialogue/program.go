// Program loading for the supported Yarn Spinner 3 compiled-program subset.
// It is portable Go and does not import a game's generated engine bindings.
package dialogue

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"unicode/utf8"
)

const MaxProgramBytes = 4 << 20
const MaxStringBytes = 4 << 20
const MaxInstructions = 65536
const MaxNodes = 1024

type Value struct {
	Kind   string
	String string
	Number float32
	Bool   bool
}

func String(value string) Value  { return Value{Kind: "string", String: value} }
func Number(value float32) Value { return Value{Kind: "number", Number: value} }
func Boolean(value bool) Value   { return Value{Kind: "bool", Bool: value} }
func (value Value) valid() bool {
	switch value.Kind {
	case "string":
		return utf8.ValidString(value.String) && len(value.String) <= 4096
	case "number":
		return !math.IsNaN(float64(value.Number)) && !math.IsInf(float64(value.Number), 0)
	case "bool":
		return true
	}
	return false
}

type instruction struct {
	op                 int
	text               string
	number             float32
	flag               bool
	destination, count int
	condition          bool
}
type node struct {
	instructions []instruction
	headers      map[string]string
}
type Line struct {
	ID   string
	Text string
	Tags []string
}
type Program struct {
	nodes       map[string]node
	initial     map[string]Value
	lines       map[string]Line
	fingerprint string
}

// Load accepts official Yarn Spinner 3.2.1 .yarnc bytes and the compiler's string
// table. V2 protobufs and unsupported saliency instructions fail explicitly.
func Load(compiled, table []byte) (*Program, error) {
	if len(compiled) == 0 || len(compiled) > MaxProgramBytes || len(table) > MaxStringBytes {
		return nil, fmt.Errorf("Yarn program exceeds limits")
	}
	program := &Program{nodes: map[string]node{}, initial: map[string]Value{}, lines: map[string]Line{}}
	var err error
	program.lines, err = loadStrings(table)
	if err != nil {
		return nil, err
	}
	version := 0
	instructions := 0
	err = fields(compiled, func(field wireField) error {
		switch field.number {
		case 2:
			if field.wire != 2 {
				return fmt.Errorf("invalid node map")
			}
			key, value, err := mapEntry(field.data)
			if err != nil {
				return err
			}
			if _, exists := program.nodes[key]; exists || key == "" || len(key) > 256 || len(program.nodes) >= MaxNodes {
				return fmt.Errorf("invalid or duplicate Yarn node")
			}
			n := node{headers: map[string]string{}}
			err = fields(value, func(part wireField) error {
				switch part.number {
				case 2, 3, 4, 5:
					return fmt.Errorf("Yarn Spinner 2 programs are not supported")
				case 6:
					k, v, e := mapEntry(part.data)
					if e != nil {
						return e
					}
					n.headers[k] = string(v)
				case 7:
					i, e := decodeInstruction(part.data)
					if e != nil {
						return e
					}
					n.instructions = append(n.instructions, i)
					instructions++
					if instructions > MaxInstructions {
						return fmt.Errorf("Yarn instruction limit exceeded")
					}
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("node %s: %w", key, err)
			}
			program.nodes[key] = n
		case 3:
			key, value, err := mapEntry(field.data)
			if err != nil {
				return err
			}
			v, err := decodeValue(value)
			if err != nil {
				return err
			}
			if len(program.initial) >= 1024 {
				return fmt.Errorf("Yarn variable limit exceeded")
			}
			program.initial[key] = v
		case 4:
			if field.wire != 0 {
				return fmt.Errorf("invalid language version")
			}
			version = int(field.value)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if version != 3 || len(program.nodes) == 0 {
		return nil, fmt.Errorf("expected Yarn Spinner 3 program, language version %d", version)
	}
	for name, n := range program.nodes {
		for _, i := range n.instructions {
			if (i.op == 1 || i.op == 10 || i.op == 5) && (i.destination < 0 || i.destination >= len(n.instructions)) {
				return nil, fmt.Errorf("node %s: invalid instruction destination", name)
			}
			if i.op == 3 || i.op == 5 {
				if _, ok := program.lines[i.text]; !ok {
					return nil, fmt.Errorf("node %s: missing line %s", name, i.text)
				}
			}
			if i.op == 16 || i.op == 18 {
				if _, ok := program.nodes[i.text]; !ok {
					return nil, fmt.Errorf("missing destination node %s", i.text)
				}
			}
		}
	}
	digest := sha256.Sum256(compiled)
	program.fingerprint = hex.EncodeToString(digest[:])
	return program, nil
}

type wireField struct {
	number, wire int
	value        uint64
	data         []byte
}

func varint(data []byte) (uint64, int, error) {
	var value uint64
	for index := 0; index < 10 && index < len(data); index++ {
		b := data[index]
		if index == 9 && b > 1 {
			return 0, 0, fmt.Errorf("protobuf varint overflow")
		}
		value |= uint64(b&127) << uint(index*7)
		if b < 128 {
			return value, index + 1, nil
		}
	}
	return 0, 0, fmt.Errorf("truncated protobuf varint")
}
func fields(data []byte, visit func(wireField) error) error {
	for len(data) > 0 {
		tag, n, err := varint(data)
		if err != nil {
			return err
		}
		data = data[n:]
		field := wireField{number: int(tag >> 3), wire: int(tag & 7)}
		if field.number == 0 {
			return fmt.Errorf("invalid protobuf field")
		}
		switch field.wire {
		case 0:
			field.value, n, err = varint(data)
			if err != nil {
				return err
			}
			data = data[n:]
		case 1:
			if len(data) < 8 {
				return fmt.Errorf("truncated protobuf fixed64")
			}
			field.data = data[:8]
			data = data[8:]
		case 2:
			length, size, e := varint(data)
			if e != nil || length > uint64(len(data)-size) {
				return fmt.Errorf("truncated protobuf bytes")
			}
			data = data[size:]
			field.data = data[:int(length)]
			data = data[int(length):]
		case 5:
			if len(data) < 4 {
				return fmt.Errorf("truncated protobuf float")
			}
			field.data = data[:4]
			data = data[4:]
		default:
			return fmt.Errorf("unsupported protobuf wire type")
		}
		if err := visit(field); err != nil {
			return err
		}
	}
	return nil
}
func mapEntry(data []byte) (key string, value []byte, err error) {
	err = fields(data, func(f wireField) error {
		if f.wire != 2 {
			return fmt.Errorf("invalid protobuf map")
		}
		switch f.number {
		case 1:
			key = string(f.data)
		case 2:
			value = f.data
		}
		return nil
	})
	return
}
func floatValue(data []byte) float32 {
	return math.Float32frombits(uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24)
}
func decodeValue(data []byte) (Value, error) {
	var value Value
	err := fields(data, func(f wireField) error {
		switch {
		case f.number == 1 && f.wire == 2:
			value = String(string(f.data))
		case f.number == 2 && f.wire == 0:
			value = Boolean(f.value != 0)
		case f.number == 3 && f.wire == 5:
			value = Number(floatValue(f.data))
		default:
			return fmt.Errorf("unsupported Yarn operand")
		}
		return nil
	})
	if err != nil {
		return Value{}, err
	}
	if !value.valid() {
		return Value{}, fmt.Errorf("invalid Yarn operand")
	}
	return value, nil
}
func decodeInstruction(data []byte) (instruction, error) {
	var i instruction
	err := fields(data, func(f wireField) error {
		if i.op != 0 || f.wire != 2 || f.number < 1 || f.number > 20 {
			return fmt.Errorf("unsupported Yarn instruction %d (saliency/node groups require another runtime)", f.number)
		}
		i.op = f.number
		return fields(f.data, func(p wireField) error {
			switch p.wire {
			case 2:
				i.text = string(p.data)
			case 5:
				i.number = floatValue(p.data)
			case 0:
				switch i.op {
				case 1, 10:
					i.destination = int(p.value)
				case 3, 4:
					i.count = int(p.value)
				case 5:
					switch p.number {
					case 2:
						i.destination = int(p.value)
					case 3:
						i.count = int(p.value)
					case 4:
						i.condition = p.value != 0
					}
				case 9:
					i.flag = p.value != 0
				}
			default:
				return fmt.Errorf("invalid Yarn instruction operand")
			}
			return nil
		})
	})
	if err != nil {
		return instruction{}, err
	}
	if i.op == 0 || i.count < 0 || i.count > 128 {
		return instruction{}, fmt.Errorf("invalid Yarn instruction")
	}
	return i, nil
}
