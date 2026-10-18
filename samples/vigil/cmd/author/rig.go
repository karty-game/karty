package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"math"
	"os"

	"example.com/vigil/internal/dungeon"
)

// Whole-body frames come from a single skinned humanoid and animation. No
// costume pixels or limb silhouettes are independently warped at runtime.
type spritePose struct{ OffsetX, OffsetY, OffsetZ, OriginX, OriginY float64 }

var (
	rigCanvas        = 192
	rigRootX         = 96.0
	rigRootY         = 160.0
	rigProjectedSize = 2.5
	activeRig        [dungeon.WalkTextureCount + dungeon.WalkDirections]spritePose
)

type renderDescription struct {
	CanvasSize    int     `json:"canvasSize"`
	RootX         float64 `json:"rootX"`
	RootY         float64 `json:"rootY"`
	ProjectedSize float64 `json:"projectedSize"`
	Frames        int     `json:"frames"`
	Directions    int     `json:"directions"`
	Stride        float64 `json:"stride"`
}

func renderedCharacter(directory string) ([]*image.NRGBA, renderDescription) {
	var description renderDescription
	data, err := os.ReadFile(directory + "/pilgrim-render.json")
	must(err)
	must(json.Unmarshal(data, &description))
	if description.CanvasSize < 64 || description.CanvasSize > 512 || description.ProjectedSize <= 0 || description.RootX <= 0 ||
		description.RootX >= float64(description.CanvasSize) ||
		description.RootY <= 0 ||
		description.RootY >= float64(description.CanvasSize) ||
		description.Frames != dungeon.WalkFrames ||
		description.Directions != dungeon.WalkDirections ||
		math.Abs(description.Stride-dungeon.WalkStride) > 1e-6 {
		panic("invalid whole-body animation description")
	}
	canvas := description.CanvasSize
	frames := make([]*image.NRGBA, len(activeRig))
	for _, sheet := range []struct {
		name                 string
		columns, rows, first int
	}{
		{"pilgrim-walk", dungeon.WalkFrames, dungeon.WalkDirections, 0},
		{"pilgrim-idle", dungeon.WalkDirections, 1, dungeon.WalkTextureCount},
	} {
		im := load(directory + "/" + sheet.name + ".png")
		if im.Bounds().Dx() != canvas*sheet.columns || im.Bounds().Dy() != canvas*sheet.rows {
			panic("whole-body atlas dimensions disagree with render metadata: " + sheet.name)
		}
		for row := range sheet.rows {
			for column := range sheet.columns {
				frame := image.NewNRGBA(image.Rect(0, 0, canvas, canvas))
				draw.Draw(frame, frame.Bounds(), im, image.Pt(column*canvas, row*canvas), draw.Src)
				frames[sheet.first+row*sheet.columns+column] = frame
			}
		}
	}
	return frames, description
}

// Anchor the upright plane at its nearest painted sole. Shifting its ground
// depth and bitmap origin together preserves the original root projection,
// while keeping all opaque pixels above the real floor. The physical root is
// the separate fixed ground shadow, so animation cannot move collision/sight.
func groundedSprite(im image.Image, description renderDescription) spritePose {
	bounds := opaqueCharacterBounds(im, im.Bounds())
	if bounds.Empty() || bounds.Min.X < 1 || bounds.Max.X >= description.CanvasSize || bounds.Min.Y < 1 ||
		bounds.Max.Y >= description.CanvasSize {
		panic(fmt.Sprintf("empty or clipped whole-body frame: %v", bounds))
	}
	originY := float64(bounds.Max.Y) / float64(description.CanvasSize)
	vertical := (originY - description.RootY/float64(description.CanvasSize)) * description.ProjectedSize
	forward := -2 * vertical
	return spritePose{
		OffsetX: forward / math.Sqrt2,
		OffsetY: forward / math.Sqrt2,
		OriginX: description.RootX / float64(description.CanvasSize),
		OriginY: originY,
	}
}
func prepareRenderedCharacter() {
	frames, description := renderedCharacter(sourceArt)
	rigCanvas, rigRootX, rigRootY, rigProjectedSize = description.CanvasSize, description.RootX, description.RootY, description.ProjectedSize
	for pose, frame := range frames {
		save(characterTextureName(pose), frame)
		activeRig[pose] = groundedSprite(frame, description)
	}
}

// Compact names keep the level catalog within its metadata bound. Authoring,
// source comparisons and art review share these same complete-body PNGs.
func characterTextureName(pose int) string {
	const digits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

	return string([]byte{byte('A' + pose/len(digits)), digits[pose%len(digits)]})
}
