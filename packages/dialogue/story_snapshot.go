package dialogue

import (
	"fmt"
	"slices"
)

func marshalSnapshot(saved snapshot) ([]byte, error) {
	s := saved.State
	w := newRecord("YSS2", 48*1024)
	w.str(saved.Program, 64)
	w.str(s.Node, 256)
	w.integer(s.PC, MaxInstructions)
	w.boolean(s.Waiting)
	w.boolean(s.Done)
	if len(s.Variables)+len(s.Stack)+len(s.Calls)+len(s.Options)+len(s.Visits) > 2399 {
		return nil, fmt.Errorf("Yarn snapshot record limit exceeded")
	}
	names := make([]string, 0, len(s.Variables))
	for name := range s.Variables {
		names = append(names, name)
	}
	slices.Sort(names)
	w.integer(len(names), 1024)
	for _, name := range names {
		w.str(name, 256)
		w.value(s.Variables[name])
	}
	w.integer(len(s.Stack), 1024)
	for _, v := range s.Stack {
		w.value(v)
	}
	w.integer(len(s.Calls), 64)
	for _, f := range s.Calls {
		w.str(f.Node, 256)
		w.integer(f.PC, MaxInstructions)
	}
	w.integer(len(s.Options), 128)
	for _, c := range s.Options {
		w.integer(c.ID, 65535)
		w.str(c.Line.ID, 256)
		w.str(c.Line.Text, 4096)
		w.boolean(c.Enabled)
		w.integer(c.Destination, MaxInstructions)
		w.integer(len(c.Line.Tags), 128)
		for _, tag := range c.Line.Tags {
			w.str(tag, 256)
		}
	}
	names = names[:0]
	for name := range s.Visits {
		names = append(names, name)
	}
	slices.Sort(names)
	w.integer(len(names), 1024)
	for _, name := range names {
		w.str(name, 256)
		w.integer(s.Visits[name], 1<<24)
	}
	return w.finish()
}
func unmarshalSnapshot(data []byte) (snapshot, error) {
	if _, err := decodeSnapshot(data, false); err != nil {
		return snapshot{}, err
	}
	return decodeSnapshot(data, true)
}
func decodeSnapshot(data []byte, build bool) (snapshot, error) {
	r := readRecord(data, "YSS2", 48*1024, build)
	saved := snapshot{Version: 1, Program: r.str(64)}
	s := state{Node: r.str(256), PC: r.integer(MaxInstructions), Waiting: r.boolean(), Done: r.boolean()}
	if build {
		s.Variables = map[string]Value{}
		s.Visits = map[string]int{}
	}
	total := 0
	n := r.count(1024, 6)
	total += n
	for i := 0; i < n; i++ {
		name := r.str(256)
		value := r.value()
		if build {
			if _, ok := s.Variables[name]; ok {
				r.invalid = true
			}
			s.Variables[name] = value
		}
	}
	n = r.count(1024, 2)
	total += n
	for i := 0; i < n; i++ {
		v := r.value()
		if build {
			s.Stack = append(s.Stack, v)
		}
	}
	n = r.count(64, 8)
	total += n
	for i := 0; i < n; i++ {
		f := callFrame{r.str(256), r.integer(MaxInstructions)}
		if build {
			s.Calls = append(s.Calls, f)
		}
	}
	n = r.count(128, 21)
	total += n
	for i := 0; i < n; i++ {
		c := Choice{
			ID:          r.integer(65535),
			Line:        Line{ID: r.str(256), Text: r.str(4096)},
			Enabled:     r.boolean(),
			Destination: r.integer(MaxInstructions),
		}
		tags := r.count(128, 4)
		for j := 0; j < tags; j++ {
			tag := r.str(256)
			if build {
				c.Line.Tags = append(c.Line.Tags, tag)
			}
		}
		if build {
			s.Options = append(s.Options, c)
		}
	}
	n = r.count(1024, 8)
	total += n
	for i := 0; i < n; i++ {
		name := r.str(256)
		visits := r.integer(1 << 24)
		if build {
			if _, ok := s.Visits[name]; ok {
				r.invalid = true
			}
			s.Visits[name] = visits
		}
	}
	if total > 2399 {
		r.invalid = true
	}
	saved.State = s
	return saved, r.done()
}
