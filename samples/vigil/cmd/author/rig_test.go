package main

import (
	"bytes"
	"crypto/sha256"
	"image"
	"math"
	"path/filepath"
	"sort"
	"testing"

	"example.com/vigil/internal/dungeon"
)

func TestWholeBodyAtlasRegistrationAndGroundPlane(t *testing.T) {
	frames, description := renderedCharacter(filepath.Join("..", "..", sourceArt))
	for pose, frame := range frames {
		anchor := groundedSprite(frame, description)
		bounds := opaqueCharacterBounds(frame, frame.Bounds())
		// Reprojecting the actual submitted plane must reproduce the rendered
		// model's common root. Sole compensation must not move the hips/torso.
		scale := float64(description.CanvasSize) / description.ProjectedSize
		x := (anchor.OffsetX-anchor.OffsetY)/math.Sqrt2*scale - anchor.OriginX*float64(description.CanvasSize)
		y := -(anchor.OffsetX+anchor.OffsetY)*.5/math.Sqrt2*scale - anchor.OriginY*float64(description.CanvasSize)
		if math.Abs(x+description.RootX) > 1e-7 || math.Abs(y+description.RootY) > 1e-7 {
			t.Fatal("visual root moves", pose, x, y, description)
		}
		if anchor.OffsetZ != 0 || math.Abs(anchor.OriginY*float64(description.CanvasSize)-float64(bounds.Max.Y)) > 1e-7 {
			t.Fatal("sole does not meet floor", pose, anchor, bounds)
		}
	}
	for row := range dungeon.WalkDirections {
		seen := map[[32]byte]bool{}
		upper := map[[32]byte]bool{}
		for phase := range dungeon.WalkFrames {
			frame := frames[row*dungeon.WalkFrames+phase]
			seen[sha256.Sum256(frame.Pix)] = true
			// Protect against the rejected frozen-torso / independent-leg mechanism.
			region := image.Rect(0, 0, description.CanvasSize, int(description.RootY*.65))
			hash := sha256.New()
			for y := region.Min.Y; y < region.Max.Y; y++ {
				hash.Write(frame.Pix[y*frame.Stride : y*frame.Stride+description.CanvasSize*4])
			}
			var sum [32]byte
			copy(sum[:], hash.Sum(nil))
			upper[sum] = true
		}
		if len(seen) != dungeon.WalkFrames || len(upper) < dungeon.WalkFrames/2 {
			t.Fatal("whole-body motion missing", row, len(seen), len(upper))
		}
	}
}

// Browser source comparisons and art review share the packaged textures. Keep
// their alias order and complete pixels tied to the independent source atlases.
func TestRetainedCharacterTexturesMatchRenderedSources(t *testing.T) {
	frames, _ := renderedCharacter(filepath.Join("..", "..", sourceArt))
	for pose, frame := range frames {
		im := load("../../levels/crypt/" + characterTextureName(pose) + ".png")
		retained := crop(im, im.Bounds())
		if retained.Bounds() != frame.Bounds() || !bytes.Equal(retained.Pix, frame.Pix) {
			t.Fatal("runtime texture differs from source atlas", pose, characterTextureName(pose))
		}
	}
}

// A stale first pose during Blender dependency evaluation produced an isolated
// head/shoulder/shield jump. Compare actual painted neighbors around every seam;
// this guards that specific bake defect, not the artistic quality of a gait.
func TestWholeBodyBakeHasNoIsolatedPoseJump(t *testing.T) {
	frames, _ := renderedCharacter(filepath.Join("..", "..", sourceArt))
	for row := range dungeon.WalkDirections {
		differences := make([]float64, dungeon.WalkFrames)
		for pose := range dungeon.WalkFrames {
			a := frames[row*dungeon.WalkFrames+pose]
			b := frames[row*dungeon.WalkFrames+(pose+1)%dungeon.WalkFrames]
			for i := 0; i < len(a.Pix); i += 4 {
				aa, ba := float64(a.Pix[i+3])/255, float64(b.Pix[i+3])/255
				for channel := range 3 {
					differences[pose] += math.Abs(float64(a.Pix[i+channel])*aa - float64(b.Pix[i+channel])*ba)
				}
				differences[pose] += math.Abs(float64(a.Pix[i+3]) - float64(b.Pix[i+3]))
			}
		}
		ordered := append([]float64(nil), differences...)
		sort.Float64s(ordered)
		median := (ordered[len(ordered)/2-1] + ordered[len(ordered)/2]) / 2
		for pose, difference := range differences {
			if difference > median*4 {
				t.Fatal("isolated whole-body pose jump", row, pose, difference, median)
			}
		}
	}
}
