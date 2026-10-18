package packages

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty-ui/codegen"
	"github.com/karty-game/karty/internal/project"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

func uiFixture(t *testing.T) (string, string) {
	t.Helper()
	_, game := fixture(t)

	kit := filepath.Join(t.TempDir(), "arbitrary-folder-name")
	if err := os.CopyFS(kit, os.DirFS("../../testdata/package-ui/library")); err != nil {
		t.Fatal(err)
	}

	write(t, filepath.Join(kit, "assets/body.ttf"), goregular.TTF)
	name := filepath.Join(kit, ManifestName)
	write(t, name, append(read(t, name), []byte("\n[[ui.fonts]]\nrole = 'body'\nsource = 'assets/body.ttf'\n")...))

	return kit, game
}

func TestUIPackageDefaultsConflictUnlessConsumerSelectsAnOverride(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"theme", "font"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			kit, game := uiFixture(t)

			other := filepath.Join(t.TempDir(), "second")
			if err := os.CopyFS(other, os.DirFS(kit)); err != nil {
				t.Fatal(err)
			}

			write(t, filepath.Join(other, ManifestName), []byte(`format_version=1
name='other-kit'
version='0.1.0'
sdk='0.0.13'
[ui]
theme='ui/theme.toml'
[[ui.images]]
name='panel'
source='assets/panel.png'
profile='interface'
[[ui.fonts]]
role='body'
source='assets/body.ttf'
`))

			if mode == "font" {
				write(t, filepath.Join(other, "assets/body.ttf"), gobold.TTF)
			}

			for _, input := range []string{kit, other} {
				if _, err := Import(t.Context(), game, input, false); err != nil {
					t.Fatal(err)
				}
			}

			set := restore(t, game)
			if _, err := ResolveUI(
				t.Context(),
				game,
				project.Assets{},
				set,
			); err == nil ||
				!strings.Contains(err.Error(), "conflicting package "+mode) {
				t.Fatalf("conflicting %s defaults accepted: %v", mode, err)
			}

			write(
				t,
				filepath.Join(game, "theme.toml"),
				[]byte("extends='dark'\n[images.hud-panel]\nasset='hud-kit.panel'\nslice=[4,4,4,4]\n"),
			)
			write(t, filepath.Join(game, "my-font.ttf"), goregular.TTF)

			assets := project.Assets{
				Theme: project.Theme{Source: "theme.toml"},
				Fonts: []project.Font{{Role: "body", Source: "my-font.ttf"}},
			}

			resolved, err := ResolveUI(t.Context(), game, assets, set)
			if err != nil {
				t.Fatal("consumer override did not resolve defaults:", err)
			}

			if _, err := project.CompileUI(game, resolved.UI, resolved.Layouts, resolved.Theme.Source); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestUIPackagePresentationEditKeepsGeneratedGoIdentity(t *testing.T) {
	t.Parallel()

	kit, game := uiFixture(t)
	if _, err := Import(t.Context(), game, kit, false); err != nil {
		t.Fatal(err)
	}

	set := restore(t, game)

	assets, err := ResolveUI(t.Context(), game, project.Assets{}, set)
	if err != nil {
		t.Fatal(err)
	}

	views, err := project.CompileUI(game, assets.UI, assets.Layouts, assets.Theme.Source)
	if err != nil {
		t.Fatal(err)
	}

	before, err := codegen.UIPackageFiles(views, "example.com/game")
	if err != nil {
		t.Fatal(err)
	}

	name := filepath.Join(kit, "ui/hud-badge.kui")
	write(t, name, bytes.ReplaceAll(read(t, name), []byte(">Change</button>"), []byte(">Apply</button>")))

	changed, err := ResolveUI(t.Context(), game, project.Assets{}, set)
	if err != nil {
		t.Fatal(err)
	}

	views, err = project.CompileUI(game, changed.UI, changed.Layouts, changed.Theme.Source)
	if err != nil {
		t.Fatal(err)
	}

	after, err := codegen.UIPackageFiles(views, "example.com/game")
	if err != nil {
		t.Fatal(err)
	}

	if assets.UI[0].Source != changed.UI[0].Source || !bytes.Equal(before[assets.UI[0].Source], after[changed.UI[0].Source]) {
		t.Fatal("presentation edit changed generated Go identity")
	}
}

func TestUIPackageFolderAndArchiveCompileWithConsumerComponents(t *testing.T) {
	t.Parallel()
	kit, game := uiFixture(t)
	module := read(t, filepath.Join(game, "go.mod"))

	definition, err := Import(t.Context(), game, kit, false)
	if err != nil || definition.GoModule != "" {
		t.Fatalf("UI-only import: %+v: %v", definition, err)
	}

	if !bytes.Equal(module, read(t, filepath.Join(game, "go.mod"))) {
		t.Fatal("UI-only import rewrote Go dependencies")
	}

	write(
		t,
		filepath.Join(game, "ui/game-screen.kui"),
		[]byte(
			`<template><panel modal="true"><HudBadge props={ HudBadgeProps{Title: title, Value: value, Change: change} }/></panel></template>
<script setup lang="go">func setup(title string, value func() string, change func()) {}</script>`,
		),
	)
	write(t, filepath.Join(game, "ui/theme.toml"), []byte("extends = 'dark'\n[colors]\nprimary = '#cc3300ff'\n"))
	before := read(t, filepath.Join(game, "ui/theme.toml"))
	assets := project.Assets{
		UI:    []project.Texture{{Name: "screen", Source: "ui/game-screen.kui"}},
		Theme: project.Theme{Source: "ui/theme.toml"},
	}

	for _, mode := range []string{"folder", "archive"} {
		if mode == "archive" {
			archive := filepath.Join(t.TempDir(), "arbitrary-archive-name.kpkg")
			export(t, kit, archive)

			if _, err := Import(t.Context(), game, archive, true); err != nil {
				t.Fatal(err)
			}

			if err := os.RemoveAll(kit); err != nil {
				t.Fatal(err)
			}

			if err := os.Remove(archive); err != nil {
				t.Fatal(err)
			}

			if err := os.RemoveAll(filepath.Join(game, ".karty")); err != nil {
				t.Fatal(err)
			}
		}

		checkConsumerUI(t, game, assets, before)
	}
}

func checkConsumerUI(t *testing.T, game string, assets project.Assets, before []byte) {
	t.Helper()
	set := restore(t, game)

	resolved, err := ResolveUI(t.Context(), game, assets, set)
	if err != nil {
		t.Fatalf("%s: %v", "resolved", err)
	}

	views, err := project.CompileUI(game, resolved.UI, resolved.Layouts, resolved.Theme.Source)
	if err != nil || len(views) != 3 {
		t.Fatalf("%s components: %+v: %v", "resolved", views, err)
	}

	if views[1].Name != "HudBadge" || views[1].Asset != "hud-kit.badge" || len(resolved.Fonts) != 1 || len(resolved.Textures) != 1 {
		t.Fatalf("missing package exports: %+v / %+v", views, resolved)
	}

	contents := read(t, filepath.Join(game, resolved.Theme.Source))
	if !bytes.Contains(contents, []byte("#cc3300ff")) || !bytes.Contains(contents, []byte("hud-kit.panel")) ||
		!bytes.Contains(contents, []byte("30")) {
		t.Fatalf("incorrect theme precedence: %s", contents)
	}

	again, err := ResolveUI(t.Context(), game, assets, set)
	if err != nil || again.UI[1].Source != resolved.UI[1].Source || again.Theme.Source != resolved.Theme.Source {
		t.Fatalf("unstable UI inputs: %v", err)
	}

	if !bytes.Equal(before, read(t, filepath.Join(game, "ui/theme.toml"))) {
		t.Fatal("rewrote authored theme")
	}
}

func TestUIPackageInvalidExportsAreRejectedBeforeImport(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"missing component", "invalid unused component", "escape", "component limit", "oversized component", "unknown image", "invalid font role", "duplicate asset"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			kit, game := uiFixture(t)
			manifest := filepath.Join(kit, ManifestName)

			switch mode {
			case "missing component":
				if err := os.Remove(filepath.Join(kit, "ui/hud-badge.kui")); err != nil {
					t.Fatal(err)
				}
			case "invalid unused component":
				write(t, filepath.Join(kit, "ui/hud-unused.kui"), []byte("invalid UI"))
			case "escape":
				if err := os.Remove(filepath.Join(kit, "ui/hud-badge.kui")); err != nil {
					t.Fatal(err)
				}

				if err := os.Symlink(filepath.Join(game, "go.mod"), filepath.Join(kit, "ui/hud-badge.kui")); err != nil {
					t.Fatal(err)
				}
			case "component limit":
				data := read(t, manifest)
				for range MaxUIComponents {
					data = append(data, []byte("\n[[ui.components]]\nname='extra'\nsource='ui/hud-unused.kui'\n")...)
				}

				write(t, manifest, data)
			case "oversized component":
				write(t, filepath.Join(kit, "ui/hud-badge.kui"), []byte(strings.Repeat("x", 65537)))
			case "unknown image":
				write(t, filepath.Join(kit, "ui/theme.toml"), []byte("extends='dark'\n[images.panel]\nasset='missing'\nslice=[0,0,0,0]\n"))
			case "invalid font role":
				write(t, manifest, bytes.ReplaceAll(read(t, manifest), []byte("role = 'body'"), []byte("role = 'anything'")))
			case "duplicate asset":
				write(t, manifest, append(read(t, manifest), []byte("\n[[ui.components]]\nname='badge'\nsource='ui/hud-unused.kui'\n")...))
			}

			before := read(t, filepath.Join(game, "karty.toml"))
			if _, err := Import(t.Context(), game, kit, false); err == nil {
				t.Fatal("invalid UI package imported")
			}

			if !bytes.Equal(before, read(t, filepath.Join(game, "karty.toml"))) {
				t.Fatal("invalid package changed dependency manifest")
			}
		})
	}
}

func TestUIPackageCollisionsOverridesAndCancellation(t *testing.T) {
	t.Parallel()

	kit, game := uiFixture(t)
	if _, err := Import(t.Context(), game, kit, false); err != nil {
		t.Fatal(err)
	}

	set := restore(t, game)

	assets := project.Assets{UI: []project.Texture{{Name: "hud-kit.badge"}}}
	if _, err := ResolveUI(t.Context(), game, assets, set); err == nil {
		t.Fatal("UI asset shadowed game definition")
	}

	assets = project.Assets{Textures: []project.Texture{{Name: "hud-kit.panel"}}}
	if _, err := ResolveUI(t.Context(), game, assets, set); err == nil {
		t.Fatal("UI image shadowed game definition")
	}

	write(t, filepath.Join(game, "ui/hud-badge.kui"), []byte(`<template><panel/></template>`))

	assets = project.Assets{
		UI:    []project.Texture{{Name: "local", Source: "ui/hud-badge.kui"}},
		Fonts: []project.Font{{Role: "body", Source: "my-font.ttf"}},
	}

	resolved, err := ResolveUI(t.Context(), game, assets, set)
	if err != nil || len(resolved.Fonts) != 1 || resolved.Fonts[0].Source != "my-font.ttf" {
		t.Fatalf("project font override: %+v: %v", resolved.Fonts, err)
	}

	if _, err := project.CompileUI(game, resolved.UI, resolved.Layouts, resolved.Theme.Source); err == nil {
		t.Fatal("duplicate KartUI component name accepted")
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := ResolveUI(ctx, game, project.Assets{}, set); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled UI resolution: %v", err)
	}
}
