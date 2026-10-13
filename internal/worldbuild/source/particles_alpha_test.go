package source_test

import (
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/worldbuild/source"
)

func TestAlphaParticleSourceVersionAndExplicitFields(t *testing.T) {
	t.Parallel()

	input := strings.Replace(particleSource, "version: 10", "version: 11", 1)
	input = strings.Replace(input, "  version: 1", "  version: 2", 1)
	input = strings.Replace(input, "blend: additive", "blend: alpha", 1)

	for _, test := range []struct {
		name, input string
		ok          bool
	}{
		{"source11 alpha2", input, true},
		{"source11 premultiplied2", strings.Replace(input, "blend: alpha", "blend: premultiplied", 1), true},
		{"source10 premultiplied2", strings.Replace(strings.Replace(input, "version: 11", "version: 10", 1), "blend: alpha", "blend: premultiplied", 1), false},
		{"legacy premultiplied", strings.Replace(strings.Replace(input, "  version: 2", "  version: 1", 1), "blend: alpha", "blend: premultiplied", 1), false},
		{"source10 alpha2", strings.Replace(input, "version: 11", "version: 10", 1), false},
		{"legacy alpha", strings.Replace(input, "  version: 2", "  version: 1", 1), false},
		{"unknown version", strings.Replace(input, "  version: 2", "  version: 3", 1), false},
		{"null blend", strings.Replace(input, "blend: alpha", "blend: null", 1), false},
		{"missing blend", strings.Replace(input, ", blend: alpha", "", 1), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			expanded, err := source.Decode([]byte(test.input))
			if (err == nil) != test.ok {
				t.Fatalf("source gate: %v", err)
			}

			blend := "alpha"
			if strings.Contains(test.input, "blend: premultiplied") {
				blend = "premultiplied"
			}

			if test.ok && (expanded.Particles.Version != 2 || expanded.Particles.Presets[0].Sprite.Blend != blend) {
				t.Fatal("version/blend changed")
			}
		})
	}
}
