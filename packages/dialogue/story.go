package dialogue

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

const MaxSteps = 2048

type Choice struct {
	ID          int
	Line        Line
	Enabled     bool
	Destination int
}
type Event struct {
	Kind    string
	Line    Line
	Command string
	Choices []Choice
}
type callFrame struct {
	Node string
	PC   int
}
type state struct {
	Node      string
	PC        int
	Variables map[string]Value
	Stack     []Value
	Calls     []callFrame
	Options   []Choice
	Waiting   bool
	Done      bool
	Visits    map[string]int
}
type Story struct {
	program   *Program
	state     state
	functions map[string]func([]Value) (Value, error)
}

func NewStory(program *Program) *Story {
	s := &Story{program: program, functions: map[string]func([]Value) (Value, error){}}
	s.state = state{Variables: map[string]Value{}, Visits: map[string]int{}, Done: true}
	if program != nil {
		for key, value := range program.initial {
			s.state.Variables[key] = value
		}
	}
	return s
}
func (s *Story) Start(name string) error {
	if s.program == nil {
		return fmt.Errorf("missing Yarn program")
	}
	if _, ok := s.program.nodes[name]; !ok {
		return fmt.Errorf("missing Yarn node %s", name)
	}
	s.state.Node = name
	s.state.PC = 0
	s.state.Stack = nil
	s.state.Calls = nil
	s.state.Options = nil
	s.state.Waiting = false
	s.state.Done = false
	return nil
}
func (s *Story) Variable(name string) (Value, bool) {
	value, ok := s.state.Variables[name]
	return value, ok
}
func (s *Story) Register(name string, function func([]Value) (Value, error)) {
	s.functions[name] = function
}
func (s *Story) Choose(id int) error {
	if !s.state.Waiting {
		return fmt.Errorf("Yarn is not waiting for a choice")
	}
	if id == -1 {
		for _, choice := range s.state.Options {
			if choice.Enabled {
				return fmt.Errorf("an available Yarn choice must be selected")
			}
		}
		s.state.Stack = append(s.state.Stack, Boolean(false))
		s.state.Options = nil
		s.state.Waiting = false
		return nil
	}
	for _, choice := range s.state.Options {
		if choice.ID == id && choice.Enabled {
			s.state.Stack = append(s.state.Stack, Number(float32(choice.Destination)), Boolean(true))
			s.state.Options = nil
			s.state.Waiting = false
			return nil
		}
	}
	return fmt.Errorf("invalid or unavailable Yarn choice")
}
func (s *Story) pop() (Value, error) {
	if len(s.state.Stack) == 0 {
		return Value{}, fmt.Errorf("Yarn stack underflow")
	}
	n := len(s.state.Stack) - 1
	v := s.state.Stack[n]
	s.state.Stack = s.state.Stack[:n]
	return v, nil
}
func (s *Story) peek() (Value, error) {
	if len(s.state.Stack) == 0 {
		return Value{}, fmt.Errorf("Yarn stack underflow")
	}
	return s.state.Stack[len(s.state.Stack)-1], nil
}
func valueText(value Value) string {
	switch value.Kind {
	case "string":
		return value.String
	case "number":
		return strconv.FormatFloat(float64(value.Number), 'f', -1, 32)
	case "bool":
		if value.Bool {
			return "True"
		}
		return "False"
	}
	return ""
}
func (s *Story) substitute(text string, count int) (string, error) {
	if count > len(s.state.Stack) {
		return "", fmt.Errorf("Yarn substitution stack underflow")
	}
	replacements := make([]string, 2*count)
	for n := count - 1; n >= 0; n-- {
		v, _ := s.pop()
		replacements[n*2] = "{" + strconv.Itoa(n) + "}"
		replacements[n*2+1] = valueText(v)
	}
	if count > 0 {
		text = strings.NewReplacer(replacements...).Replace(text)
	}
	if len(text) > 4096 {
		return "", fmt.Errorf("Yarn presented text exceeds 4096 bytes")
	}
	return text, nil
}
func (s *Story) completeNode() {
	s.state.Visits[s.state.Node]++
	n := s.program.nodes[s.state.Node]
	if variable := n.headers["tracking"]; strings.HasPrefix(variable, "$") {
		s.state.Variables[variable] = Number(float32(s.state.Visits[s.state.Node]))
	}
}

// Continue advances until a line, choices, command or completion. A bounded
// instruction budget makes malformed loops fail instead of hanging a game tick.
func (s *Story) Continue() (event Event, err error) {
	if s.state.Waiting {
		return Event{}, fmt.Errorf("choose an option before continuing")
	}
	if s.state.Done {
		return Event{Kind: "complete"}, nil
	}
	defer func() {
		if err != nil {
			s.state.Done = true
		}
	}()
	for step := 0; step < MaxSteps; step++ {
		n := s.program.nodes[s.state.Node]
		if s.state.PC < 0 || s.state.PC >= len(n.instructions) {
			return Event{}, fmt.Errorf("Yarn instruction pointer out of range")
		}
		i := n.instructions[s.state.PC]
		s.state.PC++
		switch i.op {
		case 1:
			s.state.PC = i.destination
		case 2:
			v, e := s.peek()
			if e != nil {
				return Event{}, e
			}
			if v.Kind != "number" || v.Number < 0 || int(v.Number) >= len(n.instructions) {
				return Event{}, fmt.Errorf("invalid indirect jump")
			}
			s.state.PC = int(v.Number)
		case 3, 4, 5:
			line := s.program.lines[i.text]
			text := i.text
			if i.op != 4 {
				text = line.Text
			}
			text, e := s.substitute(text, i.count)
			if e != nil {
				return Event{}, e
			}
			line.Text = text
			if i.op == 3 {
				return Event{Kind: "line", Line: line}, nil
			}
			if i.op == 4 {
				return Event{Kind: "command", Command: text}, nil
			}
			enabled := true
			if i.condition {
				v, e := s.pop()
				if e != nil {
					return Event{}, e
				}
				if v.Kind != "bool" {
					return Event{}, fmt.Errorf("option condition must be boolean")
				}
				enabled = v.Bool
			}
			if len(s.state.Options) >= 128 {
				return Event{}, fmt.Errorf("Yarn option limit exceeded")
			}
			s.state.Options = append(
				s.state.Options,
				Choice{ID: len(s.state.Options), Line: line, Enabled: enabled, Destination: i.destination},
			)
		case 6:
			if len(s.state.Options) == 0 {
				s.state.Done = true
				return Event{Kind: "complete"}, nil
			}
			s.state.Waiting = true
			return Event{Kind: "choices", Choices: append([]Choice(nil), s.state.Options...)}, nil
		case 7:
			s.state.Stack = append(s.state.Stack, String(i.text))
		case 8:
			s.state.Stack = append(s.state.Stack, Number(i.number))
		case 9:
			s.state.Stack = append(s.state.Stack, Boolean(i.flag))
		case 10:
			v, e := s.peek()
			if e != nil {
				return Event{}, e
			}
			if v.Kind != "bool" {
				return Event{}, fmt.Errorf("condition must be boolean")
			}
			if !v.Bool {
				s.state.PC = i.destination
			}
		case 11:
			if _, e := s.pop(); e != nil {
				return Event{}, e
			}
		case 12:
			if e := s.call(i.text); e != nil {
				return Event{}, e
			}
		case 13:
			value, ok := s.state.Variables[i.text]
			if !ok {
				return Event{}, fmt.Errorf("undefined variable %s (smart variables are unsupported)", i.text)
			}
			s.state.Stack = append(s.state.Stack, value)
		case 14:
			v, e := s.peek()
			if e != nil {
				return Event{}, e
			}
			if prior, ok := s.state.Variables[i.text]; !ok || prior.Kind != v.Kind {
				return Event{}, fmt.Errorf("undefined variable or changed type: %s", i.text)
			}
			s.state.Variables[i.text] = v
		case 15:
			s.completeNode()
			for _, frame := range s.state.Calls {
				s.state.Node = frame.Node
				s.completeNode()
			}
			s.state.Done = true
			s.state.Calls = nil
			return Event{Kind: "complete"}, nil
		case 16, 17, 18, 19:
			target := i.text
			if i.op == 17 || i.op == 19 {
				v, e := s.peek()
				if e != nil {
					return Event{}, e
				}
				if v.Kind != "string" {
					return Event{}, fmt.Errorf("node destination must be string")
				}
				target = v.String
			}
			if _, ok := s.program.nodes[target]; !ok {
				return Event{}, fmt.Errorf("unknown Yarn node %s", target)
			}
			if i.op == 18 || i.op == 19 {
				if len(s.state.Calls) >= 64 {
					return Event{}, fmt.Errorf("Yarn detour depth exceeded")
				}
				s.state.Calls = append(s.state.Calls, callFrame{s.state.Node, s.state.PC})
			} else {
				s.completeNode()
				for _, frame := range s.state.Calls {
					s.state.Node = frame.Node
					s.completeNode()
				}
				s.state.Calls = nil
				s.state.Stack = nil
				s.state.Options = nil
			}
			s.state.Node = target
			s.state.PC = 0
		case 20:
			s.completeNode()
			if len(s.state.Calls) == 0 {
				s.state.Done = true
				return Event{Kind: "complete"}, nil
			}
			last := len(s.state.Calls) - 1
			frame := s.state.Calls[last]
			s.state.Calls = s.state.Calls[:last]
			s.state.Node = frame.Node
			s.state.PC = frame.PC
		default:
			return Event{}, fmt.Errorf("unsupported Yarn operation %d", i.op)
		}
		if len(s.state.Stack) > 1024 {
			return Event{}, fmt.Errorf("Yarn stack limit exceeded")
		}
	}
	return Event{}, fmt.Errorf("Yarn instruction budget exceeded (%d)", MaxSteps)
}

type snapshot struct {
	Version int
	Program string
	State   state
}

func (s *Story) Save() ([]byte, error) {
	if s.program == nil {
		return nil, fmt.Errorf("missing Yarn program")
	}
	data, err := marshalSnapshot(snapshot{1, s.program.fingerprint, s.state})
	if len(data) > 48*1024 {
		return nil, fmt.Errorf("Yarn snapshot exceeds 48 KiB")
	}
	return data, err
}
func (s *Story) Restore(data []byte) error {
	if s.program == nil || len(data) > 48*1024 {
		return fmt.Errorf("invalid Yarn snapshot")
	}
	saved, err := unmarshalSnapshot(data)
	if err != nil {
		return err
	}
	state := saved.State
	if saved.Version != 1 || saved.Program != s.program.fingerprint || state.Variables == nil || state.Visits == nil ||
		len(state.Variables) != len(s.program.initial) ||
		len(state.Visits) > len(s.program.nodes) ||
		len(state.Stack) > 1024 ||
		len(state.Options) > 128 ||
		len(state.Calls) > 64 ||
		state.Waiting && len(state.Options) == 0 {
		return fmt.Errorf("Yarn snapshot version, program or bounds mismatch")
	}
	n, ok := s.program.nodes[state.Node]
	if !ok || state.PC < 0 || state.PC > len(n.instructions) {
		return fmt.Errorf("invalid saved Yarn position")
	}
	for key, value := range state.Variables {
		initial, ok := s.program.initial[key]
		if !ok || initial.Kind != value.Kind || !value.valid() {
			return fmt.Errorf("invalid saved Yarn variable %s", key)
		}
	}
	for name, count := range state.Visits {
		if _, ok := s.program.nodes[name]; !ok || count < 0 || count > 1<<24 {
			return fmt.Errorf("invalid saved Yarn visit")
		}
	}
	if state.Waiting && state.Done {
		return fmt.Errorf("invalid completed Yarn choice")
	}
	for _, v := range state.Stack {
		if !v.valid() {
			return fmt.Errorf("invalid saved Yarn stack")
		}
	}
	for _, frame := range state.Calls {
		n, ok := s.program.nodes[frame.Node]
		if !ok || frame.PC < 0 || frame.PC > len(n.instructions) {
			return fmt.Errorf("invalid saved Yarn detour")
		}
	}
	for index, choice := range state.Options {
		if choice.ID != index || choice.Destination < 0 || choice.Destination >= len(n.instructions) {
			return fmt.Errorf("invalid saved Yarn choice")
		}
		if _, ok := s.program.lines[choice.Line.ID]; !ok || len(choice.Line.Text) > 4096 || !utf8.ValidString(choice.Line.Text) ||
			len(choice.Line.Tags) > 128 {
			return fmt.Errorf("invalid saved Yarn line")
		}
		for _, tag := range choice.Line.Tags {
			if len(tag) > 256 || !utf8.ValidString(tag) {
				return fmt.Errorf("invalid saved Yarn metadata")
			}
		}
	}
	s.state = state
	return nil
}
