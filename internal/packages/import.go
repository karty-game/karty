package packages

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
	"golang.org/x/mod/modfile"
)

// Import records a checksum-pinned catalog release, portable local archive or
// explicit development folder. External local archives are copied into the project.
// --replace is explicit permission to update one existing package declaration.
func Import(ctx context.Context, project, input string, replace bool) (Manifest, error) {
	return importWithCatalog(ctx, project, input, replace, DefaultCatalogURL, nil)
}

func importWithCatalog(ctx context.Context, project, input string, replace bool, catalogURL string, client *http.Client) (Manifest, error) {
	return importPackage(ctx, project, importRequest{input: input, replace: replace}, catalogURL, client)
}

func importPackage(ctx context.Context, project string, request importRequest, catalogURL string, client *http.Client) (Manifest, error) {
	project, err := filepath.Abs(project)
	if err != nil {
		return Manifest{}, err
	}

	sources, sdk, err := LoadSources(ctx, project)
	if err != nil {
		return Manifest{}, err
	}

	root, err := os.OpenRoot(project)
	if err != nil {
		return Manifest{}, err
	}
	defer root.Close()

	before, err := Read(ctx, root, "karty.toml", MaxManifestBytes)
	if err != nil {
		return Manifest{}, err
	}

	client = remoteArchiveClient(client)

	selected, named, err := request.selection(sources)
	if err != nil {
		return Manifest{}, err
	}

	var (
		source      Source
		packageRoot *os.Root
	)

	if named {
		if err := validateImportTarget(sources, selected.Name, request.replace); err != nil {
			return Manifest{}, err
		}

		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, remoteArchiveTimeout)
		defer cancel()

		selected, packageRoot, err = openCatalogImport(ctx, root, project, selected, catalogURL, client)
		source = selected.Source
	} else {
		source, packageRoot, err = openImportSource(ctx, root, project, request.input)
	}

	if err != nil {
		return Manifest{}, err
	}

	defer packageRoot.Close()

	definition, err := ReadManifest(ctx, packageRoot)
	if err != nil {
		return definition, err
	}

	if named && (definition.Name != selected.Name || definition.Version != selected.Version) {
		return definition, fmt.Errorf(
			"%w: catalog selected %s@%s but archive declares %s@%s",
			ErrPackage,
			selected.Name,
			selected.Version,
			definition.Name,
			definition.Version,
		)
	}

	name := definition.Name
	if err := validateImportTarget(sources, name, request.replace); err != nil {
		return definition, err
	}

	if !compatibleSDK(definition.SDK, sdk) {
		return definition, fmt.Errorf("%w: package requires SDK >= %s, project pins %s", ErrPackage, definition.SDK, sdk)
	}

	_, exists := sources[name]

	after, err := editDeclaration(before, name, source, exists)
	if err != nil {
		return definition, err
	}

	var decoded map[string]any
	if err := toml.Unmarshal(after, &decoded); err != nil {
		return definition, err
	}

	oldMod, newMod, err := prepareImportModule(ctx, root, name, definition, exists)
	if err != nil {
		return definition, err
	}

	if err := commitImport(ctx, root, project, before, after, oldMod, newMod, client); err != nil {
		return definition, err
	}

	return definition, nil
}

func prepareImportModule(ctx context.Context, root *os.Root, name string, definition Manifest, exists bool) ([]byte, []byte, error) {
	if definition.GoModule == "" && !exists {
		return nil, nil, nil
	}

	before, err := Read(ctx, root, "go.mod", MaxManifestBytes)
	if err != nil && (!os.IsNotExist(err) || definition.GoModule != "") {
		return nil, nil, err
	}

	if before == nil {
		return nil, nil, nil
	}

	after, err := editModule(before, name, definition)

	return before, after, err
}

func validateImportTarget(sources map[string]Source, name string, replace bool) error {
	_, exists := sources[name]
	if exists && !replace {
		return fmt.Errorf("%w: package %s already exists; use --replace to update it", ErrPackage, name)
	}

	if !exists && len(sources) >= MaxPackages {
		return ErrPackage
	}

	return nil
}

func editModule(data []byte, packageName string, definition Manifest) ([]byte, error) {
	file, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return nil, err
	}

	if file.Module == nil || file.Module.Mod.Path == definition.GoModule {
		return nil, fmt.Errorf("%w: invalid consumer module", ErrPackage)
	}

	target := "./.karty/packages/" + packageName
	// Remove only replacements owned by this package, preserving all other modules.
	for _, replacement := range append([]*modfile.Replace(nil), file.Replace...) {
		owned := replacement.New.Version == "" && filepath.Clean(replacement.New.Path) == filepath.Clean(target)
		if replacement.Old.Path == definition.GoModule && !owned {
			return nil, fmt.Errorf(
				"%w: go.mod already replaces %s; remove that replacement before importing",
				ErrPackage,
				definition.GoModule,
			)
		}

		if owned {
			oldModule := replacement.Old.Path
			if err := file.DropReplace(replacement.Old.Path, replacement.Old.Version); err != nil {
				return nil, err
			}

			if oldModule != definition.GoModule {
				if err := file.DropRequire(oldModule); err != nil {
					return nil, err
				}
			}
		}
	}

	if definition.GoModule != "" {
		if err := file.AddRequire(definition.GoModule, "v"+definition.Version); err != nil {
			return nil, err
		}

		if err := file.AddReplace(definition.GoModule, "", target, ""); err != nil {
			return nil, err
		}
	}

	file.Cleanup()

	return file.Format()
}

// Replace a single explicit package table, preserving unrelated TOML verbatim.
// Inline/dotted alternatives remain buildable; ambiguous rewrites fail closed.
func editDeclaration(data []byte, packageName string, source Source, exists bool) ([]byte, error) {
	body, err := toml.Marshal(source)
	if err != nil {
		return nil, err
	}

	entry := append([]byte("[packages."+packageName+"]\n"), body...)
	if !exists {
		return append(append(append([]byte(nil), data...), '\n'), entry...), nil
	}

	var parser unstable.Parser
	parser.Reset(data)

	start, end := -1, -1
	active := false

	for parser.NextExpression() {
		node := parser.Expression()
		if node.Kind == unstable.Table || node.Kind == unstable.ArrayTable {
			keys := []string{}

			iterator := node.Key()
			for iterator.Next() {
				keys = append(keys, string(iterator.Node().Data))
			}

			active = len(keys) == 2 && keys[0] == "packages" && keys[1] == packageName
			if active {
				offset := int(node.Child().Raw.Offset)
				start = bytes.LastIndexByte(data[:offset], '\n') + 1
				end = lineEnd(data, offset)
			}
		} else if active && node.Kind == unstable.KeyValue {
			end = lineEnd(data, int(node.Raw.Offset+node.Raw.Length))
		}
	}

	if err := parser.Error(); err != nil {
		return nil, err
	}

	if start < 0 || end < start {
		return nil, fmt.Errorf(
			"%w: cannot update an inline package declaration; use an explicit [packages.%s] table",
			ErrPackage,
			packageName,
		)
	}

	result := append([]byte(nil), data[:start]...)
	result = append(result, entry...)

	return append(result, data[end:]...), nil
}

func lineEnd(data []byte, offset int) int {
	if index := bytes.IndexByte(data[offset:], '\n'); index >= 0 {
		return offset + index + 1
	}

	return len(data)
}

func openImportSource(ctx context.Context, root *os.Root, project, input string) (Source, *os.Root, error) {
	input, err := filepath.Abs(input)
	if err != nil {
		return Source{}, nil, err
	}

	input, err = filepath.EvalSymlinks(input)
	if err != nil {
		return Source{}, nil, err
	}

	info, err := os.Stat(input)
	if err != nil {
		return Source{}, nil, err
	}

	var (
		source      Source
		packageRoot *os.Root
	)
	if info.IsDir() {
		packageRoot, err = os.OpenRoot(input)

		relative, relErr := filepath.Rel(project, input)
		if relErr != nil {
			relative = input
		}

		source.Folder = filepath.ToSlash(relative)

		return source, packageRoot, err
	}

	if !info.Mode().IsRegular() || !strings.HasSuffix(input, ".kpkg") {
		return Source{}, nil, fmt.Errorf("%w: expected a folder or .kpkg archive", ErrPackage)
	}

	parent, err := os.OpenRoot(filepath.Dir(input))
	if err != nil {
		return Source{}, nil, err
	}

	data, err := Read(ctx, parent, filepath.Base(input), MaxArchiveBytes)
	_ = parent.Close()

	if err != nil {
		return Source{}, nil, err
	}

	source.SHA256 = Digest(data)

	if err := MakeDirectories(root, ".karty/packages/.store"); err != nil {
		return Source{}, nil, err
	}

	packageRoot, err = restoreArchive(ctx, root, source.SHA256, data)
	if err != nil {
		return Source{}, nil, err
	}

	relative, relErr := filepath.Rel(project, input)
	if relErr != nil || !validArchiveSource(filepath.ToSlash(relative)) {
		relative = "packages/" + source.SHA256 + ".kpkg"
		if err := WriteGenerated(ctx, root, relative, data, true); err != nil {
			_ = packageRoot.Close()

			return Source{}, nil, err
		}
	}

	source.Archive = filepath.ToSlash(relative)

	if err != nil {
		return Source{}, nil, err
	}

	return source, packageRoot, nil
}

func commitImport(ctx context.Context, root *os.Root, project string, before, after, oldMod, newMod []byte, client *http.Client) error {
	if len(after) > MaxManifestBytes || len(newMod) > MaxManifestBytes {
		return ErrPackage
	}
	// Recheck user-owned files before committing the explicitly requested edits.
	current, err := Read(ctx, root, "karty.toml", MaxManifestBytes)
	if err != nil || !bytes.Equal(current, before) {
		return fmt.Errorf("%w: project manifest changed during import", ErrPackage)
	}

	if oldMod != nil {
		current, err = Read(ctx, root, "go.mod", MaxManifestBytes)
		if err != nil || !bytes.Equal(current, oldMod) {
			return fmt.Errorf("%w: go.mod changed during import", ErrPackage)
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if newMod != nil {
		if err := WriteGenerated(ctx, root, "go.mod", newMod, false); err != nil {
			return err
		}
	}

	if err = WriteGenerated(ctx, root, "karty.toml", after, false); err == nil {
		var set *Set

		set, err = restoreWithClient(ctx, project, client)
		if set != nil {
			set.Close()
		}
	}

	if err != nil {
		// Roll back only bytes still owned by this import. Concurrent user edits
		// always win; cache/verified archive files remain harmless reusable inputs.
		rollbackCtx := context.WithoutCancel(ctx)

		current, _ := Read(rollbackCtx, root, "karty.toml", MaxManifestBytes)
		if bytes.Equal(current, after) {
			err = errors.Join(err, WriteGenerated(rollbackCtx, root, "karty.toml", before, false))
		}

		if newMod != nil {
			current, _ = Read(rollbackCtx, root, "go.mod", MaxManifestBytes)
			if bytes.Equal(current, newMod) {
				err = errors.Join(err, WriteGenerated(rollbackCtx, root, "go.mod", oldMod, false))
			}
		}

		return err
	}

	return nil
}
