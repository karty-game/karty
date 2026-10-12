package bake

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/karty-game/karty/internal/levelbuild"
)

func TestSummaryAddsLevelPairsAndExplainsChunkLighting(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer

	reports := []levelbuild.BakeReport{{Name: "levels.first", OutputBytes: 1024}, {Name: "levels.second", OutputBytes: 2048}}
	if err := writeSummary(&output, reports, time.Second); err != nil {
		t.Fatal(err)
	}

	for _, expected := range []string{
		"Bake complete", "Levels: 2 | Chunks: 0", "Wrote 4 files, 3.0 KiB, under .karty/bakes",
		"whole-level lighting", "Builds bake chunk lighting", "Next: karty build\n",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("summary missing %q:\n%s", expected, &output)
		}
	}
}

func TestSummaryReportKeepsPathsAndPackagingAdvice(t *testing.T) {
	t.Parallel()

	for _, automatic := range []bool{false, true} {
		t.Run(map[bool]string{false: "manual", true: "automatic"}[automatic], func(t *testing.T) {
			t.Parallel()

			var output bytes.Buffer

			report := levelbuild.BakeReport{
				Name:        "levels.room",
				Manifest:    "manifest.bin",
				Image:       "atlas.qoi",
				Automatic:   automatic,
				OutputBytes: 2048,
			}
			report.Stats.ReceiverTexels, report.Stats.Rays, report.Stats.Samples, report.Stats.Bounces = 64, 100, 1, 0

			report.Stats.Denoise = "off"
			if err := writeReport(&output, report); err != nil {
				t.Fatal(err)
			}

			for _, expected := range []string{"Baked levels.room", "Receiver texels: 64 | Rays: 100", "Samples: 1 | Bounces: 0", "Denoise: off", "Files: 2, 2.0 KiB", "manifest.bin", "atlas.qoi"} {
				if !strings.Contains(output.String(), expected) {
					t.Errorf("level report missing %q:\n%s", expected, &output)
				}
			}

			if strings.Contains(output.String(), "offline = true") == automatic {
				t.Fatalf("packaging advice automatic=%v:\n%s", automatic, &output)
			}
		})
	}
}

func TestSummaryReturnsOutputErrors(t *testing.T) {
	t.Parallel()

	if err := writeSummary(failedSummaryWriter{}, nil, 0); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("aggregate output error = %v", err)
	}

	if err := writeReport(failedSummaryWriter{}, levelbuild.BakeReport{}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("level output error = %v", err)
	}
}

type failedSummaryWriter struct{}

func (failedSummaryWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}
