package levelbuild

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/karty-game/karty-sdk/format/world"
	worldschema "github.com/karty-game/karty/internal/worldbuild/schema"
	"github.com/karty-game/karty/internal/worldbuild/source"
	"github.com/pelletier/go-toml/v2"
)

var ErrSchemaOutput = errors.New("invalid level schema output")

// GenerateSchemas refreshes the base and per-level editor schemas. When
// validate is true, schema checks run before SDK installation or asset work.
// Only level manifests and authored YAML are read; texture bytes are not needed.
func GenerateSchemas(directory string, validate bool) (int, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return 0, err
	}
	defer root.Close()

	base, err := worldschema.Encode(worldschema.Base())
	if err != nil {
		return 0, err
	}

	if err = writeSchema(root, worldschema.BaseFilename, base); err != nil {
		return 0, err
	}

	entries, err := fs.ReadDir(root.FS(), "levels")
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}

	if err != nil {
		return 0, fmt.Errorf("read levels for schemas: %w", err)
	}

	count := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		levelDirectory := filepath.Join(directory, "levels", entry.Name())

		levelRoot, openErr := root.OpenRoot("levels/" + entry.Name())
		if openErr != nil {
			return count, openErr
		}

		contents, readErr := readSchemaManifest(levelRoot)
		_ = levelRoot.Close()

		if errors.Is(readErr, fs.ErrNotExist) {
			continue
		}

		if readErr != nil {
			return count, readErr
		}

		var definition manifest
		if err := toml.Unmarshal(contents, &definition); err != nil {
			return count, fmt.Errorf("parse %s: %w", filepath.Join(levelDirectory, "level.toml"), err)
		}

		if definition.World.Source == "" {
			continue
		}

		value, readErr := schemaWorld(levelDirectory, definition)
		s := worldschema.Level(value, schemaTextures(definition))

		data, err := worldschema.Encode(s)
		if err != nil {
			return count, err
		}

		if err = writeSchema(root, "levels/"+entry.Name()+".schema.json", data); err != nil {
			return count, err
		}

		count++
		if readErr != nil {
			return count, readErr
		}

		if validate {
			if err := validateSchemaValue(levelDirectory, definition, value); err != nil {
				return count, err
			}
		}
	}

	return count, nil
}

func readSchemaManifest(root *os.Root) ([]byte, error) {
	file, err := root.Open("level.toml")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > world.MaxEncodedSize {
		return nil, fmt.Errorf("%w: level.toml must be a regular file of 1–%d bytes", ErrManifest, world.MaxEncodedSize)
	}

	contents, err := io.ReadAll(io.LimitReader(file, world.MaxEncodedSize+1))
	if len(contents) > world.MaxEncodedSize {
		return nil, ErrManifest
	}

	return contents, err
}

func schemaTextures(definition manifest) []string {
	names := make([]string, 0, len(definition.Textures))
	for _, texture := range definition.Textures {
		names = append(names, texture.Name)
	}

	return names
}

func schemaWorld(directory string, definition manifest) (any, error) {
	contents, err := readConfinedFile(directory, definition.World.Source, world.MaxEncodedSize)
	if err != nil {
		return nil, err
	}

	value, err := worldschema.Parse(contents)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Join(directory, definition.World.Source), err)
	}

	return value, nil
}

func validateWorldSchema(directory string, definition manifest) error {
	if definition.World.Source == "" {
		return nil
	}

	value, err := schemaWorld(directory, definition)
	if err != nil {
		return err
	}

	return validateSchemaValue(directory, definition, value)
}

func validateSchemaValue(directory string, definition manifest, value any) error {
	err := worldschema.Validate(value, worldschema.Level(value, schemaTextures(definition)))
	if err == nil {
		return nil
	}
	// Keep existing semantic error identities available to callers when a
	// schema bound also violates the public world contract.
	_, semanticErr := source.Load(directory, definition.World.Source)

	return fmt.Errorf("%s: %w", filepath.Join(directory, definition.World.Source), errors.Join(err, semanticErr))
}

func writeSchema(root *os.Root, name string, contents []byte) error {
	if !fs.ValidPath(name) || strings.Contains(name, "\\") {
		return ErrSchemaOutput
	}

	for _, directory := range []string{".karty", ".karty/schemas", ".karty/schemas/levels"} {
		if info, err := root.Lstat(directory); err == nil {
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("%w: %s must be a directory, without symlinks", ErrSchemaOutput, directory)
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}

		if err := root.MkdirAll(directory, 0750); err != nil {
			return err
		}
	}

	path := ".karty/schemas/" + name
	if info, err := root.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%w: %s must be a regular file, without symlinks", ErrSchemaOutput, path)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	if err := root.WriteFile(path, contents, 0600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}
