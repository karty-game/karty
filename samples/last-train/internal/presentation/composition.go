package presentation

import "fmt"

// Composition is authored direction owned by a run, independent of animation
// phases and the reader's lifetime motion preference.
type Composition struct {
	Frame             string
	Ensemble          bool
	Mara, Elias, Iris string
}

func DefaultComposition() Composition {
	return Composition{Frame: "wide", Mara: "neutral", Elias: "neutral", Iris: "neutral"}
}

var frames = [...]string{"wide", "medium", "close"}
var expressions = [3][4]string{
	{"neutral", "worried", "relieved", "resolute"},
	{"neutral", "wistful", "hopeful", "reading"},
	{"neutral", "guarded", "relieved", "offering"},
}

func expressionIndex(name, expression string) (byte, bool) {
	character := -1
	switch name {
	case "mara":
		character = 0
	case "elias":
		character = 1
	case "iris":
		character = 2
	}
	if character >= 0 {
		for i, value := range expressions[character] {
			if value == expression {
				return byte(i), true
			}
		}
	}
	return 0, false
}
func ValidExpression(name, expression string) bool {
	_, ok := expressionIndex(name, expression)
	return ok
}
func (c *Composition) SetExpression(name, expression string) error {
	if !ValidExpression(name, expression) {
		return fmt.Errorf("unknown expression %s %s", name, expression)
	}
	switch name {
	case "mara":
		c.Mara = expression
	case "elias":
		c.Elias = expression
	case "iris":
		c.Iris = expression
	}
	return nil
}

// Save writes the fixed nine-byte LTC1 composition record.
func (c Composition) Save() ([]byte, error) {
	frame := byte(255)
	for i, v := range frames {
		if c.Frame == v {
			frame = byte(i)
		}
	}
	mara, a := expressionIndex("mara", c.Mara)
	elias, b := expressionIndex("elias", c.Elias)
	iris, d := expressionIndex("iris", c.Iris)
	if frame == 255 || !a || !b || !d {
		return nil, fmt.Errorf("invalid composition")
	}
	flags := byte(0)
	if c.Ensemble {
		flags = 1
	}
	return []byte{'L', 'T', 'C', '1', frame, flags, mara, elias, iris}, nil
}
func ReadComposition(data []byte) (Composition, error) {
	if len(data) == 0 {
		return DefaultComposition(), nil
	}
	if len(data) != 9 || string(data[:4]) != "LTC1" || data[4] >= 3 || data[5] > 1 || data[6] >= 4 || data[7] >= 4 || data[8] >= 4 {
		return Composition{}, fmt.Errorf("incompatible or invalid composition record")
	}
	return Composition{
		Frame:    frames[data[4]],
		Ensemble: data[5] == 1,
		Mara:     expressions[0][data[6]],
		Elias:    expressions[1][data[7]],
		Iris:     expressions[2][data[8]],
	}, nil
}
