package dialogue

import (
	"encoding/binary"
	"fmt"
	"math"
	"unicode/utf8"
)

// Records are length-framed, versioned and fully scanned before allocation.
type recordWriter struct {
	data    []byte
	limit   int
	invalid bool
}

func newRecord(magic string, limit int) *recordWriter {
	return &recordWriter{data: append([]byte(magic), 0, 0, 0, 0), limit: limit}
}
func (w *recordWriter) add(n int) []byte {
	if w.invalid || n < 0 || n > w.limit-len(w.data) {
		w.invalid = true
		return nil
	}
	at := len(w.data)
	w.data = append(w.data, make([]byte, n)...)
	return w.data[at:]
}
func (w *recordWriter) u8(v byte) {
	if b := w.add(1); b != nil {
		b[0] = v
	}
}
func (w *recordWriter) u32(v uint32) {
	if b := w.add(4); b != nil {
		binary.LittleEndian.PutUint32(b, v)
	}
}
func (w *recordWriter) integer(v, max int) {
	if v < 0 || v > max {
		w.invalid = true
		return
	}
	w.u32(uint32(v))
}
func (w *recordWriter) str(s string, max int) {
	if len(s) > max || !utf8.ValidString(s) {
		w.invalid = true
		return
	}
	w.u32(uint32(len(s)))
	copy(w.add(len(s)), s)
}
func (w *recordWriter) boolean(v bool) {
	if v {
		w.u8(1)
	} else {
		w.u8(0)
	}
}
func (w *recordWriter) finish() ([]byte, error) {
	if w.invalid {
		return nil, fmt.Errorf("binary dialogue record exceeds bounds")
	}
	binary.LittleEndian.PutUint32(w.data[4:], uint32(len(w.data)-8))
	return w.data, nil
}

type recordReader struct {
	data           []byte
	at             int
	invalid, build bool
}

func readRecord(data []byte, magic string, limit int, build bool) *recordReader {
	r := &recordReader{data: data, at: 8, build: build}
	r.invalid = len(data) < 8 || len(data) > limit
	if !r.invalid {
		r.invalid = string(data[:4]) != magic || uint64(binary.LittleEndian.Uint32(data[4:8])) != uint64(len(data)-8)
	}
	return r
}
func (r *recordReader) take(n int) []byte {
	if r.invalid || n < 0 || n > len(r.data)-r.at {
		r.invalid = true
		return nil
	}
	b := r.data[r.at : r.at+n]
	r.at += n
	return b
}
func (r *recordReader) u8() byte {
	if b := r.take(1); b != nil {
		return b[0]
	}
	return 0
}
func (r *recordReader) u32() uint32 {
	if b := r.take(4); b != nil {
		return binary.LittleEndian.Uint32(b)
	}
	return 0
}
func (r *recordReader) integer(max int) int {
	v := r.u32()
	if uint64(v) > uint64(max) {
		r.invalid = true
		return 0
	}
	return int(v)
}
func (r *recordReader) count(max, minBytes int) int {
	n := r.integer(max)
	if r.invalid || minBytes > 0 && n > (len(r.data)-r.at)/minBytes {
		r.invalid = true
		return 0
	}
	return n
}
func (r *recordReader) str(max int) string {
	n := r.integer(max)
	b := r.take(n)
	if !utf8.Valid(b) {
		r.invalid = true
	}
	if r.build {
		return string(b)
	}
	return ""
}
func (r *recordReader) boolean() bool {
	v := r.u8()
	if v > 1 {
		r.invalid = true
	}
	return v == 1
}
func (r *recordReader) done() error {
	if r.invalid || r.at != len(r.data) {
		return fmt.Errorf("incompatible or malformed binary dialogue record")
	}
	return nil
}
func (w *recordWriter) value(v Value) {
	switch v.Kind {
	case "string":
		w.u8(1)
		w.str(v.String, 4096)
	case "number":
		w.u8(2)
		if !v.valid() {
			w.invalid = true
		}
		w.u32(math.Float32bits(v.Number))
	case "bool":
		w.u8(3)
		w.boolean(v.Bool)
	default:
		w.invalid = true
	}
}
func (r *recordReader) value() Value {
	switch r.u8() {
	case 1:
		return String(r.str(4096))
	case 2:
		v := Number(math.Float32frombits(r.u32()))
		if !v.valid() {
			r.invalid = true
		}
		return v
	case 3:
		return Boolean(r.boolean())
	default:
		r.invalid = true
		return Value{}
	}
}
