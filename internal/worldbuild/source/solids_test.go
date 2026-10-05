package source_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

func TestSourceExtrasPresenceVersionAndSyntax(t *testing.T) {
	t.Parallel()

	for _, field := range []string{"solids", "contents"} {
		for _, value := range []string{"[]", "null"} {
			_, err := source.Decode([]byte("version: 5\n" + uvRoomYAML + field + ": " + value + "\n"))
			if !errors.Is(err, worldsource.ErrVersion) {
				t.Fatalf("old version accepted %s %s: %v", field, value, err)
			}
		}

		_, err := source.Decode([]byte("version: 6\n" + uvRoomYAML + field + ": null\n"))
		if err == nil {
			t.Fatal("explicit null accepted")
		}
	}

	text := "version: 6\n" + uvRoomYAML + "prefabs:\n  - id: empty\n    solids: []\n"
	if _, err := source.Decode([]byte(text)); err == nil {
		t.Fatal("empty detail prefab accepted")
	}

	if _, err := source.Decode([]byte(strings.Replace(text, "solids: []", "solids: [{id: bad}]", 1))); err == nil {
		t.Fatal("incomplete solid accepted")
	}
}
