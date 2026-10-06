package actionbuild

import (
	"errors"
	"fmt"
)

const (
	maxSourceFiles  = 512
	maxDeclarations = 256
	maxSteps        = 1024
	maxStringBytes  = 1024
	maxJSONDepth    = 64
	maxSchemaDepth  = 80
)

var ErrInvalidActions = errors.New("invalid authored actions")

func invalidActions(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidActions, fmt.Sprintf(format, args...))
}
