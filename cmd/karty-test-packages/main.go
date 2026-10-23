// karty-test-packages prepares immutable public release fixtures outside tests.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/karty-game/karty/internal/packages"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/testpackages"
	"github.com/pelletier/go-toml/v2"
)

func main() {
	path := flag.String("path", "", "prepare releases and print one package's source path")
	archive := flag.String("archive", "", "prepare releases and print one package's verified archive path")

	flag.Parse()

	if err := run(*path, *archive, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(sourceName, archiveName string, args []string) error {
	if len(args) != 0 || sourceName != "" && archiveName != "" {
		return fmt.Errorf("use at most one of --path NAME or --archive NAME: %w", os.ErrInvalid)
	}

	pins, err := testpackages.Pins()
	if err != nil {
		return err
	}

	for _, name := range []string{sourceName, archiveName} {
		if _, ok := pins[name]; name != "" && !ok {
			return fmt.Errorf("unknown published package fixture %q: %w", name, os.ErrInvalid)
		}
	}

	project, err := testpackages.Project()
	if err != nil {
		return err
	}

	manifest := struct {
		Project struct {
			Name string `toml:"name"`
		} `toml:"project"`
		SDK struct {
			Version string `toml:"version"`
		} `toml:"sdk"`
		Packages map[string]testpackages.Pin `toml:"packages"`
	}{Packages: pins}
	manifest.Project.Name = "released-package-fixtures"
	manifest.SDK.Version = release.SDKVersion()

	data, err := toml.Marshal(manifest)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(project, 0755); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(project, "karty.toml"), data, 0o600); err != nil {
		return err
	}

	const preparationTimeout = 5 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), preparationTimeout)
	defer cancel()

	set, err := packages.Restore(ctx, project)
	if err != nil {
		return err
	}
	defer set.Close()

	switch {
	case sourceName != "":
		path, err := testpackages.Source(sourceName)
		if err != nil {
			return err
		}

		fmt.Fprintln(os.Stdout, path)
	case archiveName != "":
		path, err := testpackages.Archive(archiveName)
		if err != nil {
			return err
		}

		fmt.Fprintln(os.Stdout, path)
	default:
		fmt.Fprintf(os.Stdout, "Prepared %d published package fixtures in %s\n", len(pins), project)
	}

	return nil
}
