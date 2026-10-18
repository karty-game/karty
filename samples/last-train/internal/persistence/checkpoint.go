package persistence

import (
	"encoding/binary"
	"fmt"
)

// Checkpoint is the sample's envelope, independent of the engine's opaque store.
// Direction contains authored staging; animation phases are intentionally transient.
type Checkpoint struct{ Story, View, Direction []byte }

// Match the engine's per-key limit, including the checkpoint envelope.
const maxCheckpointBytes = 48 * 1024

func (c Checkpoint) Save() ([]byte, error) {
	if len(c.Story) == 0 || len(c.View) == 0 || len(c.Story) > maxCheckpointBytes || len(c.View) > maxCheckpointBytes ||
		len(c.Direction) > maxCheckpointBytes ||
		len(c.Story)+len(c.View)+len(c.Direction) > maxCheckpointBytes-16 {
		return nil, fmt.Errorf("checkpoint exceeds storage bounds")
	}
	data := make([]byte, 16, 16+len(c.Story)+len(c.View)+len(c.Direction))
	copy(data, "LTN2")
	binary.LittleEndian.PutUint32(data[4:], uint32(len(c.Story)))
	binary.LittleEndian.PutUint32(data[8:], uint32(len(c.View)))
	binary.LittleEndian.PutUint32(data[12:], uint32(len(c.Direction)))
	data = append(data, c.Story...)
	data = append(data, c.View...)
	return append(data, c.Direction...), nil
}

func ReadCheckpoint(data []byte) (Checkpoint, error) {
	bad := fmt.Errorf("checkpoint is incomplete or incompatible")
	if len(data) < 12 || len(data) > 64*1024 {
		return Checkpoint{}, bad
	}
	header, directionSize := 12, uint64(0)
	switch string(data[:4]) {
	case "LTN1":
	case "LTN2":
		if len(data) < 16 {
			return Checkpoint{}, bad
		}
		header = 16
		directionSize = uint64(binary.LittleEndian.Uint32(data[12:]))
	default:
		return Checkpoint{}, bad
	}
	storySize, viewSize := uint64(binary.LittleEndian.Uint32(data[4:])), uint64(binary.LittleEndian.Uint32(data[8:]))
	if storySize == 0 || viewSize == 0 || storySize+viewSize+directionSize != uint64(len(data)-header) {
		return Checkpoint{}, bad
	}
	storyEnd, viewEnd := header+int(storySize), header+int(storySize+viewSize)
	return Checkpoint{Story: data[header:storyEnd], View: data[storyEnd:viewEnd], Direction: data[viewEnd:]}, nil
}
