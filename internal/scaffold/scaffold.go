// Package scaffold renders versioned project templates from core resources.
package scaffold

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/karty-game/karty-ui/codegen"
	"github.com/karty-game/karty/internal/project"
	"github.com/karty-game/karty/internal/sdk"
)

type staticError string

func (err staticError) Error() string {
	return string(err)
}

const (
	errProjectDirectoryExists  staticError = "project directory already exists"
	errUnsupportedTemplateFile staticError = "template file must end in .tmpl or .base64"
)

// CreateGame renders a new game project at destination.
func CreateGame(destination, name string, manifest sdk.Manifest) error {
	return CreateTemplate(destination, name, manifest, "game")
}

// CreateTemplate selects user-owned game or UI source while sharing SDK setup.
func CreateTemplate(destination, name string, manifest sdk.Manifest, selection string) error {
	if !supportsTemplate(selection, manifest.API.Version) {
		return errUnsupportedTemplateFile
	}

	if _, err := os.Stat(destination); err == nil {
		return fmt.Errorf("%s: %w", destination, errProjectDirectoryExists)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect project directory: %w", err)
	}

	tree, err := sdk.Resources(manifest)
	if err != nil {
		return err
	}

	version := templateVersion(selection, manifest)

	gameTemplate := filepath.Join("templates", selection, version)
	if err := fs.WalkDir(tree, gameTemplate, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			return nil
		}

		contents, err := fs.ReadFile(tree, path)
		if err != nil {
			return err
		}

		relative, err := filepath.Rel(gameTemplate, path)
		if err != nil {
			return err
		}

		data := struct {
			Name     string
			Manifest sdk.Manifest
		}{Name: name, Manifest: manifest}

		relative, output, renderErr := renderTemplateFile(path, relative, contents, data)
		if renderErr != nil {
			return renderErr
		}

		target := filepath.Join(destination, relative)

		mkdirErr := os.MkdirAll(filepath.Dir(target), 0o750)
		if mkdirErr != nil {
			return mkdirErr
		}

		return os.WriteFile(target, output, 0o600)
	}); err != nil {
		return fmt.Errorf("render game template: %w", err)
	}

	generated, err := sdk.ClientFiles(manifest, "tinygo")
	if err != nil {
		return fmt.Errorf("generate client API: %w", err)
	}

	config, err := project.Load(destination)
	if err != nil {
		return err
	}

	textureNames := make([]string, 0, len(config.Assets.Textures))
	for _, texture := range config.Assets.Textures {
		textureNames = append(textureNames, texture.Name)
	}

	modulePath := "example.com/" + name
	engineImport := modulePath + "/.karty/engine"

	assetFile, err := codegen.TextureAssetPackageFile(textureNames, engineImport)
	if err != nil {
		return fmt.Errorf("generate typed assets: %w", err)
	}

	generated["assets/textures.go"] = assetFile
	if manifest.API.Version == "0.0.1" {
		if err := addUIViews(
			destination, modulePath, generated, config.Assets.UI, config.Assets.Layouts, config.Assets.Theme.Source,
		); err != nil {
			return err
		}
	}

	if err := addUIPackage(
		destination,
		modulePath,
		manifest.API.Version,
		config.Assets.UI,
		config.Assets.Layouts,
		config.Assets.Theme.Source,
		generated,
	); err != nil {
		return err
	}

	if err := writeBindings(destination, generated); err != nil {
		return err
	}

	return sdk.SyncDocs(destination, manifest)
}

func templateVersion(selection string, manifest sdk.Manifest) string {
	if selection != "ui" {
		return manifest.Templates.Game
	}

	return manifest.Templates.Game
}

func supportsTemplate(selection, version string) bool {
	return selection == "game" || (selection == "ui" && version == "0.0.1")
}

func addUIPackage(
	directory, module, version string,
	assets []project.Texture,
	layouts []project.Layout,
	theme string,
	generated map[string][]byte,
) error {
	if version != "0.0.1" {
		return nil
	}

	views, err := project.CompileUI(directory, assets, layouts, theme)
	if err != nil {
		return err
	}

	files, err := codegen.UIPackageFiles(views, module)
	if err != nil {
		return err
	}

	for _, view := range views {
		if view.Local {
			generated["ui/karty_ui_"+view.Name+".go"] = files[view.Source]
		}
	}

	return nil
}

func writeBindings(destination string, generated map[string][]byte) error {
	for relativePath, contents := range generated {
		generatedRoot := filepath.Join(destination, ".karty")

		path := filepath.Join(generatedRoot, filepath.FromSlash(relativePath))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return fmt.Errorf("create generated client API directory: %w", err)
		}

		if err := os.WriteFile(path, contents, 0o600); err != nil {
			return fmt.Errorf("write generated client API: %w", err)
		}
	}

	return nil
}

func addUIAssets(generated map[string][]byte, assets []project.Texture, engineImport string) error {
	names := make([]string, 0, len(assets))
	for _, asset := range assets {
		names = append(names, asset.Name)
	}

	contents, err := codegen.UIAssetPackageFile(names, engineImport)
	if err != nil {
		return err
	}

	generated["assets/ui.go"] = contents

	return nil
}

func renderTemplateFile(path, relative string, contents []byte, data any) (string, []byte, error) {
	if strings.HasSuffix(relative, ".tmpl") {
		rendered, err := template.New(path).Parse(string(contents))
		if err != nil {
			return "", nil, fmt.Errorf("parse template %s: %w", path, err)
		}

		var output bytes.Buffer
		if err := rendered.Execute(&output, data); err != nil {
			return "", nil, fmt.Errorf("render template %s: %w", path, err)
		}

		return strings.TrimSuffix(relative, ".tmpl"), output.Bytes(), nil
	}

	if strings.HasSuffix(relative, ".base64") {
		encoded := strings.Join(strings.Fields(string(contents)), "")

		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return "", nil, fmt.Errorf("decode template %s: %w", path, err)
		}

		return strings.TrimSuffix(relative, ".base64"), decoded, nil
	}

	return "", nil, fmt.Errorf("%s: %w", path, errUnsupportedTemplateFile)
}

func addUIViews(
	directory string,
	modulePath string,
	generated map[string][]byte,
	assets []project.Texture,
	layouts []project.Layout,
	theme string,
) error {
	if err := addUIAssets(generated, assets, modulePath+"/.karty/engine"); err != nil {
		return err
	}

	views, err := project.CompileUI(directory, assets, layouts, theme)
	if err != nil {
		return err
	}

	generated["engine/ui-views.go"], err = codegen.UIViewFile(views)

	return err
}
