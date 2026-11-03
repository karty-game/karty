package levelbuild

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/karty-game/karty-sdk/format/character"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty/internal/characterimport"
	"github.com/karty-game/karty/internal/packages"
)

type characterEntry struct {
	ID         string `toml:"id"`
	Model      string `toml:"model"`
	Animations string `toml:"animations"`
}

func resolveBuildPackages(ctx context.Context, directory string, definition manifest, set *packages.Set) (manifest, error) {
	definition, err := resolveLevelPackages(ctx, directory, definition, set)
	if err != nil {
		return definition, err
	}

	return resolveCharacterPackages(ctx, directory, definition, set)
}

func appendCharacters(
	ctx context.Context,
	directory string,
	definition manifest,
	data []level.SourceEntry,
	features []string,
) ([]level.SourceEntry, []string, error) {
	if len(definition.Characters) == 0 {
		return data, features, nil
	}

	if len(definition.Characters) > character.MaxModels || definition.Level.Kind != "world" || definition.World.Source == "" {
		return nil, nil, fmt.Errorf("characters require a world level and at most %d models: %w", character.MaxModels, ErrManifest)
	}

	inputs := make([]characterimport.Input, 0, len(definition.Characters))
	readCache := map[string][]byte{}
	total := 0

	read := func(source string) ([]byte, error) {
		if data, ok := readCache[source]; ok {
			return data, nil
		}

		data, err := readConfinedFile(directory, source, characterimport.MaxSourceBytes)
		if err != nil {
			return nil, err
		}

		total += len(data)
		if total > characterimport.MaxSourceBytes {
			return nil, fmt.Errorf("character source budget: %w", ErrManifest)
		}

		readCache[source] = data

		return data, nil
	}
	for _, entry := range definition.Characters {
		model, err := read(entry.Model)
		if err != nil {
			return nil, nil, err
		}

		animations, err := read(entry.Animations)
		if err != nil {
			return nil, nil, err
		}

		inputs = append(inputs, characterimport.Input{ID: entry.ID, Model: model, Animations: animations})
	}

	document, err := characterimport.Compile(ctx, inputs)
	if err != nil {
		return nil, nil, err
	}

	encoded, err := character.Encode(*document)
	if err != nil {
		return nil, nil, err
	}

	data = append(data, level.SourceEntry{Name: character.EntryName, Kind: level.EntryData, Data: encoded})
	features = append(features, character.Feature)
	slices.Sort(features)
	features = slices.Compact(features)

	return data, features, nil
}

func resolveCharacterPackages(ctx context.Context, directory string, definition manifest, set *packages.Set) (manifest, error) {
	if len(definition.Characters) == 0 {
		return definition, nil
	}

	if len(definition.Characters) > character.MaxModels {
		return definition, ErrManifest
	}

	root, err := os.OpenRoot(directory)
	if err != nil {
		return definition, err
	}
	defer root.Close()

	resolve := func(source string) (string, error) {
		if !strings.HasPrefix(source, "@") {
			return source, nil
		}

		name, path, ok := strings.Cut(strings.TrimPrefix(source, "@"), "/")
		if !ok || name == "" || !packages.ValidPath(path) || set == nil {
			return "", fmt.Errorf("invalid character package source %q: %w", source, ErrManifest)
		}

		item := set.Items[name]
		if item == nil {
			return "", fmt.Errorf("unknown character package %q: %w", name, ErrManifest)
		}

		contents, readErr := packages.Read(ctx, item.Root, path, characterimport.MaxSourceBytes)
		if readErr != nil {
			return "", readErr
		}

		staged := ".karty/packages/" + packages.Digest(contents) + ".glb"
		if writeErr := packages.WriteGenerated(ctx, root, staged, contents, true); writeErr != nil {
			return "", writeErr
		}

		return staged, nil
	}

	for i := range definition.Characters {
		entry := &definition.Characters[i]

		entry.Model, err = resolve(entry.Model)
		if err != nil {
			return definition, err
		}

		entry.Animations, err = resolve(entry.Animations)
		if err != nil {
			return definition, err
		}
	}

	return definition, nil
}
