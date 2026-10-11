package levelbuild

import (
	"testing"

	"github.com/karty-game/karty-sdk/format/level"
)

func TestAuthoredMetadataYAMLAndTOMLPreserveIntegerPrecision(t *testing.T) {
	t.Parallel()

	for _, fixture := range []struct{ path, source string }{
		{"metadata.yaml", "custom: 9007199254740993\nlarge: 18446744073709551615\nactive: true\nobject:\n  label: hello\n"},
		{"metadata.toml", "custom = 9007199254740993\nactive = true\n[object]\nlabel = 'hello'\n"},
	} {
		t.Run(fixture.path, func(t *testing.T) {
			t.Parallel()

			metadata, err := parseAuthoredMetadata([]byte(fixture.source), fixture.path)
			if err != nil {
				t.Fatal(err)
			}

			data := binaryMetadata(t, metadata)
			decoded, err := level.DecodeMetadata(data)
			custom, _ := decoded.Uint("custom")
			object, _ := decoded.Object("object")

			label, _ := object.String("label")
			if err != nil || custom != 9007199254740993 || label != "hello" {
				t.Fatal("metadata precision/custom object lost", err)
			}
		})
	}

	for _, fixture := range []struct{ path, source string }{
		{"metadata.toml", `{"custom":1}`},
		{"metadata.yaml", "custom: 1\ncustom: 2\n"},
		{"metadata.yaml", "custom: &value [1]\nother: *value\n"},
		{"metadata.yaml", "---\ncustom: 1\n---\ncustom: 2\n"},
		{"metadata.yaml", "[1,2]"},
	} {
		if _, err := parseAuthoredMetadata([]byte(fixture.source), fixture.path); err == nil {
			t.Fatalf("accepted invalid authored metadata: %s", fixture.source)
		}
	}
}
