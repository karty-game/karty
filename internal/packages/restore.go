package packages

import (
	"context"
	"crypto/rand"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
)

type Resolved struct {
	Manifest  Manifest
	Root      *os.Root
	Directory string
}
type Set struct{ Items map[string]*Resolved }

func (set *Set) Close() {
	if set != nil {
		for _, item := range set.Items {
			_ = item.Root.Close()
		}
	}
}
func (set *Set) Names() []string {
	names := make([]string, 0, len(set.Items))
	for name := range set.Items {
		names = append(names, name)
	}

	slices.Sort(names)

	return names
}

func folderPath(project, name string) (string, error) {
	if !filepath.IsAbs(name) {
		name = filepath.Join(project, name)
	}

	absolute, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}

	return filepath.EvalSymlinks(absolute)
}

// FolderRoots describes explicit development capabilities. The watcher opens a
// separate confined root for each; project path confinement is never relaxed.
func FolderRoots(ctx context.Context, project string) (map[string]string, error) {
	sources, _, err := LoadSources(ctx, project)
	if err != nil {
		return nil, err
	}

	roots := map[string]string{}

	for packageName, source := range sources {
		if source.Folder == "" {
			continue
		}

		root, err := folderPath(project, source.Folder)
		if err != nil {
			return nil, fmt.Errorf("package %s: %w", packageName, err)
		}

		roots[packageName] = root
	}

	return roots, nil
}

// Restore regenerates only ignored cache/link state, never project manifests or
// go.mod. Every archive is verified against the committed source and checksum.
func Restore(ctx context.Context, project string) (_ *Set, err error) {
	return restoreWithClient(ctx, project, nil)
}

func restoreWithClient(ctx context.Context, project string, client *http.Client) (_ *Set, err error) {
	client = remoteArchiveClient(client)

	sources, sdk, err := LoadSources(ctx, project)
	if err != nil {
		return nil, err
	}

	set := &Set{Items: map[string]*Resolved{}}

	defer func() {
		if err != nil {
			set.Close()
		}
	}()

	if len(sources) == 0 {
		return set, nil
	}

	project, err = filepath.Abs(project)
	if err != nil {
		return nil, err
	}

	root, err := os.OpenRoot(project)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	if err := MakeDirectories(root, ".karty/packages/.store"); err != nil {
		return nil, err
	}

	packageNames := make([]string, 0, len(sources))
	for packageName := range sources {
		packageNames = append(packageNames, packageName)
	}

	slices.Sort(packageNames)

	modules := map[string]bool{}
	targets := map[string]string{}

	for _, packageName := range packageNames {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		directory, target, packageRoot, err := resolveSource(ctx, root, project, sources[packageName], client)
		if err != nil {
			return nil, fmt.Errorf("package %s: %w", packageName, err)
		}

		definition, manifestErr := ReadManifest(ctx, packageRoot)
		if manifestErr != nil {
			_ = packageRoot.Close()

			return nil, fmt.Errorf("package %s: %w", packageName, manifestErr)
		}

		set.Items[packageName] = &Resolved{definition, packageRoot, directory}
		if err := validateResolvedManifest(packageName, sdk, definition, modules); err != nil {
			return nil, err
		}

		if definition.GoModule != "" {
			modules[definition.GoModule] = true
		}

		targets[packageName] = target
	}
	// Validate the complete dependency set and all owned package link paths before
	// changing any links. A bad later package must not redirect earlier package links.
	for _, packageName := range packageNames {
		if err := checkLinkOutput(root, packageName); err != nil {
			return nil, err
		}
	}

	for _, packageName := range packageNames {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if err := makeLink(root, packageName, targets[packageName]); err != nil {
			return nil, err
		}
	}

	return set, nil
}

func validateResolvedManifest(name, sdk string, definition Manifest, modules map[string]bool) error {
	if definition.Name != name {
		return fmt.Errorf("%w: packages.%s references package %q; the key must match its manifest name", ErrPackage, name, definition.Name)
	}

	if !compatibleSDK(definition.SDK, sdk) {
		return fmt.Errorf("%w: package %s requires SDK >= %s, project pins %s", ErrPackage, name, definition.SDK, sdk)
	}

	if definition.GoModule != "" && modules[definition.GoModule] {
		return fmt.Errorf("%w: package %s duplicates Go module %s", ErrPackage, name, definition.GoModule)
	}

	return nil
}

func compatibleSDK(minimum, version string) bool {
	if minimum == "" {
		return true
	}

	current := "v" + version

	return semver.Canonical(current) == current && semver.Compare(current, "v"+minimum) >= 0
}

func restoreArchive(ctx context.Context, project *os.Root, digest string, data []byte) (*os.Root, error) {
	files, err := decodeArchive(ctx, data)
	if err != nil {
		return nil, err
	}

	cache := ".karty/packages/.store/" + digest
	if info, err := project.Lstat(cache); err == nil {
		if !info.IsDir() {
			return nil, ErrPackage
		}

		root, err := project.OpenRoot(cache)
		if err != nil {
			return nil, err
		}

		if err := verifyCache(ctx, root, files); err != nil {
			_ = root.Close()

			return nil, err
		}

		return root, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}

	stage := fmt.Sprintf(".karty/packages/.store/.stage-%x", nonce)
	if err := project.Mkdir(stage, 0700); err != nil {
		return nil, err
	}
	defer func() { _ = project.RemoveAll(stage) }()

	for name, contents := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if err := WriteGenerated(ctx, project, path.Join(stage, name), contents, true); err != nil {
			return nil, err
		}
	}

	root, err := project.OpenRoot(stage)
	if err != nil {
		return nil, err
	}

	_, err = ReadManifest(ctx, root)
	_ = root.Close()

	if err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := project.Rename(stage, cache); err != nil {
		// Another restore may have completed the identical content-addressed
		// package. Verify it independently rather than weakening ownership checks.
		root, openErr := project.OpenRoot(cache)
		if openErr != nil {
			return nil, err
		}

		if verifyErr := verifyCache(ctx, root, files); verifyErr != nil {
			_ = root.Close()

			return nil, verifyErr
		}

		return root, nil
	}

	return project.OpenRoot(cache)
}

func verifyCache(ctx context.Context, root *os.Root, expected map[string][]byte) error {
	count := 0

	err := fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		if entry.Type()&os.ModeSymlink != 0 {
			return ErrPackage
		}

		if entry.IsDir() {
			if strings.Count(name, "/") > MaxPathDepth {
				return ErrPackage
			}

			return nil
		}

		data, ok := expected[name]
		if !ok || count >= MaxFiles {
			return fmt.Errorf("%w: unexpected cached file %s", ErrPackage, name)
		}

		cached, err := Read(ctx, root, name, MaxFileBytes)
		if err != nil || Digest(cached) != Digest(data) {
			return fmt.Errorf("%w: cached package changed at %s; clear .karty/packages to restore", ErrPackage, name)
		}

		count++

		return nil
	})
	if err == nil && count != len(expected) {
		err = fmt.Errorf("%w: incomplete cached package", ErrPackage)
	}

	return err
}

func checkLinkOutput(root *os.Root, packageName string) error {
	info, err := root.Lstat(".karty/packages/" + packageName)
	if os.IsNotExist(err) {
		return nil
	}

	if err != nil {
		return err
	}

	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("%w: package link %s must be a generated link", ErrPackage, packageName)
	}

	return nil
}

func makeLink(root *os.Root, packageName, target string) error {
	name := ".karty/packages/" + packageName
	if info, err := root.Lstat(name); err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("%w: package link %s must be a generated link", ErrPackage, packageName)
		}

		before, err := root.Readlink(name)
		if err != nil {
			return err
		}

		if before == target {
			return nil
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}

	temporary := fmt.Sprintf(".karty/packages/.link-%x", nonce)
	if err := root.Symlink(target, temporary); err != nil {
		return fmt.Errorf("create development/package link (filesystem link support is required): %w", err)
	}
	defer func() { _ = root.Remove(temporary) }()

	return root.Rename(temporary, name)
}

func CheckModules(ctx context.Context, project string, set *Set) error {
	hasCode := false

	for _, item := range set.Items {
		if item.Manifest.GoModule != "" {
			hasCode = true

			break
		}
	}

	if !hasCode {
		return nil
	}

	root, err := os.OpenRoot(project)
	if err != nil {
		return err
	}
	defer root.Close()

	data, err := Read(ctx, root, "go.mod", MaxManifestBytes)
	if err != nil {
		return err
	}

	file, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return err
	}

	for _, packageName := range set.Names() {
		definition := set.Items[packageName].Manifest
		if definition.GoModule == "" {
			continue
		}

		required, replaced := false, false

		for _, entry := range file.Require {
			if entry.Mod.Path == definition.GoModule && entry.Mod.Version == "v"+definition.Version {
				required = true
			}
		}

		for _, entry := range file.Replace {
			if entry.Old.Path == definition.GoModule && entry.Old.Version == "" && entry.New.Version == "" &&
				filepath.Clean(entry.New.Path) == filepath.Join(".karty", "packages", packageName) {
				replaced = true
			}
		}

		if !required || !replaced {
			return fmt.Errorf(
				"%w: go.mod must require %s v%s and replace it with ./.karty/packages/%s (karty package import configures this)",
				ErrPackage,
				definition.GoModule,
				definition.Version,
				packageName,
			)
		}
	}

	return nil
}

func resolveSource(ctx context.Context, root *os.Root, project string,
	source Source, client *http.Client) (string, string, *os.Root, error) {
	if source.Folder != "" {
		directory, err := folderPath(project, source.Folder)
		if err != nil {
			return "", "", nil, err
		}

		packageRoot, err := os.OpenRoot(directory)

		return directory, directory, packageRoot, err
	}

	data, err := readArchiveSource(ctx, root, source, client)
	if err != nil {
		return "", "", nil, err
	}

	if Digest(data) != source.SHA256 {
		return "", "", nil, fmt.Errorf("%w: archive checksum mismatch", ErrPackage)
	}

	packageRoot, err := restoreArchive(ctx, root, source.SHA256, data)
	if err != nil {
		return "", "", nil, err
	}

	if validRemoteArchiveSource(source.Archive) {
		if err := WriteGenerated(ctx, root, remoteArchiveCachePath(source.SHA256), data, true); err != nil {
			_ = packageRoot.Close()

			return "", "", nil, err
		}
	}

	directory := filepath.Join(project, ".karty/packages/.store", source.SHA256)

	return directory, filepath.Join(".store", source.SHA256), packageRoot, err
}
