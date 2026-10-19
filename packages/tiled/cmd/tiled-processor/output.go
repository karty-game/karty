package main

import (
	"bytes"
	"errors"
	"fmt"
)

var errOutputLimit = errors.New("compiled Tiled output exceeds bounds")

type boundedOutput struct {
	bytes.Buffer
	limit int
}

func (output *boundedOutput) Write(data []byte) (int, error) {
	if len(data) > output.limit-output.Len() {
		return 0, errOutputLimit
	}
	return output.Buffer.Write(data)
}
func outputBytes(files memoryFS) int {
	total := 0
	for _, data := range files {
		total += len(data)
	}
	return total
}
func setOutput(files memoryFS, name string, data []byte) error {
	previous, exists := files[name]
	if exists && name != "assets.json" {
		return fmt.Errorf("%w: duplicate output %s", errOutputLimit, name)
	}
	if (!exists && len(files) >= maxFiles) || len(data) > maxBytes-outputBytes(files)+len(previous) {
		return errOutputLimit
	}
	files[name] = data
	return nil
}
