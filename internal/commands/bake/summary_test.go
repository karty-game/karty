package bake

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	worldlightmapbake "github.com/karty-game/karty-sdk/bake/worldlightmap"
	"github.com/karty-game/karty/internal/levelbuild"
)

func TestSummaryAddsLevelPairsAndExplainsChunkLighting(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer

	reports := []levelbuild.BakeReport{
		{Name: "levels.first", OutputBytes: 1024, Duration: 100 * time.Millisecond, Stats: worldlightmapbake.Stats{Rays: 2_000_000}},
		{Name: "levels.second", OutputBytes: 2048, Duration: 900 * time.Millisecond, Stats: worldlightmapbake.Stats{Rays: 3_000_000}},
	}
	if err := writeSummary(&output, reports, 2*time.Second); err != nil {
		t.Fatal(err)
	}

	for _, expected := range []string{
		"Bake complete", "Levels: 2 | Chunks: 0", "Wrote 4 files, 3.0 KiB, under .karty/bakes",
		"whole-level lighting", "Builds bake chunk lighting", "Next: karty build\n",
		"Rays: 5000000 | Throughput: 2.50 Mrays/sec",
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
				Duration:    250 * time.Millisecond,
			}
			report.Stats.ReceiverTexels, report.Stats.Rays, report.Stats.Samples, report.Stats.Bounces = 64, 2_500_000, 1, 0
			report.Stats.Duration = 125 * time.Millisecond

			report.Stats.Denoise = "off"
			if err := writeReport(&output, report); err != nil {
				t.Fatal(err)
			}

			for _, expected := range []string{"Baked levels.room", "Receiver texels: 64 | Rays: 2500000 | Throughput: 10.00 Mrays/sec", "Samples: 1 | Bounces: 0", "Denoise: off", "Files: 2, 2.0 KiB", "manifest.bin", "atlas.qoi"} {
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

func TestSummaryThroughputWithoutPositiveElapsedTime(t *testing.T) {
	t.Parallel()

	for _, duration := range []time.Duration{0, -time.Second} {
		var output bytes.Buffer

		report := levelbuild.BakeReport{Duration: duration, Stats: worldlightmapbake.Stats{Rays: 2_000_000}}
		if err := writeReport(&output, report); err != nil {
			t.Fatal(err)
		}

		if err := writeSummary(&output, []levelbuild.BakeReport{report}, duration); err != nil {
			t.Fatal(err)
		}

		if strings.Count(output.String(), "Throughput: 0.00 Mrays/sec") != 2 {
			t.Fatalf("nonpositive elapsed time %s: %s", duration, &output)
		}
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

func TestSummaryReusedBakeDoesNotClaimTracingOrWrites(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer

	report := levelbuild.BakeReport{Name: "levels.cached", Reused: true, Manifest: "manifest.bin", Image: "atlas.qoi"}
	if err := writeReport(&output, report); err != nil {
		t.Fatal(err)
	}

	if err := writeSummary(&output, []levelbuild.BakeReport{report}, time.Second); err != nil {
		t.Fatal(err)
	}

	for _, expected := range []string{"Reused levels.cached", "No rays traced or files written", "Wrote 0 files", "manifest.bin", "atlas.qoi"} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("reused report lacks %q: %s", expected, &output)
		}
	}

	if err := writeReport(failedSummaryWriter{}, report); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("reused output error = %v", err)
	}
}

func (failedSummaryWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}
