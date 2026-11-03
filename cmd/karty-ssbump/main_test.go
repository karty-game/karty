package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestCommandProducesLinearPNGAndMatchingReport(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	input, output := filepath.Join(directory, "height.png"), filepath.Join(directory, "ssbump.png")
	pixels := image.NewGray(image.Rect(0, 0, 8, 8))

	for y := range 8 {
		for x := range 8 {
			pixels.SetGray(x, y, color.Gray{Y: 128})
		}
	}

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, pixels); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(input, encoded.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if err := run(t.Context(), []string{"--height", input, "--output", output}, &stdout, &stderr); err != nil {
		t.Fatalf("command failed: %v; stderr: %s", err, &stderr)
	}

	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}

	r, g, b, a := decoded.At(4, 4).RGBA()
	if r != 85*257 || g != r || b != r || a != 65535 {
		t.Fatalf("flat PNG contains wrong linear coefficients: %d,%d,%d,%d", r, g, b, a)
	}

	var report struct {
		Algorithm string `json:"algorithm"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
		SHA256    string `json:"sha256"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256(data)
	if report.Algorithm == "" || report.Width != 8 || report.Height != 8 || report.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("report does not identify output bytes: %+v", report)
	}

	// Cancellation and invalid controls must preserve already published output.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := run(ctx, []string{"--height", input, "--output", output}, &stdout, &stderr); err == nil {
		t.Fatal("canceled invocation succeeded")
	}

	if err := run(t.Context(), []string{"--height", input, "--output", output, "--depth", "NaN"}, &stdout, &stderr); err == nil {
		t.Fatal("non-finite depth accepted")
	}

	unchanged, err := os.ReadFile(output)
	if err != nil || !bytes.Equal(data, unchanged) {
		t.Fatalf("failed invocation changed output: %v", err)
	}

	if err := run(t.Context(), []string{"--height", input, "--output", input}, &stdout, &stderr); err == nil {
		t.Fatal("command allowed replacing height input")
	}
}
