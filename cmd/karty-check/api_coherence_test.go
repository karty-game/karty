package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/project"
	"github.com/karty-game/karty/internal/release"
)

func TestCurrentAPIConsumers(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..")

	workspace, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer workspace.Close()

	retired := regexp.MustCompile(
		`\b(Vec2D|Vec3D|NewVec2D|NewVec3D|Transform2d|Renderer2d|Sprite2d|WorldActorRef|WorldActorID|LevelRequestID|PhysicsBox|PhysicsBodySettings|PhysicsTarget|SimulationTarget|AudioEntity|TransformTarget|WorldPosition|PathPoint|PathQuery|SimulationWorld|PhysicsRayResult|PhysicsOverlapResult|PhysicsQueryRay|PhysicsQueryOverlap|OnPhysicsRayResult|OnPhysicsOverlapResult|PhysicsRay|NewSprite2DName|UpdateAssetName|SetPhysicsBox|ClearPhysicsBody|MovePhysicsBody|SetPhysicsVelocity|SetPhysicsBodySettings|SetPhysicsGravity|ApplyPhysicsImpulse|ApplyPhysicsRay|ApplyPhysicsExplosion|SetSimulationScale|QueryPhysicsRay|QueryPhysicsOverlap|ApplyAuthoritativeTransform|ApplyTransformUpdate|AttachToCamera|AttachToActor|QueryWorldTag|FindWorldActor|SetSectorFloorZ|SetSectorCeilingZ)\b`,
	)

	for _, directory := range []string{"samples", "testdata", "cmd/karty-check/testdata", "website/content", "packages/controllers"} {
		err := fs.WalkDir(workspace.FS(), directory, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if entry.IsDir() {
				switch entry.Name() {
				case ".karty", "dist", "node_modules", ".git", ".codex", ".agents":
					return filepath.SkipDir
				}

				return nil
			}

			if !strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, ".tmpl") && !strings.HasSuffix(path, ".md") {
				return nil
			}

			contents, err := workspace.ReadFile(path)
			if err != nil {
				return err
			}

			if match := retired.Find(contents); match != nil {
				t.Errorf("%s still teaches or calls retired API %s", path, match)
			}

			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	checkSampleSDKPins(t, root)
}

func checkSampleSDKPins(t *testing.T, root string) {
	t.Helper()

	manifests, err := filepath.Glob(filepath.Join(root, "samples", "*", "karty.toml"))
	if err != nil {
		t.Fatal(err)
	}

	for _, manifest := range manifests {
		config, err := project.Load(filepath.Dir(manifest))
		if err != nil {
			t.Fatal(err)
		}

		expected := release.SDKVersion()

		if config.SDK.Version != expected {
			t.Errorf("%s pins %s, want %s", manifest, config.SDK.Version, expected)
		}
	}
}
