package geometry

import (
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

func compileSolidRenderers(
	document *world.Document,
	expanded source.Expanded,
	materials map[string]uint32,
	used map[uint32]struct{},
) error {
	if len(expanded.SolidPrefabs) == 0 {
		return nil
	}

	payload := &world.SolidRenderers{Version: world.SolidRenderersVersion}

	for _, prefab := range expanded.SolidPrefabs {
		local := world.Document{}
		if err := compileSolids(&local, source.Expanded{Solids: prefab.Solids, UV: expanded.UV}, materials, used); err != nil {
			return err
		}

		payload.Prefabs = append(payload.Prefabs, world.SolidPrefab{ID: prefab.ID, Solids: local.StaticSolids.Items})
	}

	collect := func(contents []source.Content) {
		for _, content := range contents {
			if content.Actor != nil && content.Actor.SolidRenderer != "" {
				payload.Actors = append(payload.Actors, world.SolidAttachment{Actor: content.ID, Prefab: content.Actor.SolidRenderer})
			}
		}
	}
	collect(expanded.Contents)

	for _, room := range expanded.Rooms {
		collect(room.Contents)
	}

	document.SolidRenderers = payload

	return nil
}
