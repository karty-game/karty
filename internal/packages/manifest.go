package packages

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

var namePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,47}$`)

type Source struct {
	Archive string `koanf:"archive" toml:"archive,omitempty"`
	Folder  string `koanf:"folder"  toml:"folder,omitempty"`
	SHA256  string `koanf:"sha256"  toml:"sha256,omitempty"`
}

type Manifest struct {
	FormatVersion int         `toml:"format_version"`
	Name          string      `toml:"name"`
	Version       string      `toml:"version"`
	SDK           string      `toml:"sdk,omitempty"`
	Prefabs       string      `toml:"prefabs,omitempty"`
	Textures      []Texture   `toml:"textures,omitempty"`
	UI            UIExports   `toml:"ui,omitempty"`
	Processors    []Processor `toml:"processors,omitempty"`
	// GoModule is read from go.mod, never a second editable declaration.
	GoModule string `toml:"-"`
}

type Texture struct {
	Name    string `toml:"name"`
	Source  string `toml:"source"`
	Profile string `toml:"profile"`
}

func LoadSources(ctx context.Context, directory string) (map[string]Source, string, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, "", err
	}
	defer root.Close()

	data, err := Read(ctx, root, "karty.toml", MaxManifestBytes)
	if os.IsNotExist(err) {
		return nil, "", nil
	}

	if err != nil {
		return nil, "", err
	}

	var document struct {
		Packages map[string]Source `toml:"packages"`
		SDK      struct {
			Version string `toml:"version"`
		} `toml:"sdk"`
	}
	if err := toml.Unmarshal(data, &document); err != nil {
		return nil, "", err
	}

	var raw struct {
		Packages map[string]map[string]any `toml:"packages"`
	}
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, "", err
	}

	if len(document.Packages) > MaxPackages {
		return nil, "", fmt.Errorf("%w: at most 16 packages", ErrPackage)
	}

	if err := validateSources(document.Packages, raw.Packages); err != nil {
		return nil, "", err
	}

	return document.Packages, document.SDK.Version, nil
}

func ReadManifest(ctx context.Context, root *os.Root) (Manifest, error) {
	data, err := Read(ctx, root, ManifestName, MaxManifestBytes)
	if err != nil {
		return Manifest{}, err
	}

	var definition Manifest
	if err := toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields().Decode(&definition); err != nil {
		return definition, err
	}

	if definition.FormatVersion != 1 || !namePattern.MatchString(definition.Name) || !validPath(definition.Name) ||
		semver.Canonical("v"+definition.Version) != "v"+definition.Version ||
		len(definition.Version) > 64 {
		return definition, fmt.Errorf("%w: unsupported format, identity or version", ErrPackage)
	}

	if definition.SDK != "" && semver.Canonical("v"+definition.SDK) != "v"+definition.SDK {
		return definition, fmt.Errorf("%w: invalid SDK version", ErrPackage)
	}

	if definition.Prefabs != "" && !validPath(definition.Prefabs) {
		return definition, ErrPackage
	}

	if len(definition.Textures) > MaxTextures {
		return definition, ErrPackage
	}

	seen := map[string]bool{}

	var total int

	for _, texture := range definition.Textures {
		if !namePattern.MatchString(texture.Name) || seen[texture.Name] || !validPath(texture.Source) ||
			!strings.HasSuffix(strings.ToLower(texture.Source), ".png") ||
			texture.Profile == "" {
			return definition, fmt.Errorf("%w: invalid texture %q", ErrPackage, texture.Name)
		}

		seen[texture.Name] = true

		data, err := Read(ctx, root, texture.Source, MaxFileBytes)
		if err != nil {
			return definition, fmt.Errorf("texture %s: %w", texture.Name, err)
		}

		total += len(data)
		if total > MaxTotalBytes {
			return definition, ErrPackage
		}
	}

	definition.GoModule, err = readGoModule(ctx, root, definition.Version)
	if err != nil {
		return definition, err
	}

	if definition.GoModule == "" && definition.Prefabs == "" && len(definition.Textures) == 0 && definition.UI.empty() &&
		len(definition.Processors) == 0 {
		return definition, fmt.Errorf("%w: package exports no code or resources", ErrPackage)
	}

	if err := validatePrefabs(ctx, root, definition); err != nil {
		return definition, err
	}

	if err := validateUI(ctx, root, definition.UI); err != nil {
		return definition, err
	}

	if err := validateProcessors(definition.Processors); err != nil {
		return definition, err
	}

	return definition, nil
}

func readGoModule(ctx context.Context, root *os.Root, version string) (string, error) {
	data, err := Read(ctx, root, "go.mod", MaxManifestBytes)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	if err == nil {
		mod, err := modfile.Parse("go.mod", data, nil)
		if err != nil {
			return "", err
		}

		if mod.Module == nil || len(mod.Replace) > 0 {
			return "", fmt.Errorf("%w: package go.mod needs a module and no local replacements", ErrPackage)
		}

		modulePath := mod.Module.Mod.Path
		if err := module.Check(modulePath, "v"+version); err != nil {
			return "", err
		}

		return modulePath, nil
	}

	return "", nil
}

func validateSources(sources map[string]Source, raw map[string]map[string]any) error {
	packageNames := map[string]bool{}

	for packageName, source := range sources {
		folded := strings.ToLower(packageName)
		if packageNames[folded] {
			return fmt.Errorf("%w: package names differ only by case", ErrPackage)
		}

		packageNames[folded] = true

		for key := range raw[packageName] {
			if key != "folder" && key != "archive" && key != "sha256" {
				return fmt.Errorf("%w: unknown packages.%s.%s", ErrPackage, packageName, key)
			}
		}

		if !namePattern.MatchString(packageName) || !validPath(packageName) || (source.Archive == "") == (source.Folder == "") {
			return fmt.Errorf("%w: %s requires exactly one archive or folder", ErrPackage, packageName)
		}

		switch {
		case source.Folder != "":
			if source.SHA256 != "" || strings.Contains(source.Folder, "://") {
				return ErrPackage
			}
		case !validArchiveSource(source.Archive) || !strings.HasSuffix(source.Archive, ".kpkg") || len(source.SHA256) != 64 || strings.ToLower(source.SHA256) != source.SHA256:
			return fmt.Errorf("%w: %s needs a confined .kpkg path and SHA-256", ErrPackage, packageName)
		default:
			for _, c := range source.SHA256 {
				if !strings.ContainsRune("0123456789abcdef", c) {
					return ErrPackage
				}
			}
		}
	}

	return nil
}

func validArchiveSource(name string) bool {
	if !validPath(name) {
		return false
	}

	for part := range strings.SplitSeq(name, "/") {
		if excluded(part) {
			return false
		}
	}

	return true
}
