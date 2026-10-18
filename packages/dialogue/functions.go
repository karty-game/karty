package dialogue

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func (s *Story) call(name string) error {
	count, err := s.pop()
	if err != nil {
		return err
	}
	if count.Kind != "number" || count.Number < 0 || count.Number > 128 || count.Number != float32(int(count.Number)) {
		return fmt.Errorf("invalid Yarn function arity")
	}
	args := make([]Value, int(count.Number))
	for n := len(args) - 1; n >= 0; n-- {
		value, e := s.pop()
		if e != nil {
			return e
		}
		args[n] = value
	}
	var value Value
	if custom := s.functions[name]; custom != nil {
		value, err = custom(args)
	} else {
		value, err = s.builtin(name, args)
	}
	if err != nil {
		return err
	}
	if !value.valid() {
		return fmt.Errorf("invalid result from Yarn function %s", name)
	}
	s.state.Stack = append(s.state.Stack, value)
	return nil
}
func (s *Story) builtin(name string, args []Value) (Value, error) {
	if strings.HasPrefix(name, "Bool.") {
		name = "Boolean." + strings.TrimPrefix(name, "Bool.")
	}
	fail := func() (Value, error) {
		return Value{}, fmt.Errorf("unsupported Yarn function or argument types: %s", name)
	}
	if len(args) == 1 {
		v := args[0]
		switch name {
		case "visited", "visited_count":
			if v.Kind != "string" {
				return fail()
			}
			count := s.state.Visits[v.String]
			if name == "visited" {
				return Boolean(count > 0), nil
			}
			return Number(float32(count)), nil
		case "string", "format_invariant":
			return String(valueText(v)), nil
		case "number":
			if v.Kind == "number" {
				return v, nil
			}
			n, err := strconv.ParseFloat(v.String, 32)
			if err != nil {
				return fail()
			}
			return Number(float32(n)), nil
		case "bool":
			switch v.Kind {
			case "bool":
				return v, nil
			case "number":
				return Boolean(v.Number != 0), nil
			case "string":
				n, err := strconv.ParseBool(v.String)
				if err != nil {
					return fail()
				}
				return Boolean(n), nil
			}
		case "Boolean.Not":
			if v.Kind == "bool" {
				return Boolean(!v.Bool), nil
			}
		case "Number.UnaryMinus":
			if v.Kind == "number" {
				return Number(-v.Number), nil
			}
		case "round", "floor", "ceil", "int", "decimal", "inc", "dec":
			if v.Kind != "number" {
				return fail()
			}
			n := float64(v.Number)
			switch name {
			case "round":
				n = math.RoundToEven(n)
			case "floor":
				n = math.Floor(n)
			case "ceil":
				n = math.Ceil(n)
			case "int":
				n = math.Trunc(n)
			case "decimal":
				n -= math.Trunc(n)
			case "inc":
				n = math.Floor(n) + 1
			case "dec":
				n = math.Ceil(n) - 1
			}
			return Number(float32(n)), nil
		}
	}
	if len(args) != 2 {
		return fail()
	}
	a, b := args[0], args[1]
	if a.Kind != b.Kind {
		return fail()
	}
	if (name == "String.EqualTo" && a.Kind == "string") || (name == "Number.EqualTo" && a.Kind == "number") ||
		(name == "Boolean.EqualTo" && a.Kind == "bool") {
		return Boolean(a == b), nil
	}
	if (name == "String.NotEqualTo" && a.Kind == "string") || (name == "Number.NotEqualTo" && a.Kind == "number") ||
		(name == "Boolean.NotEqualTo" && a.Kind == "bool") {
		return Boolean(a != b), nil
	}
	if a.Kind == "string" && name == "String.Add" {
		return String(a.String + b.String), nil
	}
	if a.Kind == "bool" {
		switch name {
		case "Boolean.And":
			return Boolean(a.Bool && b.Bool), nil
		case "Boolean.Or":
			return Boolean(a.Bool || b.Bool), nil
		case "Boolean.Xor":
			return Boolean(a.Bool != b.Bool), nil
		}
		return fail()
	}
	if a.Kind != "number" {
		return fail()
	}
	x, y := a.Number, b.Number
	switch name {
	case "Number.Add":
		return Number(x + y), nil
	case "Number.Minus":
		return Number(x - y), nil
	case "Number.Multiply":
		return Number(x * y), nil
	case "Number.Divide":
		if y == 0 {
			return Value{}, fmt.Errorf("Yarn division by zero")
		}
		return Number(x / y), nil
	case "Number.Modulo":
		if y == 0 {
			return Value{}, fmt.Errorf("Yarn modulo by zero")
		}
		return Number(float32(math.Mod(float64(x), float64(y)))), nil
	case "Number.GreaterThan":
		return Boolean(x > y), nil
	case "Number.GreaterThanOrEqualTo":
		return Boolean(x >= y), nil
	case "Number.LessThan":
		return Boolean(x < y), nil
	case "Number.LessThanOrEqualTo":
		return Boolean(x <= y), nil
	case "min":
		return Number(min(x, y)), nil
	case "max":
		return Number(max(x, y)), nil
	}
	return fail()
}
