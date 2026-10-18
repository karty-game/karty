package dialogue

import "fmt"

// EncodeStrings is the build-time counterpart of the KST1 runtime string table.
// Rows must be sorted by unique ID to make compiler outputs deterministic.
func EncodeStrings(lines []Line) ([]byte, error) {
	w := newRecord("KST1", MaxStringBytes)
	w.integer(len(lines), MaxInstructions)
	previous := ""
	for _, line := range lines {
		if line.ID == "" || line.ID <= previous {
			w.invalid = true
		}
		previous = line.ID
		w.str(line.ID, 256)
		w.str(line.Text, 4096)
		w.integer(len(line.Tags), 128)
		for _, tag := range line.Tags {
			w.str(tag, 256)
		}
	}
	return w.finish()
}
func decodeStrings(data []byte, build bool) (map[string]Line, error) {
	r := readRecord(data, "KST1", MaxStringBytes, build)
	n := r.count(MaxInstructions, 12)
	var lines map[string]Line
	if build {
		lines = make(map[string]Line, n)
	}
	previous := ""
	for i := 0; i < n; i++ {
		line := Line{ID: r.str(256), Text: r.str(4096)}
		tags := r.count(128, 4)
		for j := 0; j < tags; j++ {
			tag := r.str(256)
			if build {
				line.Tags = append(line.Tags, tag)
			}
		}
		if build {
			if line.ID == "" || line.ID <= previous {
				return nil, fmt.Errorf("invalid or unsorted Yarn string ID")
			}
			previous = line.ID
			lines[line.ID] = line
		}
	}
	return lines, r.done()
}
func loadStrings(data []byte) (map[string]Line, error) {
	if _, err := decodeStrings(data, false); err != nil {
		return nil, err
	}
	return decodeStrings(data, true)
}
