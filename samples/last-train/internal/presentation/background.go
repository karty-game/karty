package presentation

import "github.com/karty-game/dialogue"

// ArchitecturalBackground keeps the painted station and train interiors still.
// Actor direction and rain remain animated, while railings and window frames
// keep one fixed sampling position and scale instead of shimmering each update.
func ArchitecturalBackground(authored dialogue.StageTransform) dialogue.StageTransform {
	authored.X, authored.Y = .5, .5
	authored.Height, authored.Rotation = 1.035, 0
	return authored
}
