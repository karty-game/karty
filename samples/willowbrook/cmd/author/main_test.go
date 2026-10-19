package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReusableTilesAndGroundedAnimation(t *testing.T) {
	root := filepath.Join("..", "..", "levels", "village")
	for i := range 12 {
		f, err := os.Open(filepath.Join(root, fmt.Sprintf("hero-%02d.png", i)))
		if err != nil {
			t.Fatal(err)
		}
		im, err := png.Decode(f)
		_ = f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if im.Bounds().Dx() != 32 || im.Bounds().Dy() != 48 || alphaBounds(im).Max.Y != 48 {
			t.Fatalf("pose %d lost its common size/sole", i)
		}
		body := crop(im, im.Bounds())
		keepLargestComponent(body)
		if alphaBounds(body).Max.Y != 48 {
			t.Fatalf("pose %d uses detached shadow pixels instead of the character's feet for its sole", i)
		}
	}
	files, err := filepath.Glob(filepath.Join(root, "*.png"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			t.Fatal(err)
		}
		config, err := png.DecodeConfig(f)
		_ = f.Close()
		if err != nil {
			t.Fatal(err)
		}
		limit := 512
		if config.Width > limit || config.Height > limit {
			t.Fatalf("oversized scene texture: %s %dx%d", file, config.Width, config.Height)
		}
	}
}

func TestApprovedDualGridAtlasCornerStates(t *testing.T) {
	for mask := range 16 {
		path := filepath.Join("..", "..", "levels", "village", fmt.Sprintf("dual-%02d.png", mask))
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		im, err := png.Decode(f)
		_ = f.Close()
		if err != nil {
			t.Fatal(err)
		}
		for corner := range 4 {
			x, y := 12, 12
			if corner%2 != 0 {
				x = 115
			}
			if corner/2 != 0 {
				y = 115
			}
			if atlasDirt(im, x, y) != (mask&(1<<corner) != 0) {
				t.Fatalf("approved atlas mask %d lost corner %d", mask, corner)
			}
		}
	}
}

func atlasDirt(im image.Image, x, y int) bool {
	var red, green uint64
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			r, g, _, _ := im.At(x+dx, y+dy).RGBA()
			red += uint64(r)
			green += uint64(g)
		}
	}
	return red > green
}

func TestTiledLevelHasOnlyReusableImages(t *testing.T) {
	root := filepath.Join("..", "..")
	data, err := os.ReadFile(filepath.Join(root, "levels/village/level.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "[world]") || !strings.Contains(string(data), `kind = "tiled"`) {
		t.Fatal("2D map still depends on 3D world geometry")
	}
	if strings.Contains(string(data), `name = "static-`) {
		t.Fatal("build-generated chunk images returned to the authored level catalog")
	}
	chunks, err := filepath.Glob(filepath.Join(root, "levels/village/static-*.png"))
	if err != nil || len(chunks) != 0 {
		t.Fatal("build-generated chunk images are present in authored source", chunks, err)
	}
	if _, err := os.Stat(filepath.Join(root, "internal/village/static-chunks.bin")); !os.IsNotExist(err) {
		t.Fatal("compiled chunk placements are present in authored source", err)
	}
	for _, name := range []string{"cottage", "tree", "cliff", "fence", "clue-note", "treasure-closed", "treasure-open"} {
		f, err := os.Open(filepath.Join(root, "levels/village", name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		im, err := png.Decode(f)
		_ = f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if im.Bounds().Dx() > 256 || im.Bounds().Dy() > 256 {
			t.Fatalf("scene-sized texture %s", name)
		}
	}
}
