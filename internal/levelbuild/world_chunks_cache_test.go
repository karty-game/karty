package levelbuild

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

//nolint:gocognit,gocyclo,maintidx,gosec // One cache lifecycle uses only isolated fixture paths.
func TestDevelopmentBuildsGeometryAndUsesOnlyMatchingPrebuiltLighting(t *testing.T) {
	t.Parallel()

	for _, lighting := range []bool{false, true} {
		name := "mesh"
		if lighting {
			name = "lightmapped"
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := lightingLevelFixture(t, "version: 4\n"+chunkRoomsYAML+directLightmapYAML)
			selected := directLightmapSDK(t)
			selected.Assets.Capabilities.Runtime = append(selected.Assets.Capabilities.Runtime, asset.Capability(worldchunk.Feature))

			if lighting {
				enableFixtureLightmap(
					t,
					root,
					"enabled = true\noffline = true\nlights = ['red','blue']\npage_size = 512\ndensity = 1\nbake_samples = 1\nbake_bounces = 0\nbake_denoise = 'off'\n",
				)
			}

			enableFixtureChunks(t, root, "max_sectors = 1\n")

			expectedChunks := 2
			dev := func(wantBake bool) Artifact {
				t.Helper()

				artifacts, err := BuildAllWithOptions(t.Context(), root, 4, "", selected, Options{Development: true})
				if err != nil || len(artifacts) != 1 {
					t.Fatalf("dev build: %v", err)
				}

				artifact := artifacts[0]
				if !slices.Contains(artifact.Features, worldchunk.Feature) || len(artifact.Chunks) != expectedChunks {
					t.Fatalf("dev lost geometry chunks: %d", len(artifact.Chunks))
				}

				envelope := executeLightmapEnvelope(t, artifact.Bytes)
				worldBytes, _, found := envelope.Read(world.EntryName, 0, world.MaxEncodedSize)

				document, err := world.Decode(worldBytes)
				if !found || err != nil || len(document.Sectors) != 2 || len(document.Contents) != 1 || len(artifact.AuthoredActors) != 1 {
					t.Fatalf("dev lost full topology or actors: %v", err)
				}

				_, _, hasCatalog := envelope.Read(worldchunk.CatalogEntryName, 0, worldchunk.MaxEncodedSize)
				if !hasCatalog {
					t.Fatal("dev transport does not match its feature declaration")
				}

				if slices.Contains(artifact.Features, worldlightmap.PrebakeFeature) != wantBake {
					t.Fatalf("dev prebake feature does not match expected availability: %v", wantBake)
				}

				for _, chunk := range artifact.Chunks {
					child := executeLightmapEnvelope(t, chunk.Bytes)

					_, _, prebaked := child.Read(worldlightmap.PrebakeEntryName, 0, worldlightmap.MaxPrebakeManifestSize)
					if prebaked != wantBake {
						t.Fatalf("dev prebake=%v want=%v", prebaked, wantBake)
					}
				}

				return artifact
			}
			dev(false)

			cachePath := filepath.Join(root, automaticBakeDirectory, "lighting")

			previewFiles, err := os.ReadDir(cachePath)
			if err != nil || len(previewFiles) != 3 {
				t.Fatalf("cold dev did not cache complete mesh-only chunks: %v", err)
			}

			built, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
			if err != nil || len(built) != 1 || len(built[0].Chunks) != 2 {
				t.Fatalf("prebuild: %v", err)
			}

			warm := dev(lighting)
			if !bytes.Equal(warm.Bytes, built[0].Bytes) || !bytes.Equal(warm.Chunks[0].Bytes, built[0].Chunks[0].Bytes) ||
				!bytes.Equal(warm.Chunks[1].Bytes, built[0].Chunks[1].Bytes) {
				t.Fatal("dev did not reuse the exact prebuilt artifacts")
			}

			files, err := os.ReadDir(cachePath)
			if err != nil {
				t.Fatal(err)
			}

			for _, file := range files {
				if file.Name() == chunkPixelCacheDirectory && file.IsDir() {
					continue // Optional pixel cache is unnecessary for complete artifact reuse.
				}

				if !lighting || slices.ContainsFunc(previewFiles, func(preview os.DirEntry) bool { return preview.Name() == file.Name() }) {
					continue
				}

				path := filepath.Join(cachePath, file.Name())

				original, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}

				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}

				dev(false)

				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("dev regenerated a missing cache file")
				}

				if err := os.WriteFile(path, []byte("corrupt"), 0o600); err != nil {
					t.Fatal(err)
				}

				dev(false)

				if after, err := os.ReadFile(path); err != nil || string(after) != "corrupt" {
					t.Fatal("dev repaired a corrupt cache instead of falling back")
				}

				if err := os.WriteFile(
					path,
					original,
					0o600,
				); err != nil {
					t.Fatal(err)
				}
			}

			dev(lighting)

			levelPath := filepath.Join(root, "levels", "lighting")
			for _, edit := range []struct {
				file, from, to string
				count          int
			}{
				{"world.yaml", "ceiling: {c: 4}", "ceiling: {c: 3.8}", 2},
				{"world.yaml", "radius: 6", "radius: 5", 2},
				{"level.toml", "max_sectors = 1", "max_sectors = 2", 1},
			} {
				path := filepath.Join(levelPath, edit.file)

				original, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}

				changed := strings.Replace(string(original), edit.from, edit.to, 1)
				if changed == string(original) {
					t.Fatal("fixture edit did not match")
				}

				if err := os.WriteFile(
					path,
					[]byte(changed),
					0o600,
				); err != nil {
					t.Fatal(err)
				}

				expectedChunks = edit.count

				dev(false)

				if err := os.WriteFile(
					path,
					original,
					0o600,
				); err != nil {
					t.Fatal(err)
				}

				expectedChunks = 2
			}

			dev(lighting)
		})
	}
}

func TestDevelopmentChunkCacheRebuildsEscapingChildSymlink(t *testing.T) {
	t.Parallel()
	root := lightingLevelFixture(t, "version: 4\n"+chunkRoomsYAML)
	selected := directLightmapSDK(t)
	selected.Assets.Capabilities.Runtime = append(selected.Assets.Capabilities.Runtime, asset.Capability(worldchunk.Feature))

	enableFixtureChunks(t, root, "max_sectors = 1\n")

	built, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
	if err != nil {
		t.Fatal(err)
	}

	child := built[0].Chunks[0]

	outside := filepath.Join(t.TempDir(), "child.kld")
	if err := os.WriteFile(outside, child.Bytes, 0o600); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(root, automaticBakeDirectory, "lighting", child.ContentSHA256+".chunk.kld")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(outside, path); err != nil {
		t.Skip("symlinks unavailable:", err)
	}

	artifacts, err := BuildAllWithOptions(t.Context(), root, 4, "", selected, Options{Development: true})
	if err != nil || len(artifacts) != 1 || len(artifacts[0].Chunks) != 2 {
		t.Fatalf("dev did not regenerate geometry after rejecting the escaping cached child: %v", err)
	}

	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatal("escaping symlink was not replaced by a confined generated child")
	}
}
