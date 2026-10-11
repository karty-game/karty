package levelbuild

import (
	"fmt"
	"strings"

	"github.com/karty-game/karty-sdk/format/world"
)

type chunkAreaSettings struct {
	Name      string   `toml:"name"`
	Instances []string `toml:"instances"`
	Rooms     []string `toml:"rooms"`
}

// Areas constrain grouping without modifying the full authored graph used by
// mesh compilation, receiver layout, and baking. Unselected sectors share zero.
func resolveChunkAreas(areas []chunkAreaSettings, document *world.Document) ([]int, error) {
	if len(areas) == 0 {
		return nil, nil
	}

	if len(areas) >= world.MaxSectors {
		return nil, fmt.Errorf("too many chunk areas: %w", ErrManifest)
	}

	groups := make([]int, len(document.Sectors))
	names := make(map[string]bool, len(areas))

	for index, area := range areas {
		if area.Name == "" || strings.TrimSpace(area.Name) != area.Name || len(area.Name) > world.MaxIdentifierBytes ||
			names[area.Name] || len(area.Instances)+len(area.Rooms) == 0 {
			return nil, fmt.Errorf("invalid or duplicate chunk area %q: %w", area.Name, ErrManifest)
		}

		names[area.Name] = true

		for _, instance := range area.Instances {
			if err := assignChunkArea(groups, document, areas, index, instance, true); err != nil {
				return nil, err
			}
		}

		for _, room := range area.Rooms {
			if err := assignChunkArea(groups, document, areas, index, room, false); err != nil {
				return nil, err
			}
		}
	}

	return groups, nil
}

func assignChunkArea(groups []int, document *world.Document, areas []chunkAreaSettings, area int, selector string, instance bool) error {
	if selector == "" {
		return fmt.Errorf("chunk area %q has an empty selector: %w", areas[area].Name, ErrManifest)
	}

	matched := false

	for index, sector := range document.Sectors {
		if !chunkAreaMatches(sector, selector, instance) {
			continue
		}

		matched = true

		if groups[index] != 0 && groups[index] != area+1 {
			return fmt.Errorf("sector %q overlaps chunk areas %q and %q: %w",
				sector.ID, areas[groups[index]-1].Name, areas[area].Name, ErrManifest)
		}

		groups[index] = area + 1
	}

	if !matched {
		return fmt.Errorf("chunk area %q selector %q matches no authored room or instance: %w",
			areas[area].Name, selector, ErrManifest)
	}

	return nil
}

func chunkAreaMatches(sector world.Sector, selector string, instance bool) bool {
	if instance {
		return sector.Instance == selector || strings.HasPrefix(sector.Instance, selector+"/")
	}

	room := sector.SourceRoom
	if sector.Instance != "" {
		room = sector.Instance + "/" + room
	}

	return room == selector
}
