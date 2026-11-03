package levelbuild

import (
	"testing"

	"github.com/karty-game/karty-sdk/format/worldmaterialssbump"
)

func TestWorldSSBumpGeneratedEntryNamesAreReserved(t *testing.T) {
	t.Parallel()

	for _, name := range []string{worldmaterialssbump.DescriptorEntry, worldmaterialssbump.AtlasEntry, worldmaterialssbump.MipTailEntry} {
		if _, err := loadData(t.TempDir(), []dataEntry{{Name: name, Source: "authored.bin"}}); err == nil {
			t.Fatal("authored SS entry accepted", name)
		}
	}
}
