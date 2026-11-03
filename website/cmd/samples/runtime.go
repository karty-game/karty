package main

import (
	"fmt"
	"io/fs"
	"os"
	"regexp"
)

var runtimeVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

func requireRuntimeDirectory(root *os.Root, path string) error {
	info, err := root.Lstat(path)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		return fmt.Errorf("runtime output directory must not be linked: %w", os.ErrInvalid)
	}

	return nil
}

func validateRuntime(source fs.FS) error {
	versions, err := fs.ReadDir(source, "runtime")
	if err != nil {
		return err
	}

	if len(versions) == 0 {
		return fmt.Errorf("shared runtime has no versions: %w", os.ErrInvalid)
	}

	for _, version := range versions {
		if !version.IsDir() || !runtimeVersion.MatchString(version.Name()) {
			return fmt.Errorf("invalid shared runtime version: %w", os.ErrInvalid)
		}

		directory := "runtime/" + version.Name()

		files, err := fs.ReadDir(source, directory)
		if err != nil {
			return err
		}

		if len(files) != 1 || !files[0].IsDir() || !hostedRuntimeHash.MatchString(files[0].Name()) {
			return fmt.Errorf("shared runtime must contain one verified WebGPU bundle: %w", os.ErrInvalid)
		}

		shared, err := fs.Sub(source, directory+"/"+files[0].Name())
		if err != nil {
			return err
		}

		if err := validateHostedRuntime(shared, files[0].Name()); err != nil {
			return err
		}
	}

	return nil
}
